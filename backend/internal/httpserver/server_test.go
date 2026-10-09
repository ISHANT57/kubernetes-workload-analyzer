package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

type fakeLatestProvider struct {
	run      *model.AnalysisRun
	findings []model.Finding
}

func (f *fakeLatestProvider) Latest() *model.AnalysisRun { return f.run }
func (f *fakeLatestProvider) Findings() []model.Finding  { return f.findings }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealthz_AlwaysOK(t *testing.T) {
	// /healthz must stay 200 even with no run at all -- it answers "is the process alive", not
	// "is the data good" (that is /readyz's job).
	s := New(&fakeLatestProvider{run: nil}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz with no run yet: status = %d, want 200", rec.Code)
	}
}

func TestReadyz_NoRunYet_NotReady(t *testing.T) {
	s := New(&fakeLatestProvider{run: nil}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz before any run: status = %d, want 503", rec.Code)
	}
}

func TestReadyz_CompleteRun_Ready(t *testing.T) {
	s := New(&fakeLatestProvider{run: &model.AnalysisRun{Status: model.RunComplete}}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /readyz with a complete run: status = %d, want 200", rec.Code)
	}
}

// TestReadyz_PrometheusDown_NotReady is the httpserver-level counterpart of the runner test with
// the same name in intent: this is what actually answers the Phase 3 done-criterion "degrades
// cleanly with Prometheus down" from an external caller's point of view (e.g. a Kubernetes
// readiness probe).
func TestReadyz_PrometheusDown_NotReady(t *testing.T) {
	run := &model.AnalysisRun{
		Status:      model.RunPartial,
		QueryErrors: []model.QueryError{{Source: "prometheus", Message: "connection refused"}},
	}
	s := New(&fakeLatestProvider{run: run}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz with Prometheus down (partial run): status = %d, want 503", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding /readyz response body: %v", err)
	}
	if body["run_status"] != "partial" {
		t.Errorf("/readyz body run_status = %v, want partial", body["run_status"])
	}
}

func TestLatestRun_NoRunYet_ServiceUnavailable(t *testing.T) {
	s := New(&fakeLatestProvider{run: nil}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/latest", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/runs/latest before any run: status = %d, want 503", rec.Code)
	}
}

func TestLatestRun_ReturnsTheRun(t *testing.T) {
	run := &model.AnalysisRun{ID: "run-1", Status: model.RunComplete, WorkloadsSeen: 9}
	s := New(&fakeLatestProvider{run: run}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/latest", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/runs/latest: status = %d, want 200", rec.Code)
	}
	var got model.AnalysisRun
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if got.ID != "run-1" || got.WorkloadsSeen != 9 {
		t.Errorf("decoded run = %+v, want ID=run-1 WorkloadsSeen=9", got)
	}
}

func TestFindings_EmptyList_NotNull(t *testing.T) {
	s := New(&fakeLatestProvider{findings: nil}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/findings", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/findings with no findings: status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "[]\n" {
		t.Errorf("GET /api/findings body = %q, want \"[]\\n\" (never a bare null)", body)
	}
}

func TestFindings_ReturnsTheList(t *testing.T) {
	fs := []model.Finding{{ID: "f1", RuleID: "R001"}, {ID: "f2", RuleID: "R002"}}
	s := New(&fakeLatestProvider{findings: fs}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/findings", nil))

	var got []model.Finding
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if len(got) != 2 || got[0].ID != "f1" || got[1].ID != "f2" {
		t.Errorf("decoded findings = %+v, want 2 findings f1, f2 in order", got)
	}
}

func TestClusters_NoPeers_ReturnsSelfAndEmptyPeers(t *testing.T) {
	s := New(&fakeLatestProvider{}, nil, "", "cluster-a", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clusters", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/clusters: status = %d, want 200", rec.Code)
	}
	var got model.ClustersInfo
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if got.Self != "cluster-a" {
		t.Errorf("Self = %q, want %q", got.Self, "cluster-a")
	}
	if got.Peers == nil || len(got.Peers) != 0 {
		t.Errorf("Peers = %+v, want a non-nil empty slice", got.Peers)
	}
}

func TestClusters_WithPeers_ReturnsThem(t *testing.T) {
	peers := []model.ClusterPeer{{ID: "cluster-b", URL: "http://localhost:8081"}}
	s := New(&fakeLatestProvider{}, nil, "", "cluster-a", peers, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clusters", nil))

	var got model.ClustersInfo
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if len(got.Peers) != 1 || got.Peers[0].ID != "cluster-b" || got.Peers[0].URL != "http://localhost:8081" {
		t.Errorf("Peers = %+v, want [{cluster-b http://localhost:8081}]", got.Peers)
	}
}

type fakeTimeSeriesProvider struct {
	points     []evidence.Point
	req, limit float64
	err        error
}

func (f *fakeTimeSeriesProvider) CPUUsageSeries(ctx context.Context, wl model.WorkloadRef, container string, window time.Duration, now time.Time) ([]evidence.Point, float64, float64, error) {
	return f.points, f.req, f.limit, f.err
}
func (f *fakeTimeSeriesProvider) MemoryUsageSeries(ctx context.Context, wl model.WorkloadRef, container string, window time.Duration, now time.Time) ([]evidence.Point, float64, float64, error) {
	return f.points, f.req, f.limit, f.err
}

func TestTimeseries_NoBuilderConfigured_501(t *testing.T) {
	s := New(&fakeLatestProvider{}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo&workload=w&container=c&metric=cpu", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501 when no TimeSeriesProvider is configured", rec.Code)
	}
}

func TestTimeseries_MissingParams_400(t *testing.T) {
	s := New(&fakeLatestProvider{}, &fakeTimeSeriesProvider{}, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing required params", rec.Code)
	}
}

func TestTimeseries_InvalidMetric_400(t *testing.T) {
	s := New(&fakeLatestProvider{}, &fakeTimeSeriesProvider{}, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo&workload=w&container=c&metric=disk", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unsupported metric", rec.Code)
	}
}

func TestTimeseries_Success(t *testing.T) {
	provider := &fakeTimeSeriesProvider{points: []evidence.Point{{UnixSeconds: 100, Value: 0.5}}, req: 1.0, limit: 2.0}
	s := New(&fakeLatestProvider{}, provider, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo&workload=w&container=c&metric=cpu", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Metric  string           `json:"metric"`
		Request float64          `json:"request"`
		Limit   float64          `json:"limit"`
		Points  []evidence.Point `json:"points"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Metric != "cpu" || body.Request != 1.0 || body.Limit != 2.0 || len(body.Points) != 1 {
		t.Errorf("decoded body = %+v, want metric=cpu request=1 limit=2 with 1 point", body)
	}
}

func TestTimeseries_QueryError_502NotCrash(t *testing.T) {
	provider := &fakeTimeSeriesProvider{err: errors.New("prometheus unreachable")}
	s := New(&fakeLatestProvider{}, provider, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo&workload=w&container=c&metric=memory", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 when the underlying query fails", rec.Code)
	}
}

func TestMetrics_Served(t *testing.T) {
	s := New(&fakeLatestProvider{run: nil}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics: status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("GET /metrics: body is empty, want the Prometheus text-exposition format")
	}
}

type fakeClusterSummary struct{}

func (fakeClusterSummary) Summary(_ context.Context, now time.Time) model.ClusterSummary {
	return model.ClusterSummary{ClusterID: "c1", GeneratedAt: now, Highlights: []string{}, Errors: []string{}}
}

func TestClusterSummary_NotConfigured_501(t *testing.T) {
	s := New(&fakeLatestProvider{}, nil, "", "test-cluster", nil, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/cluster/summary", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

func TestClusterSummary_Configured_200JSON(t *testing.T) {
	s := New(&fakeLatestProvider{}, nil, "", "test-cluster", nil, testLogger()).WithClusterSummary(fakeClusterSummary{})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/cluster/summary", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cluster_id":"c1"`) {
		t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTimeseries_ReportsTheRangeThatWasQueried(t *testing.T) {
	provider := &fakeTimeSeriesProvider{points: []evidence.Point{{UnixSeconds: 1791455700, Value: 0.5}}}
	s := New(&fakeLatestProvider{}, provider, "", "test-cluster", nil, testLogger())

	for _, tc := range []struct {
		query                string
		wantWindow, wantStep int64
	}{
		{"window=24h", 86400, 300},
		{"window=1h", 3600, 30},
		{"", 86400, 300},              // default is 24h
		{"window=1000h", 172800, 300}, // clamped to 48h, and the response says so
		{"window=nonsense", 86400, 300},
	} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/timeseries?namespace=demo&workload=w&container=c&metric=cpu&"+tc.query, nil))
		var body struct {
			Points        []evidence.Point `json:"points"`
			From          int64            `json:"from"`
			To            int64            `json:"to"`
			WindowSeconds int64            `json:"window_seconds"`
			StepSeconds   int64            `json:"step_seconds"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("%q: decoding: %v", tc.query, err)
		}
		if body.WindowSeconds != tc.wantWindow || body.StepSeconds != tc.wantStep {
			t.Errorf("%q: window=%ds step=%ds, want %ds / %ds", tc.query, body.WindowSeconds, body.StepSeconds, tc.wantWindow, tc.wantStep)
		}
		if body.To-body.From != tc.wantWindow {
			t.Errorf("%q: to-from = %d, want %d", tc.query, body.To-body.From, tc.wantWindow)
		}
		// Both ends sit on the step grid, so every point is a whole-second, reproducible instant.
		if body.To%tc.wantStep != 0 || body.From%tc.wantStep != 0 {
			t.Errorf("%q: from=%d to=%d not aligned to %ds", tc.query, body.From, body.To, tc.wantStep)
		}
		// Point timestamps pass through untouched: no offset, no unit change.
		if len(body.Points) != 1 || body.Points[0].UnixSeconds != 1791455700 {
			t.Errorf("%q: points = %+v, want t=1791455700 unchanged", tc.query, body.Points)
		}
	}
}
