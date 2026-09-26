package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/findings"
	analyzermodel "github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// fakePromClient and fakeK8sClient let these tests exercise the runner's failure handling
// without a real Prometheus or Kubernetes cluster (AGENTS.md: "External calls ... sit behind
// interfaces"). instant, when set, scripts QueryInstant's answers by query text -- used by the
// partial-workload-failure tests below to wire a real findings.Analyzer/evidence.Builder through
// this fake, without a real cluster.
type fakePromClient struct {
	healthyErr error
	instant    func(query string) (model.Vector, error)
}

func (f *fakePromClient) Healthy(ctx context.Context) error { return f.healthyErr }
func (f *fakePromClient) QueryInstant(ctx context.Context, query string) (model.Vector, error) {
	if f.instant != nil {
		return f.instant(query)
	}
	return nil, nil
}
func (f *fakePromClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (model.Matrix, error) {
	return nil, nil
}

type fakeK8sClient struct {
	refs    []analyzermodel.WorkloadRef
	listErr error
}

func (f *fakeK8sClient) Healthy(ctx context.Context) error { return f.listErr }
func (f *fakeK8sClient) ListDeployments(ctx context.Context) ([]analyzermodel.WorkloadRef, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.refs, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunOnce_BothHealthy_Complete(t *testing.T) {
	prom := &fakePromClient{}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{{Name: "a"}, {Name: "b"}}}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, nil, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got == nil {
		t.Fatal("Latest(): got nil after runOnce")
	}
	if got.Status != analyzermodel.RunComplete {
		t.Errorf("Status = %q, want %q", got.Status, analyzermodel.RunComplete)
	}
	if got.WorkloadsSeen != 2 {
		t.Errorf("WorkloadsSeen = %d, want 2", got.WorkloadsSeen)
	}
	if len(got.QueryErrors) != 0 {
		t.Errorf("QueryErrors = %v, want none", got.QueryErrors)
	}
}

// TestRunOnce_PrometheusDown is the direct test of the Phase 3 done-criterion "degrades cleanly
// with Prometheus down": the run must still produce a usable snapshot (Kubernetes data is still
// good), marked partial, with the Prometheus failure recorded -- not a crash, not a run that
// silently looks identical to a fully healthy one.
func TestRunOnce_PrometheusDown(t *testing.T) {
	prom := &fakePromClient{healthyErr: errors.New("dial tcp: connection refused")}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{{Name: "a"}}}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, nil, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got == nil {
		t.Fatal("Latest(): got nil after runOnce")
	}
	if got.Status != analyzermodel.RunPartial {
		t.Errorf("Status = %q, want %q (Kubernetes still succeeded)", got.Status, analyzermodel.RunPartial)
	}
	if got.WorkloadsSeen != 1 {
		t.Errorf("WorkloadsSeen = %d, want 1 (Kubernetes data must still be used)", got.WorkloadsSeen)
	}
	if len(got.QueryErrors) != 1 || got.QueryErrors[0].Source != "prometheus" {
		t.Errorf("QueryErrors = %v, want exactly one entry with source=prometheus", got.QueryErrors)
	}
}

func TestRunOnce_KubernetesDown(t *testing.T) {
	prom := &fakePromClient{}
	k8s := &fakeK8sClient{listErr: errors.New("Forbidden")}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, nil, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunPartial {
		t.Errorf("Status = %q, want %q", got.Status, analyzermodel.RunPartial)
	}
	if got.WorkloadsSeen != 0 {
		t.Errorf("WorkloadsSeen = %d, want 0", got.WorkloadsSeen)
	}
	if len(got.QueryErrors) != 1 || got.QueryErrors[0].Source != "kubernetes" {
		t.Errorf("QueryErrors = %v, want exactly one entry with source=kubernetes", got.QueryErrors)
	}
}

func TestRunOnce_BothDown_Failed(t *testing.T) {
	prom := &fakePromClient{healthyErr: errors.New("unreachable")}
	k8s := &fakeK8sClient{listErr: errors.New("unreachable")}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, nil, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunFailed {
		t.Errorf("Status = %q, want %q", got.Status, analyzermodel.RunFailed)
	}
	if len(got.QueryErrors) != 2 {
		t.Errorf("QueryErrors = %v, want 2 entries", got.QueryErrors)
	}
}

// TestLatest_WorkloadsSeenSurvivesAFailedRun is the other half of "degrades cleanly": once a
// good Kubernetes listing has happened, a later run whose listing fails must not report
// WorkloadsSeen=0 -- that would look identical to "the cluster suddenly has zero workloads"
// instead of "we could not reach Kubernetes this cycle". docs/architecture.md: "a failed run
// keeps the previous snapshot, marked stale". The run's own Status/QueryErrors still truthfully
// mark this cycle as degraded; only the count is carried forward.
func TestLatest_WorkloadsSeenSurvivesAFailedRun(t *testing.T) {
	prom := &fakePromClient{}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, nil, testLogger())

	r.runOnce(context.Background()) // good run
	firstID := r.Latest().ID
	if r.Latest().WorkloadsSeen != 3 {
		t.Fatalf("setup: WorkloadsSeen = %d, want 3", r.Latest().WorkloadsSeen)
	}

	// Now Kubernetes access breaks (Prometheus stays healthy, to isolate the K8s-side behaviour).
	k8s.listErr = errors.New("connection refused")
	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunPartial {
		t.Fatalf("Status after second run = %q, want %q", got.Status, analyzermodel.RunPartial)
	}
	if got.ID == firstID {
		t.Error("Latest().ID did not change: runOnce must still record that a (degraded) run happened")
	}
	if got.WorkloadsSeen != 3 {
		t.Errorf("WorkloadsSeen after a failed K8s listing = %d, want 3 (carried forward from the last good run)", got.WorkloadsSeen)
	}
	if len(got.QueryErrors) != 1 || got.QueryErrors[0].Source != "kubernetes" {
		t.Errorf("QueryErrors = %v, want exactly one kubernetes entry (the run must still say this cycle failed)", got.QueryErrors)
	}
}

func TestLatest_NilBeforeFirstRun(t *testing.T) {
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, &fakePromClient{}, &fakeK8sClient{}, nil, testLogger())
	if got := r.Latest(); got != nil {
		t.Errorf("Latest() before any run: got %+v, want nil", got)
	}
}

// --- Partial workload-level failures (both sources reachable, some workloads' evidence isn't) ---
//
// The bug: Analyze()'s per-workload failures were only logged, never folded into the run's
// QueryErrors/Status, so a run with some workloads failing and others succeeding still reported
// "complete" -- indistinguishable from a fully clean run and invisible to /readyz. These tests
// wire a real findings.Analyzer and evidence.Builder through the fakes above (not a mock of
// Analyze itself), so they exercise the actual runner -> findings -> evidence path, not just the
// runner's bookkeeping in isolation.

func TestRunOnce_PartialWorkloadFailure_ReportsPartialNotComplete(t *testing.T) {
	prom := &fakePromClient{instant: scriptedInstant("broken", true)}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "broken"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "healthy"},
	}}
	a := &findings.Analyzer{Builder: &evidence.Builder{Prom: prom}, Logger: testLogger()}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, a, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunPartial {
		t.Errorf("Status = %q, want %q: prometheus/kubernetes both reachable, but one workload's evidence failed", got.Status, analyzermodel.RunPartial)
	}
	foundEvidenceErr := false
	for _, e := range got.QueryErrors {
		if e.Source == "evidence" {
			foundEvidenceErr = true
		}
	}
	if !foundEvidenceErr {
		t.Errorf("QueryErrors = %v, want an entry with source=evidence for the broken workload", got.QueryErrors)
	}
}

func TestRunOnce_PartialWorkloadFailure_SuccessfulWorkloadFindingsPreserved(t *testing.T) {
	prom := &fakePromClient{instant: scriptedInstant("broken", true)}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "broken"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "healthy"},
	}}
	a := &findings.Analyzer{Builder: &evidence.Builder{Prom: prom}, Logger: testLogger()}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, a, testLogger())

	r.runOnce(context.Background())

	fs := r.Findings()
	if len(fs) == 0 {
		t.Fatal("Findings() = [], want at least one from the healthy workload despite the broken one failing")
	}
	for _, f := range fs {
		if f.Workload.Name == "broken" {
			t.Errorf("got a finding for the broken workload: %+v", f)
		}
	}
}

func TestRunOnce_AllWorkloadsSucceed_ReportsComplete(t *testing.T) {
	prom := &fakePromClient{instant: scriptedInstant("", true)}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "a"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "b"},
	}}
	a := &findings.Analyzer{Builder: &evidence.Builder{Prom: prom}, Logger: testLogger()}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, a, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunComplete {
		t.Errorf("Status = %q, want %q", got.Status, analyzermodel.RunComplete)
	}
	if len(got.QueryErrors) != 0 {
		t.Errorf("QueryErrors = %v, want none", got.QueryErrors)
	}
}

// TestRunOnce_PrometheusDown_WorkloadFailuresDontMatter is the regression check for "existing
// Prometheus/Kubernetes outage behavior does not regress": when Prometheus itself is down,
// Analyze never runs at all (the runner's existing gate below), so workload-level accounting
// must not change a full-outage run's Status from what TestRunOnce_PrometheusDown already
// asserts -- this just re-confirms it with a real (nil) Analyzer wired in, not just nil.
func TestRunOnce_PrometheusDown_AnalyzerConfigured_StillPartial(t *testing.T) {
	prom := &fakePromClient{healthyErr: errors.New("dial tcp: connection refused"), instant: scriptedInstant("", true)}
	k8s := &fakeK8sClient{refs: []analyzermodel.WorkloadRef{{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "a"}}}
	a := &findings.Analyzer{Builder: &evidence.Builder{Prom: prom}, Logger: testLogger()}
	r := New("test-cluster", 0, time.Second, time.Second, time.Second, prom, k8s, a, testLogger())

	r.runOnce(context.Background())

	got := r.Latest()
	if got.Status != analyzermodel.RunPartial {
		t.Errorf("Status = %q, want %q (Kubernetes still succeeded; Analyze must not have run at all)", got.Status, analyzermodel.RunPartial)
	}
	if len(got.QueryErrors) != 1 || got.QueryErrors[0].Source != "prometheus" {
		t.Errorf("QueryErrors = %v, want exactly one entry with source=prometheus -- Analyze must be skipped entirely during a full source outage, not run and add evidence errors on top", got.QueryErrors)
	}
}

// scriptedInstant answers the query shapes evidence.Builder issues, matched by substring so it
// stays correct if the exact PromQL text changes: pod resolution (shared by ResolveContainers
// and Build) fails for failWorkload if set, otherwise every workload resolves to one pod and one
// container, and -- when fireRestarts is true -- a restart count above R003's threshold, so a
// "successful" workload really produces a finding rather than just avoiding an error.
func scriptedInstant(failWorkload string, fireRestarts bool) func(query string) (model.Vector, error) {
	return func(query string) (model.Vector, error) {
		switch {
		case failWorkload != "" && strings.Contains(query, `owner_name="`+failWorkload+`"`):
			return nil, errors.New("dial tcp: connection refused")
		case strings.Contains(query, "kube_pod_owner"):
			return model.Vector{{Metric: model.Metric{"pod": "pod-1"}, Value: 1}}, nil
		case strings.Contains(query, "count by (container)"):
			return model.Vector{{Metric: model.Metric{"container": "c"}, Value: 1}}, nil
		case fireRestarts && strings.Contains(query, "kube_pod_container_status_restarts_total"):
			return model.Vector{{Value: 5}}, nil
		default:
			return nil, nil
		}
	}
}
