// Package k8sclient wraps client-go behind a small interface. Its method set is deliberately
// limited to what the D-007 ClusterRole grants (namespaces, nodes, deployments, replicasets,
// statefulsets, daemonsets, horizontalpodautoscalers, events -- get/list/watch only; never pods,
// secrets or configmaps, never a write verb). See docs/decisions/ADR-007-k8s-api-scope.md.
package k8sclient

import (
	"context"
	"fmt"

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
	// workloads) rather than a separate endpoint that might be allowed by a different,
	// unrelated default RBAC binding.
	Healthy(ctx context.Context) error

	// ListWorkloads lists every Deployment, StatefulSet and DaemonSet across every namespace
	// (PR18, workload discovery coverage -- Phase 3/4 only listed Deployments). Its count
	// becomes AnalysisRun.WorkloadsSeen, and the evidence builder resolves each returned
	// WorkloadRef's live pods according to its Kind (internal/evidence's resolvePodNames). A
	// failure listing any one of the three kinds fails the whole call, matching the existing
	// "kubernetes" top-level source in internal/runner -- this stays one source, not three, so
	// the runner's failed/partial accounting does not need to change.
	ListWorkloads(ctx context.Context) ([]model.WorkloadRef, error)
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
	if _, err := c.ListWorkloads(ctx); err != nil {
		return fmt.Errorf("k8sclient: health check failed: %w", err)
	}
	return nil
}

func (c *clientsetClient) ListWorkloads(ctx context.Context) ([]model.WorkloadRef, error) {
	deployments, err := c.clientset.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: listing deployments: %w", err)
	}
	statefulSets, err := c.clientset.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: listing statefulsets: %w", err)
	}
	daemonSets, err := c.clientset.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: listing daemonsets: %w", err)
	}

	refs := make([]model.WorkloadRef, 0, len(deployments.Items)+len(statefulSets.Items)+len(daemonSets.Items))
	for _, d := range deployments.Items {
		refs = append(refs, model.WorkloadRef{ClusterID: c.clusterID, Namespace: d.Namespace, Kind: "Deployment", Name: d.Name})
	}
	for _, s := range statefulSets.Items {
		refs = append(refs, model.WorkloadRef{ClusterID: c.clusterID, Namespace: s.Namespace, Kind: "StatefulSet", Name: s.Name})
	}
	for _, ds := range daemonSets.Items {
		refs = append(refs, model.WorkloadRef{ClusterID: c.clusterID, Namespace: ds.Namespace, Kind: "DaemonSet", Name: ds.Name})
	}
	return refs, nil
}
