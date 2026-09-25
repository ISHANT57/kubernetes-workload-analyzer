package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
	s := New(&fakeLatestProvider{run: nil}, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz with no run yet: status = %d, want 200", rec.Code)
	}
}

func TestReadyz_NoRunYet_NotReady(t *testing.T) {
	s := New(&fakeLatestProvider{run: nil}, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz before any run: status = %d, want 503", rec.Code)
	}
}

func TestReadyz_CompleteRun_Ready(t *testing.T) {
	s := New(&fakeLatestProvider{run: &model.AnalysisRun{Status: model.RunComplete}}, testLogger())
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
	s := New(&fakeLatestProvider{run: run}, testLogger())
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
	s := New(&fakeLatestProvider{run: nil}, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/latest", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/runs/latest before any run: status = %d, want 503", rec.Code)
	}
}

func TestLatestRun_ReturnsTheRun(t *testing.T) {
	run := &model.AnalysisRun{ID: "run-1", Status: model.RunComplete, WorkloadsSeen: 9}
	s := New(&fakeLatestProvider{run: run}, testLogger())
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
	s := New(&fakeLatestProvider{findings: nil}, testLogger())
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
	s := New(&fakeLatestProvider{findings: fs}, testLogger())
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

func TestMetrics_Served(t *testing.T) {
	s := New(&fakeLatestProvider{run: nil}, testLogger())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics: status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("GET /metrics: body is empty, want the Prometheus text-exposition format")
	}
}
