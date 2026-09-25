// Package httpserver exposes the analyzer's operational endpoints: /healthz (liveness),
// /readyz (readiness), /metrics (Prometheus self-observability), and /api/runs/latest (a
// debugging window into the runner while Phase 4's real findings API does not exist yet).
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// LatestProvider is implemented by *runner.Runner. Declared here, not imported from the runner
// package, so httpserver depends only on model -- the smallest interface that lets this package
// be tested with a fake, per AGENTS.md ("External calls ... sit behind interfaces").
type LatestProvider interface {
	Latest() *model.AnalysisRun
	Findings() []model.Finding
}

// TimeSeriesProvider is implemented by *evidence.Builder. A separate, optional interface (not
// folded into LatestProvider) because /api/timeseries is the one endpoint that queries
// Prometheus directly per-request rather than reading the runner's snapshot -- and because
// Builder already sits behind promclient.Client, an interface-behind-an-interface here would add
// a layer without adding real testability.
type TimeSeriesProvider interface {
	CPUUsageSeries(ctx context.Context, workload model.WorkloadRef, container string, window time.Duration, now time.Time) ([]evidence.Point, float64, float64, error)
	MemoryUsageSeries(ctx context.Context, workload model.WorkloadRef, container string, window time.Duration, now time.Time) ([]evidence.Point, float64, float64, error)
}

// Server serves the analyzer's HTTP endpoints.
type Server struct {
	mux     *http.ServeMux
	latest  LatestProvider
	builder TimeSeriesProvider // nil is valid: /api/timeseries then returns 501
	logger  *slog.Logger
}

// New builds a Server. It implements http.Handler directly, so callers wire it into an
// http.Server without an extra adapter. builder may be nil (disables /api/timeseries only).
// staticDir, if non-empty, serves the built frontend (frontend/dist after `npm run build`) at
// "/" with SPA fallback -- see AGENTS.md/docs/architecture.md's "web" module: one deployable,
// no separate Node process at runtime.
func New(latest LatestProvider, builder TimeSeriesProvider, staticDir string, logger *slog.Logger) *Server {
	s := &Server{mux: http.NewServeMux(), latest: latest, builder: builder, logger: logger}
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.Handle("GET /metrics", promhttp.Handler())
	s.mux.HandleFunc("GET /api/runs/latest", s.handleLatestRun)
	s.mux.HandleFunc("GET /api/findings", s.handleFindings)
	s.mux.HandleFunc("GET /api/timeseries", s.handleTimeseries)
	if staticDir != "" {
		s.mux.HandleFunc("GET /", spaHandler(staticDir))
	}
	return s
}

// spaHandler serves files directly out of dir; any path that doesn't correspond to a real file
// (a React Router route like /findings/abc123, not a real file on disk) falls back to
// index.html, so a hard refresh or a direct link to a client-side route works.
func spaHandler(dir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleHealthz answers "is the process alive and able to respond at all" -- it never depends
// on Prometheus or Kubernetes, so a data-source outage never makes the process look dead
// (which would cause an orchestrator to restart a process that has nothing wrong with it).
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleReadyz answers "does the analyzer currently have a fully healthy snapshot". Readiness
// requires the most recent run to be RunComplete (no source degraded), which is a superset of
// docs/architecture.md's documented case ("Prometheus down ... /readyz fails") -- a Kubernetes
// outage is treated the same way, since Phase 4's rules need both sources to trust a finding.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	run := s.latest.Latest()
	if run == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "not ready",
			"reason": "no analysis run has completed yet",
		})
		return
	}
	if run.Status != model.RunComplete {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":       "not ready",
			"run_status":   run.Status,
			"query_errors": run.QueryErrors,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

// handleLatestRun exposes the runner's snapshot as JSON. Phase 4 replaces/extends this with the
// real findings API (docs/requirements.md §6: /api/findings, /api/runs/latest); the path is
// deliberately the same one already named in docs/architecture.md so nothing has to move later.
func (s *Server) handleLatestRun(w http.ResponseWriter, r *http.Request) {
	run := s.latest.Latest()
	if run == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no analysis run has completed yet",
		})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// handleFindings exposes the current ranked finding list as JSON, per docs/requirements.md §6.
// Always returns an array, never null, even when there are zero findings -- a JSON consumer
// (the Phase 5 dashboard) should never need a null-check before iterating this response.
func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	fs := s.latest.Findings()
	if fs == nil {
		fs = []model.Finding{}
	}
	writeJSON(w, http.StatusOK, fs)
}

// handleTimeseries serves one workload/container's recent usage series plus its current
// request/limit, for the dashboard's usage-vs-request chart. The backend runs a fixed,
// parameterized query (namespace/workload/container/metric/window are all validated inputs into
// a query this code already controls) -- the browser never gets to send arbitrary PromQL
// (docs/architecture.md: "the API returns only findings JSON; Prometheus is ClusterIP-only").
func (s *Server) handleTimeseries(w http.ResponseWriter, r *http.Request) {
	if s.builder == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "timeseries endpoint not configured"})
		return
	}
	q := r.URL.Query()
	ns, name, container, metric := q.Get("namespace"), q.Get("workload"), q.Get("container"), q.Get("metric")
	if ns == "" || name == "" || container == "" || metric == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "namespace, workload, container and metric are required query params"})
		return
	}
	kind := q.Get("kind")
	if kind == "" {
		kind = "Deployment"
	}
	window := 24 * time.Hour
	if ws := q.Get("window"); ws != "" {
		if d, err := time.ParseDuration(ws); err == nil {
			window = d
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	wl := model.WorkloadRef{Namespace: ns, Name: name, Kind: kind}
	now := time.Now()

	var (
		points         []evidence.Point
		request, limit float64
		err            error
	)
	switch metric {
	case "cpu":
		points, request, limit, err = s.builder.CPUUsageSeries(ctx, wl, container, window, now)
	case "memory":
		points, request, limit, err = s.builder.MemoryUsageSeries(ctx, wl, container, window, now)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric must be \"cpu\" or \"memory\""})
		return
	}
	if err != nil {
		s.logger.Warn("timeseries query failed", "namespace", ns, "workload", name, "container", container, "metric", metric, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "querying prometheus failed"})
		return
	}
	if points == nil {
		points = []evidence.Point{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metric":  metric,
		"request": request,
		"limit":   limit,
		"points":  points,
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// Encoding errors here would mean writing to an already-flushed response; there is nothing
	// actionable left to do but drop it, so it is intentionally not checked further.
	_ = json.NewEncoder(w).Encode(body)
}
