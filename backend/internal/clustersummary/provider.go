package clustersummary

import (
	"context"
	"fmt"
	"sync"
	"time"

	prommodel "github.com/prometheus/common/model"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/metrics"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// Querier is the one Prometheus capability this package needs; promclient.Client satisfies it.
type Querier interface {
	QueryInstant(ctx context.Context, query string) (prommodel.Vector, error)
}

const cacheTTL = 15 * time.Second

// Provider runs the fixed queries below. The browser never supplies PromQL. Results are cached
// for cacheTTL so a page polling every few seconds, or several open tabs, cost one set of
// queries, not one per request.
type Provider struct {
	prom      Querier
	clusterID model.ClusterID

	mu      sync.Mutex
	cached  model.ClusterSummary
	fetched time.Time
}

func NewProvider(prom Querier, clusterID model.ClusterID) *Provider {
	return &Provider{prom: prom, clusterID: clusterID}
}

type metricQuery struct {
	name  string
	query string
	dst   func(*model.ClusterSummary) **float64
}

// Requested totals leave out pods the scheduler marked unschedulable: they hold no node capacity,
// so counting their requests would overstate how full the nodes are.
var queries = []metricQuery{
	{"nodes", `count(kube_node_info)`, func(s *model.ClusterSummary) **float64 { return &s.Nodes }},
	{"nodes_ready", `sum(kube_node_status_condition{condition="Ready",status="true"})`, func(s *model.ClusterSummary) **float64 { return &s.NodesReady }},
	{"namespaces", `count(kube_namespace_status_phase{phase="Active"})`, func(s *model.ClusterSummary) **float64 { return &s.Namespaces }},
	{"pods_running", `sum(kube_pod_status_phase{phase="Running"})`, func(s *model.ClusterSummary) **float64 { return &s.PodsRunning }},
	{"pods_pending", `sum(kube_pod_status_phase{phase="Pending"})`, func(s *model.ClusterSummary) **float64 { return &s.PodsPending }},
	{"pods_failed", `sum(kube_pod_status_phase{phase="Failed"})`, func(s *model.ClusterSummary) **float64 { return &s.PodsFailed }},
	{"cpu_allocatable", `sum(kube_node_status_allocatable{resource="cpu"})`, func(s *model.ClusterSummary) **float64 { return &s.CPUAllocatableCores }},
	{"cpu_requested", `sum(kube_pod_container_resource_requests{resource="cpu"} unless on(namespace, pod) (kube_pod_status_unschedulable == 1))`, func(s *model.ClusterSummary) **float64 { return &s.CPURequestedCores }},
	{"cpu_usage", `sum(rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m]))`, func(s *model.ClusterSummary) **float64 { return &s.CPUUsageCores }},
	{"mem_allocatable", `sum(kube_node_status_allocatable{resource="memory"})`, func(s *model.ClusterSummary) **float64 { return &s.MemAllocatableBytes }},
	{"mem_requested", `sum(kube_pod_container_resource_requests{resource="memory"} unless on(namespace, pod) (kube_pod_status_unschedulable == 1))`, func(s *model.ClusterSummary) **float64 { return &s.MemRequestedBytes }},
	{"mem_usage", `sum(container_memory_working_set_bytes{container!="",container!="POD"})`, func(s *model.ClusterSummary) **float64 { return &s.MemUsageBytes }},
	{"restarts_24h", `sum(increase(kube_pod_container_status_restarts_total[24h]))`, func(s *model.ClusterSummary) **float64 { return &s.Restarts24h }},
}

// Summary returns the cached snapshot if it is fresh, else queries Prometheus. It never returns
// an error: a failed or empty query leaves that field nil and is listed in Errors, and is counted
// in analyzer_query_errors_total{source="prometheus"}.
func (p *Provider) Summary(ctx context.Context, now time.Time) model.ClusterSummary {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.fetched.IsZero() && now.Sub(p.fetched) < cacheTTL {
		return p.cached
	}

	s := model.ClusterSummary{ClusterID: p.clusterID, GeneratedAt: now, Errors: []string{}}
	for _, q := range queries {
		vec, err := p.prom.QueryInstant(ctx, q.query)
		switch {
		case err != nil:
			metrics.QueryErrorsTotal.WithLabelValues("prometheus").Inc()
			s.Errors = append(s.Errors, fmt.Sprintf("%s: query failed", q.name))
		case len(vec) == 0:
			s.Errors = append(s.Errors, fmt.Sprintf("%s: no data returned", q.name))
		default:
			v := float64(vec[0].Value)
			*q.dst(&s) = &v
		}
	}
	s.Highlights = Highlights(s)
	if s.Highlights == nil {
		s.Highlights = []string{}
	}
	p.cached, p.fetched = s, now
	return s
}
