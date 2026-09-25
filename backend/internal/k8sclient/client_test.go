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

func TestListDeployments_ReturnsAllNamespaces(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "oom"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "learn", Name: "web"}},
	)
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	refs, err := c.ListDeployments(context.Background())
	if err != nil {
		t.Fatalf("ListDeployments(): unexpected error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("ListDeployments(): got %d refs, want 2", len(refs))
	}
	for _, r := range refs {
		if r.ClusterID != "test-cluster" {
			t.Errorf("ref %+v: ClusterID = %q, want test-cluster", r, r.ClusterID)
		}
		if r.Kind != "Deployment" {
			t.Errorf("ref %+v: Kind = %q, want Deployment", r, r.Kind)
		}
	}
}

func TestListDeployments_Empty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &clientsetClient{clusterID: "test-cluster", clientset: cs}

	refs, err := c.ListDeployments(context.Background())
	if err != nil {
		t.Fatalf("ListDeployments(): unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("ListDeployments() on an empty cluster: got %d refs, want 0", len(refs))
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
