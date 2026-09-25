// Package httpserver exposes the analyzer's operational endpoints: /healthz (liveness),
// /readyz (readiness), /metrics (Prometheus self-observability), and /api/runs/latest (a
// debugging window into the runner while Phase 4's real findings API does not exist yet).
package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// LatestProvider is implemented by *runner.Runner. Declared here, not imported from the runner
// package, so httpserver depends only on model -- the smallest interface that lets this package
// be tested with a fake, per AGENTS.md ("External calls ... sit behind interfaces").
type LatestProvider interface {
	Latest() *model.AnalysisRun
}

// Server serves the analyzer's HTTP endpoints.
type Server struct {
	mux    *http.ServeMux
	latest LatestProvider
	logger *slog.Logger
}

// New builds a Server. It implements http.Handler directly, so callers wire it into an
// http.Server without an extra adapter.
func New(latest LatestProvider, logger *slog.Logger) *Server {
	s := &Server{mux: http.NewServeMux(), latest: latest, logger: logger}
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.Handle("GET /metrics", promhttp.Handler())
	s.mux.HandleFunc("GET /api/runs/latest", s.handleLatestRun)
	return s
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

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// Encoding errors here would mean writing to an already-flushed response; there is nothing
	// actionable left to do but drop it, so it is intentionally not checked further.
	_ = json.NewEncoder(w).Encode(body)
}
