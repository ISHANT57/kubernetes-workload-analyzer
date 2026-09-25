// Package k8sclient wraps client-go behind a small interface. Its method set is deliberately
// limited to what the D-007 ClusterRole grants (namespaces, nodes, deployments, replicasets,
// statefulsets, daemonsets, horizontalpodautoscalers, events -- get/list/watch only; never pods,
// secrets or configmaps, never a write verb). See docs/decisions/ADR-007-k8s-api-scope.md.
package k8sclient

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// Client is the analyzer's read-only access to the Kubernetes API.
type Client interface {
	// Healthy checks that the Kubernetes API is reachable and that this client's credentials
	// actually work, using the same permission the analysis loop relies on (listing
	// Deployments) rather than a separate endpoint that might be allowed by a different,
	// unrelated default RBAC binding.
	Healthy(ctx context.Context) error

	// ListDeployments lists every Deployment across every namespace. Phase 3 uses this only as
	// a connectivity/RBAC smoke test (its count becomes AnalysisRun.WorkloadsSeen); Phase 4's
	// evidence builder will call the equivalent StatefulSet/DaemonSet listers too and build
	// richer workload facts.
	ListDeployments(ctx context.Context) ([]model.WorkloadRef, error)
}

type clientsetClient struct {
	clusterID model.ClusterID
	clientset kubernetes.Interface
}

// New builds a Client. If kubeconfigPath and kubeContext are both empty, it tries in-cluster
// config first (Phase 7 deployment, where the pod's mounted ServiceAccount token is used), then
// falls back to the standard kubeconfig loading rules (KUBECONFIG env var, then ~/.kube/config)
// for local development.
func New(clusterID model.ClusterID, kubeconfigPath, kubeContext string) (Client, error) {
	restConfig, err := buildRESTConfig(kubeconfigPath, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("k8sclient: building REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("k8sclient: building clientset: %w", err)
	}

	return &clientsetClient{clusterID: clusterID, clientset: clientset}, nil
}

func buildRESTConfig(kubeconfigPath, kubeContext string) (*rest.Config, error) {
	if kubeconfigPath == "" && kubeContext == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
		// Not running in-cluster (or no ServiceAccount token mounted) -- fall through to
		// kubeconfig loading, which is the expected path during local development.
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

func (c *clientsetClient) Healthy(ctx context.Context) error {
	if _, err := c.ListDeployments(ctx); err != nil {
		return fmt.Errorf("k8sclient: health check failed: %w", err)
	}
	return nil
}

func (c *clientsetClient) ListDeployments(ctx context.Context) ([]model.WorkloadRef, error) {
	list, err := c.clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: listing deployments: %w", err)
	}
	return toWorkloadRefs(c.clusterID, list.Items), nil
}

func toWorkloadRefs(clusterID model.ClusterID, deployments []appsv1.Deployment) []model.WorkloadRef {
	refs := make([]model.WorkloadRef, 0, len(deployments))
	for _, d := range deployments {
		refs = append(refs, model.WorkloadRef{
			ClusterID: clusterID,
			Namespace: d.Namespace,
			Kind:      "Deployment",
			Name:      d.Name,
		})
	}
	return refs
}
