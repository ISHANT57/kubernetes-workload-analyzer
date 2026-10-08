package clustersummary

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	prommodel "github.com/prometheus/common/model"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

func f(v float64) *float64 { return &v }

func joined(h []string) string { return strings.Join(h, " | ") }

func TestHighlights_NilMetricsProduceNothing(t *testing.T) {
	if h := Highlights(model.ClusterSummary{}); len(h) != 0 {
		t.Errorf("want no highlights for an empty summary, got %v", h)
	}
}

func TestHighlights_RequestPressureAtThreshold(t *testing.T) {
	at := Highlights(model.ClusterSummary{CPUAllocatableCores: f(10), CPURequestedCores: f(8)})
	if !strings.Contains(joined(at), "CPU requests use 80% of allocatable") {
		t.Errorf("want pressure highlight at exactly 80%%, got %v", at)
	}
	below := Highlights(model.ClusterSummary{CPUAllocatableCores: f(10), CPURequestedCores: f(7.9)})
	if strings.Contains(joined(below), "allocatable") {
		t.Errorf("79%% must not trigger, got %v", below)
	}
}

func TestHighlights_UnderUseNeedsMeaningfulRequest(t *testing.T) {
	h := Highlights(model.ClusterSummary{CPURequestedCores: f(2), CPUUsageCores: f(0.4)})
	if !strings.Contains(joined(h), "Pods use 20% of the CPU they request") {
		t.Errorf("got %v", h)
	}
	tiny := Highlights(model.ClusterSummary{CPURequestedCores: f(0.05), CPUUsageCores: f(0.001)})
	if len(tiny) != 0 {
		t.Errorf("a request below the minimum is noise, got %v", tiny)
	}
}

func TestHighlights_HealthSignals(t *testing.T) {
	h := joined(Highlights(model.ClusterSummary{
		Nodes: f(3), NodesReady: f(2), PodsPending: f(4), PodsFailed: f(1), Restarts24h: f(12),
	}))
	for _, want := range []string{"1 node is not Ready (1 of 3)", "4 pods are Pending", "1 pod is in the Failed phase", "12 times in the last 24 hours"} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %q in %q", want, h)
		}
	}
}

type fakeProm struct {
	calls int
	fail  map[string]bool
	empty map[string]bool
}

func (p *fakeProm) QueryInstant(_ context.Context, q string) (prommodel.Vector, error) {
	p.calls++
	for k := range p.fail {
		if strings.Contains(q, k) {
			return nil, errors.New("boom")
		}
	}
	for k := range p.empty {
		if strings.Contains(q, k) {
			return prommodel.Vector{}, nil
		}
	}
	return prommodel.Vector{{Value: 5}}, nil
}

func TestProvider_FailedAndEmptyQueriesLeaveNilAndAreListed(t *testing.T) {
	p := NewProvider(&fakeProm{fail: map[string]bool{"kube_node_info": true}, empty: map[string]bool{"phase=\"Failed\"": true}}, "c1")
	s := p.Summary(context.Background(), time.Unix(1000, 0))
	if s.Nodes != nil || s.PodsFailed != nil {
		t.Error("failed/empty queries must leave the field nil, not zero")
	}
	if s.PodsRunning == nil || *s.PodsRunning != 5 {
		t.Error("other fields must still be filled")
	}
	e := joined(s.Errors)
	if !strings.Contains(e, "nodes: query failed") || !strings.Contains(e, "pods_failed: no data returned") {
		t.Errorf("errors = %v", s.Errors)
	}
	if s.Highlights == nil || s.Errors == nil {
		t.Error("lists must be non-nil so JSON carries [] not null")
	}
}

func TestProvider_CachesWithinTTL(t *testing.T) {
	fp := &fakeProm{}
	p := NewProvider(fp, "c1")
	t0 := time.Unix(1000, 0)
	p.Summary(context.Background(), t0)
	first := fp.calls
	p.Summary(context.Background(), t0.Add(cacheTTL-time.Second))
	if fp.calls != first {
		t.Errorf("within TTL must not query again: %d -> %d", first, fp.calls)
	}
	p.Summary(context.Background(), t0.Add(cacheTTL+time.Second))
	if fp.calls != first*2 {
		t.Errorf("after TTL must query again: %d", fp.calls)
	}
}
