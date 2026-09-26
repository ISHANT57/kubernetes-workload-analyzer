package k8sclient

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestListWorkloads_AllThreeKinds is the PR18 coverage test: Deployments, StatefulSets and
// DaemonSets must all be discovered and tagged with the correct Kind, not just Deployments.
func TestListWorkloads_AllThreeKinds(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "oom"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "learn", Name: "web"}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "monitoring", Name: "prometheus"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: "monitoring", Name: "node-exporter"}},
	)
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	refs, err := c.ListWorkloads(context.Background())
	if err != nil {
		t.Fatalf("ListWorkloads(): unexpected error: %v", err)
	}
	if len(refs) != 4 {
		t.Fatalf("ListWorkloads(): got %d refs, want 4", len(refs))
	}

	kindByName := map[string]string{}
	for _, r := range refs {
		if r.ClusterID != "test-cluster" {
			t.Errorf("ref %+v: ClusterID = %q, want test-cluster", r, r.ClusterID)
		}
		kindByName[r.Name] = r.Kind
	}
	want := map[string]string{"oom": "Deployment", "web": "Deployment", "prometheus": "StatefulSet", "node-exporter": "DaemonSet"}
	for name, wantKind := range want {
		if got := kindByName[name]; got != wantKind {
			t.Errorf("Kind for %q = %q, want %q", name, got, wantKind)
		}
	}
}

func TestListWorkloads_Empty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	refs, err := c.ListWorkloads(context.Background())
	if err != nil {
		t.Fatalf("ListWorkloads(): unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("ListWorkloads() on an empty cluster: got %d refs, want 0", len(refs))
	}
}

// TestListWorkloads_StatefulSetListingFails_WholeCallFails documents the deliberate design
// choice (PR18): a failure listing any one of the three kinds fails ListWorkloads as a whole,
// matching the runner's single "kubernetes" top-level source -- this must stay one source, not
// three, so internal/runner's failed/partial accounting does not need to change.
func TestListWorkloads_StatefulSetListingFails_WholeCallFails(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "oom"}})
	cs.PrependReactor("list", "statefulsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("statefulsets is forbidden: User cannot list resource")
	})
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	if _, err := c.ListWorkloads(context.Background()); err == nil {
		t.Error("ListWorkloads() with statefulsets listing forbidden: want error, got nil")
	}
}

func TestHealthy_PropagatesUnderlyingError(t *testing.T) {
	// Simulates the D-007 boundary being violated or a real RBAC Forbidden response -- this is
	// the "insufficient permissions" case AGENTS.md requires explicit handling for.
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("deployments is forbidden: User cannot list resource")
	})
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	if err := c.Healthy(context.Background()); err == nil {
		t.Error("Healthy() with a Forbidden underlying error: want error, got nil")
	}
}
