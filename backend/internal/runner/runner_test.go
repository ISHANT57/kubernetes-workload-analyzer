package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	analyzermodel "github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// fakePromClient and fakeK8sClient let these tests exercise the runner's failure handling
// without a real Prometheus or Kubernetes cluster (AGENTS.md: "External calls ... sit behind
// interfaces").
type fakePromClient struct {
	healthyErr error
}

func (f *fakePromClient) Healthy(ctx context.Context) error { return f.healthyErr }
func (f *fakePromClient) QueryInstant(ctx context.Context, query string) (model.Vector, error) {
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
	r := New("test-cluster", 0, time.Second, time.Second, prom, k8s, testLogger())

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
	r := New("test-cluster", 0, time.Second, time.Second, prom, k8s, testLogger())

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
	r := New("test-cluster", 0, time.Second, time.Second, prom, k8s, testLogger())

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
	r := New("test-cluster", 0, time.Second, time.Second, prom, k8s, testLogger())

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
	r := New("test-cluster", 0, time.Second, time.Second, prom, k8s, testLogger())

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
	r := New("test-cluster", 0, time.Second, time.Second, &fakePromClient{}, &fakeK8sClient{}, testLogger())
	if got := r.Latest(); got != nil {
		t.Errorf("Latest() before any run: got %+v, want nil", got)
	}
}
