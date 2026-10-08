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

	// The range of the most recent QueryRange call, so a test can assert the exact window queried.
	LastStart, LastEnd time.Time
	LastStep           time.Duration
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
	f.LastStart, f.LastEnd, f.LastStep = start, end, step
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

// --- buildScheduling (R006 evidence) ------------------------------------------------------

func TestBuild_PendingAndUnschedulable_BothTrue(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			switch {
			case strings.Contains(query, "kube_pod_owner"):
				return promcommon.Vector{{Metric: promcommon.Metric{"pod": "pending-abc"}}}, nil
			case strings.Contains(query, `kube_pod_status_phase`) && strings.Contains(query, `phase="Pending"`):
				return promcommon.Vector{{Value: 1}}, nil
			case strings.Contains(query, "kube_pod_status_unschedulable"):
				return promcommon.Vector{{Value: 1}}, nil
			default:
				return nil, nil
			}
		},
	}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "pending"}

	ev := b.Build(context.Background(), workload, "c", time.Now())

	if !ev.Scheduling.Pending {
		t.Error("Scheduling.Pending = false, want true")
	}
	if !ev.Scheduling.Unschedulable {
		t.Error("Scheduling.Unschedulable = false, want true")
	}
}

func TestBuild_SchedulableRunningPod_BothFalse(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			switch {
			case strings.Contains(query, "kube_pod_owner"):
				return promcommon.Vector{{Metric: promcommon.Metric{"pod": "running-abc"}}}, nil
			default:
				// A Running, schedulable pod: the Pending-phase query returns no series (phase
				// filter matches nothing) and the unschedulable gauge is simply absent -- both
				// legitimate "false" outcomes, not errors.
				return nil, nil
			}
		},
	}
	b := &Builder{Prom: prom}
	workload := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "right-sized"}

	ev := b.Build(context.Background(), workload, "c", time.Now())

	if ev.Scheduling.Pending || ev.Scheduling.Unschedulable {
		t.Errorf("Scheduling = %+v, want both false for a normal running pod", ev.Scheduling)
	}
}

// --- time model: UTC instants in, the same instants out ---------------------------------------
//
// A series' timestamps must reach the API as the exact Unix instants Prometheus reported: no zone
// offset baked in, no unit change. The browser alone converts to a local zone, for display.

func TestMatrixToPoints_PreservesExactUnixSeconds(t *testing.T) {
	// 10:35:00 UTC on 2026-10-08. The same instant is 16:05 IST; the API must carry 1791455700,
	// never 1791455700 +/- 19800.
	const want = int64(1791455700)
	matrix := promcommon.Matrix{{
		Metric: promcommon.Metric{"pod": "p"},
		Values: []promcommon.SamplePair{
			{Timestamp: promcommon.TimeFromUnixNano(want * int64(time.Second)), Value: 0.25},
			{Timestamp: promcommon.TimeFromUnixNano((want + 300) * int64(time.Second)), Value: 0.5},
		},
	}}
	got := matrixToPoints(matrix)
	if len(got) != 2 || got[0].UnixSeconds != want || got[1].UnixSeconds != want+300 {
		t.Fatalf("points = %+v, want exact instants %d and %d", got, want, want+300)
	}
	if utc := time.Unix(got[0].UnixSeconds, 0).UTC().Format("15:04"); utc != "10:35" {
		t.Errorf("instant renders as %s UTC, want 10:35", utc)
	}
}

func TestMatrixToPoints_SubSecondTimestampTruncatesNotRounds(t *testing.T) {
	// Prometheus samples carry milliseconds (e.g. 10:35:00.999). Unix() truncates, so a sample is
	// never reported a second late.
	ts := promcommon.TimeFromUnixNano(1791455700*int64(time.Second) + 999*int64(time.Millisecond))
	got := matrixToPoints(promcommon.Matrix{{Values: []promcommon.SamplePair{{Timestamp: ts, Value: 1}}}})
	if got[0].UnixSeconds != 1791455700 {
		t.Errorf("t = %d, want 1791455700", got[0].UnixSeconds)
	}
}

func TestCPUUsageSeries_QueriesTheRequestedWindowEndingNow(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "web-abc"}, Value: 1}}, nil
		},
	}
	b := &Builder{Prom: prom}
	now := time.Date(2026, 10, 8, 10, 37, 41, 482_000_000, time.UTC) // deliberately off-grid, with milliseconds
	wl := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "web"}

	for _, tc := range []struct {
		name   string
		window time.Duration
		want   time.Duration
		step   time.Duration
	}{
		{"1h", time.Hour, time.Hour, 30 * time.Second},
		{"24h is really 24h", 24 * time.Hour, 24 * time.Hour, 5 * time.Minute},
		{"over the cap is clamped to 48h", 90 * time.Hour, 48 * time.Hour, 5 * time.Minute},
		{"non-positive falls back to 48h", 0, 48 * time.Hour, 5 * time.Minute},
	} {
		if _, _, _, err := b.CPUUsageSeries(context.Background(), wl, "c", tc.window, now); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		// The end is now aligned down to the step grid (epoch-based), never later than now and
		// less than one step before it.
		if prom.LastEnd.After(now) || now.Sub(prom.LastEnd) >= tc.step || prom.LastEnd.UnixNano()%int64(tc.step) != 0 {
			t.Errorf("%s: end = %v, want now (%v) aligned down to a multiple of %v", tc.name, prom.LastEnd, now, tc.step)
		}
		if got := prom.LastEnd.Sub(prom.LastStart); got != tc.want {
			t.Errorf("%s: queried %v, want %v", tc.name, got, tc.want)
		}
		if prom.LastStep != tc.step {
			t.Errorf("%s: step = %v, want %v", tc.name, prom.LastStep, tc.step)
		}
	}
}

func TestSeriesRange_AlignsToTheStepGridAsWholeSecondUTCInstants(t *testing.T) {
	// 10:37:41.482 UTC. With a 5m step the grid end is 10:35:00 UTC = 16:05 IST, a whole second.
	now := time.Date(2026, 10, 8, 10, 37, 41, 482_000_000, time.UTC)
	start, end, step := SeriesRange(now, 24*time.Hour)
	if step != 5*time.Minute {
		t.Fatalf("step = %v, want 5m", step)
	}
	if end.Unix() != 1791455700 || end.Nanosecond() != 0 {
		t.Errorf("end = %d.%09d, want exactly 1791455700 (10:35:00Z)", end.Unix(), end.Nanosecond())
	}
	if end.Sub(start) != 24*time.Hour {
		t.Errorf("span = %v, want exactly the 24h window", end.Sub(start))
	}
	// Zone independence: the same instant described in IST aligns to the same end.
	ist := time.FixedZone("IST", 5*3600+30*60)
	_, endIST, _ := SeriesRange(now.In(ist), 24*time.Hour)
	if !endIST.Equal(end) {
		t.Errorf("end differs by zone: %v vs %v", endIST, end)
	}
	if got := end.In(ist).Format("15:04"); got != "16:05" {
		t.Errorf("end renders as %s IST, want 16:05", got)
	}
}

// --- CPU series source: Grafana's recording rule first, raw rate as the fallback --------------

func TestCPUUsageSeries_ReadsGrafanasRecordingRuleFirst(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "web-abc"}, Value: 1}}, nil
		},
		rangeFn: func(string) (promcommon.Matrix, error) {
			return promcommon.Matrix{{Values: []promcommon.SamplePair{{Timestamp: 1000, Value: 0.5}}}}, nil
		},
	}
	b := &Builder{Prom: prom}
	wl := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "web"}
	pts, _, _, err := b.CPUUsageSeries(context.Background(), wl, "c", time.Hour, time.Now())
	if err != nil || len(pts) != 1 {
		t.Fatalf("pts=%v err=%v", pts, err)
	}
	var rangeQueries []string
	for _, q := range prom.Queries {
		if strings.Contains(q, "container_cpu_usage_seconds_total") {
			rangeQueries = append(rangeQueries, q)
		}
	}
	if len(rangeQueries) != 1 || !strings.Contains(rangeQueries[0], cpuUsageRule) {
		t.Errorf("want exactly one range query, against the recording rule; got %v", rangeQueries)
	}
}

func TestCPUUsageSeries_FallsBackToRawRateWhenTheRuleHasNoData(t *testing.T) {
	prom := &recordingPromClient{
		instant: func(string) (promcommon.Vector, error) {
			return promcommon.Vector{{Metric: promcommon.Metric{"pod": "web-abc"}, Value: 1}}, nil
		},
		rangeFn: func(q string) (promcommon.Matrix, error) {
			if strings.Contains(q, cpuUsageRule) {
				return promcommon.Matrix{}, nil // rule not installed / no series
			}
			return promcommon.Matrix{{Values: []promcommon.SamplePair{{Timestamp: 1000, Value: 0.25}}}}, nil
		},
	}
	b := &Builder{Prom: prom}
	wl := analyzermodel.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "web"}
	pts, _, _, err := b.CPUUsageSeries(context.Background(), wl, "c", time.Hour, time.Now())
	if err != nil || len(pts) != 1 || pts[0].Value != 0.25 {
		t.Fatalf("want the fallback's point, got pts=%v err=%v", pts, err)
	}
	var raw string
	for _, q := range prom.Queries {
		if strings.Contains(q, "rate(container_cpu_usage_seconds_total") {
			raw = q
		}
	}
	if !strings.Contains(raw, "[5m]") || !strings.Contains(raw, `image!=""`) {
		t.Errorf("fallback must be the 5m cadvisor rate with image!=\"\"; got %q", raw)
	}
}
