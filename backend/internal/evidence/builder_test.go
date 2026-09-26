package evidence

import (
	"context"
	"strings"
	"testing"
	"time"

	promcommon "github.com/prometheus/common/model"

	analyzermodel "github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// recordingPromClient answers QueryInstant by substring match on the query text and records
// every query it was asked, so a test can both control the response and assert on the exact
// query shape issued for a given workload Kind (PR18: pod resolution must use a different owner
// chain per Kind).
type recordingPromClient struct {
	instant func(query string) (promcommon.Vector, error)
	rangeFn func(query string) (promcommon.Matrix, error)
	Queries []string
}

func (f *recordingPromClient) Healthy(ctx context.Context) error { return nil }
func (f *recordingPromClient) QueryInstant(ctx context.Context, query string) (promcommon.Vector, error) {
	f.Queries = append(f.Queries, query)
	if f.instant != nil {
		return f.instant(query)
	}
	return nil, nil
}
func (f *recordingPromClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (promcommon.Matrix, error) {
	f.Queries = append(f.Queries, query)
	if f.rangeFn != nil {
		return f.rangeFn(query)
	}
	return nil, nil
}

// --- resolvePodNames per Kind (PR18, workload discovery coverage) -----------------------------
//
// Deployment owns pods through an intermediate ReplicaSet (kube_pod_owner reports
// owner_kind="ReplicaSet" on the pod, not "Deployment"), so resolving it needs the two-hop join.
// StatefulSet and DaemonSet own their pods directly -- kube_pod_owner already reports the right
// owner_kind/owner_name straight on the pod -- so the two-hop join would structurally never
// match for them; a direct single query is both correct and necessary.

func TestResolvePodNames_Deployment_UsesTwoHopReplicaSetJoin(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "web-abc123"}}}, nil
		},
	}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "web"}

	names, err := b.resolvePodNames(context.Background(), workload)
	if err != nil {
		t.Fatalf("resolvePodNames(): unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "web-abc123" {
		t.Errorf("names = %v, want [web-abc123]", names)
	}
	if len(prom.Queries) != 1 {
		t.Fatalf("issued %d queries, want 1", len(prom.Queries))
	}
	q := prom.Queries[0]
	if !strings.Contains(q, "kube_pod_owner") || !strings.Contains(q, "kube_replicaset_owner") {
		t.Errorf("Deployment query = %q, want it to join kube_pod_owner to kube_replicaset_owner", q)
	}
	if !strings.Contains(q, `owner_name="web"`) {
		t.Errorf("Deployment query = %q, want it to reference owner_name=%q", q, "web")
	}
}

func TestResolvePodNames_StatefulSet_UsesDirectQuery_NoReplicaSetJoin(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "prometheus-0"}}}, nil
		},
	}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "monitoring", Kind: "StatefulSet", Name: "prometheus"}

	names, err := b.resolvePodNames(context.Background(), workload)
	if err != nil {
		t.Fatalf("resolvePodNames(): unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "prometheus-0" {
		t.Errorf("names = %v, want [prometheus-0]", names)
	}
	if len(prom.Queries) != 1 {
		t.Fatalf("issued %d queries, want 1 (a direct query, no join)", len(prom.Queries))
	}
	q := prom.Queries[0]
	if strings.Contains(q, "kube_replicaset_owner") {
		t.Errorf("StatefulSet query = %q, must not use the ReplicaSet join: a StatefulSet never creates one, so that join would structurally never match", q)
	}
	if !strings.Contains(q, `owner_kind="StatefulSet"`) || !strings.Contains(q, `owner_name="prometheus"`) {
		t.Errorf("StatefulSet query = %q, want owner_kind=%q and owner_name=%q", q, "StatefulSet", "prometheus")
	}
}

func TestResolvePodNames_DaemonSet_UsesDirectQuery_NoReplicaSetJoin(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "node-exporter-xyz"}}}, nil
		},
	}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "monitoring", Kind: "DaemonSet", Name: "node-exporter"}

	names, err := b.resolvePodNames(context.Background(), workload)
	if err != nil {
		t.Fatalf("resolvePodNames(): unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "node-exporter-xyz" {
		t.Errorf("names = %v, want [node-exporter-xyz]", names)
	}
	q := prom.Queries[0]
	if strings.Contains(q, "kube_replicaset_owner") {
		t.Errorf("DaemonSet query = %q, must not use the ReplicaSet join", q)
	}
	if !strings.Contains(q, `owner_kind="DaemonSet"`) || !strings.Contains(q, `owner_name="node-exporter"`) {
		t.Errorf("DaemonSet query = %q, want owner_kind=%q and owner_name=%q", q, "DaemonSet", "node-exporter")
	}
}

func TestResolvePodNames_UnsupportedKind_FailsClosed(t *testing.T) {
	prom := &recordingPromClient{}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "CronJob", Name: "backup"}

	_, err := b.resolvePodNames(context.Background(), workload)
	if err == nil {
		t.Fatal("resolvePodNames() with an unsupported Kind: want error, got nil")
	}
	if len(prom.Queries) != 0 {
		t.Errorf("issued %d queries for an unsupported Kind, want 0: it must fail before querying, not guess", len(prom.Queries))
	}
}

// TestResolveContainers_StatefulSetAndDaemonSet_ResolveThroughToContainers is an end-to-end
// check (through the public ResolveContainers, not just resolvePodNames) that a StatefulSet or
// DaemonSet workload reaches the container-listing query with the right pod selector -- the
// whole point of PR18 is that these two Kinds now work through the entire evidence path, not
// just the pod-name-resolution step in isolation.
func TestResolveContainers_StatefulSetAndDaemonSet_ResolveThroughToContainers(t *testing.T) {
	for _, tc := range []struct {
		kind, name, pod string
	}{
		{"StatefulSet", "prometheus", "prometheus-0"},
		{"DaemonSet", "node-exporter", "node-exporter-xyz"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			prom := &recordingPromClient{
				instant: func(query string) (promcommon.Vector, error) {
					switch {
					case strings.Contains(query, "kube_pod_owner"):
						return promcommon.Vector{{Metric: promcommon.Metric{"pod": promcommon.LabelValue(tc.pod)}}}, nil
					case strings.Contains(query, "count by (container)"):
						return promcommon.Vector{{Metric: promcommon.Metric{"container": "c"}}}, nil
					default:
						return nil, nil
					}
				},
			}
			b := &Builder{Prom: prom}
			workload := analyzermodel.WorkloadRef{Namespace: "monitoring", Kind: tc.kind, Name: tc.name}

			containers, err := b.ResolveContainers(context.Background(), workload)
			if err != nil {
				t.Fatalf("ResolveContainers(): unexpected error: %v", err)
			}
			if len(containers) != 1 || containers[0] != "c" {
				t.Errorf("containers = %v, want [c]", containers)
			}
		})
	}
}
