package evidence

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/common/model"

	analyzermodel "github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/promclient"
)

// evalWindow is the widest history any rule might ask for; Builder fetches once at this width
// and Classify (confidence.go) re-derives shorter tiers from the same fetch (see
// evidence.MatrixSource / computeStatsWindow).
const evalWindow = highWindow // 7 days

// step is the range-query output resolution. It is a query-resolution choice, not an assumption
// about the real scrape interval (see stats.go's coverage design note) -- 2m keeps a 7-day fetch
// to a few thousand points per series without losing the density needed for percentile/coverage
// math.
const step = 2 * time.Minute

// Builder queries Prometheus to build WorkloadEvidence. It never touches the Kubernetes API.
type Builder struct {
	Prom promclient.Client
}

// ResolveContainers finds every container name currently reporting resource requests/limits for
// a workload's live pods, via kube_pod_container_resource_requests -- the Prometheus-only
// substitute for reading .spec.containers[].name off the Pods API directly (D-007 grants no
// `pods` access). Returns an empty, error-free slice for a workload with no live pods (e.g.
// Pending): that is a legitimate state, not a failure.
func (b *Builder) ResolveContainers(ctx context.Context, workload analyzermodel.WorkloadRef) ([]string, error) {
	podNames, err := b.resolvePodNames(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("resolving pods: %w", err)
	}
	if len(podNames) == 0 {
		return nil, nil
	}
	query := fmt.Sprintf(`count by (container) (kube_pod_container_resource_requests{namespace=%q,pod=~%q})`,
		workload.Namespace, podRegex(podNames))
	vec, err := b.Prom.QueryInstant(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("resolving containers: %w", err)
	}
	containers := make([]string, 0, len(vec))
	for _, sample := range vec {
		if c, ok := sample.Metric["container"]; ok {
			containers = append(containers, string(c))
		}
	}
	return containers, nil
}

// maxSeriesWindow bounds GET /api/timeseries requests: the dashboard's chart never needs more
// than a few days of resolution to be useful, and this keeps a client-facing endpoint from being
// able to trigger an arbitrarily expensive 7-day-at-fine-resolution query on demand.
const maxSeriesWindow = 48 * time.Hour

// CPUUsageSeries returns CPU usage points (max across live pods at each timestamp) for the
// dashboard's usage-over-time chart, along with the current request/limit for the reference
// lines drawn alongside it. window is capped at maxSeriesWindow.
func (b *Builder) CPUUsageSeries(ctx context.Context, workload analyzermodel.WorkloadRef, container string, window time.Duration, now time.Time) ([]Point, float64, float64, error) {
	if window > maxSeriesWindow || window <= 0 {
		window = maxSeriesWindow
	}
	podNames, err := b.resolvePodNames(ctx, workload)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("resolving pods: %w", err)
	}
	if len(podNames) == 0 {
		return nil, 0, 0, nil
	}
	ns := workload.Namespace
	podSelector := podRegex(podNames)

	query := fmt.Sprintf(`max(rate(container_cpu_usage_seconds_total{namespace=%q,pod=~%q,container=%q}[2m])) by (pod)`, ns, podSelector, container)
	matrix, err := b.Prom.QueryRange(ctx, query, now.Add(-window), now, seriesStep(window))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cpu usage series: %w", err)
	}
	req, _ := firstValue(orEmptyVec(b.Prom.QueryInstant(ctx, fmt.Sprintf(`kube_pod_container_resource_requests{namespace=%q,pod=~%q,container=%q,resource="cpu"}`, ns, podSelector, container))))
	limit, _ := firstValue(orEmptyVec(b.Prom.QueryInstant(ctx, fmt.Sprintf(`kube_pod_container_resource_limits{namespace=%q,pod=~%q,container=%q,resource="cpu"}`, ns, podSelector, container))))
	return matrixToPoints(matrix), req, limit, nil
}

// MemoryUsageSeries is CPUUsageSeries' memory counterpart.
func (b *Builder) MemoryUsageSeries(ctx context.Context, workload analyzermodel.WorkloadRef, container string, window time.Duration, now time.Time) ([]Point, float64, float64, error) {
	if window > maxSeriesWindow || window <= 0 {
		window = maxSeriesWindow
	}
	podNames, err := b.resolvePodNames(ctx, workload)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("resolving pods: %w", err)
	}
	if len(podNames) == 0 {
		return nil, 0, 0, nil
	}
	ns := workload.Namespace
	podSelector := podRegex(podNames)

	query := fmt.Sprintf(`max(container_memory_working_set_bytes{namespace=%q,pod=~%q,container=%q}) by (pod)`, ns, podSelector, container)
	matrix, err := b.Prom.QueryRange(ctx, query, now.Add(-window), now, seriesStep(window))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("memory usage series: %w", err)
	}
	req, _ := firstValue(orEmptyVec(b.Prom.QueryInstant(ctx, fmt.Sprintf(`kube_pod_container_resource_requests{namespace=%q,pod=~%q,container=%q,resource="memory"}`, ns, podSelector, container))))
	limit, _ := firstValue(orEmptyVec(b.Prom.QueryInstant(ctx, fmt.Sprintf(`kube_pod_container_resource_limits{namespace=%q,pod=~%q,container=%q,resource="memory"}`, ns, podSelector, container))))
	return matrixToPoints(matrix), req, limit, nil
}

// seriesStep picks a chart-friendly resolution: fine enough to look smooth, coarse enough that a
// 48h request stays a few hundred points, not thousands.
func seriesStep(window time.Duration) time.Duration {
	if window <= 3*time.Hour {
		return 30 * time.Second
	}
	if window <= 12*time.Hour {
		return 2 * time.Minute
	}
	return 5 * time.Minute
}

// matrixToPoints flattens a (possibly multi-pod) matrix into one chronological point list, one
// point per distinct timestamp, taking the max across pods at that timestamp -- consistent with
// how evidence.computeStats already treats multi-pod data (see stats.go).
func matrixToPoints(matrix model.Matrix) []Point {
	byTime := map[int64]float64{}
	for _, series := range matrix {
		for _, sample := range series.Values {
			t := sample.Timestamp.Time().Unix()
			v := float64(sample.Value)
			if math.IsNaN(v) {
				continue
			}
			if cur, ok := byTime[t]; !ok || v > cur {
				byTime[t] = v
			}
		}
	}
	points := make([]Point, 0, len(byTime))
	for t, v := range byTime {
		points = append(points, Point{UnixSeconds: t, Value: v})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].UnixSeconds < points[j].UnixSeconds })
	return points
}

// orEmptyVec swallows a QueryInstant error and returns an empty vector: request/limit are
// secondary context for a chart, not worth failing the whole endpoint over if only they fail.
func orEmptyVec(vec model.Vector, err error) model.Vector {
	if err != nil {
		return nil
	}
	return vec
}

// Build gathers all evidence for one container of one workload.
func (b *Builder) Build(ctx context.Context, workload analyzermodel.WorkloadRef, container string, now time.Time) WorkloadEvidence {
	ev := WorkloadEvidence{Workload: workload, Container: container, AsOf: now}

	podNames, err := b.resolvePodNames(ctx, workload)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("resolving pods: %v", err))
		return ev
	}
	ev.PodNames = podNames
	if len(podNames) == 0 {
		// A workload can legitimately have zero live pods (Pending, scaled to 0). That is not
		// itself a query error -- rules that need usage data will see HasData=false via
		// Classify and report `insufficient`, which is the correct, honest outcome.
		return ev
	}
	podSelector := podRegex(podNames)

	b.buildCPU(ctx, &ev, podSelector, now)
	b.buildMemory(ctx, &ev, podSelector, now)
	b.buildRestarts(ctx, &ev, podSelector, now)
	b.buildTerminationReasons(ctx, &ev, podSelector, now)
	b.buildHPA(ctx, &ev, now)

	return ev
}

func (b *Builder) buildCPU(ctx context.Context, ev *WorkloadEvidence, podSelector string, now time.Time) {
	ns, c := ev.Workload.Namespace, ev.Container

	// rate() lookback is 2m, not the more common 5m: a 5m lookback was tried first and checked
	// live against the cpu-spike demo fixture (30s busy / 270s idle) -- with a 5m window, every
	// evaluated point's rate() already averages a full burst cycle internally, so the returned
	// series was smoothed to a near-constant ~10% duty average with p99/p50 close to 1.0,
	// silently defeating the burstiness guard below (it never fired, because the smoothing
	// happened before this code ever saw the data). 2m is short enough to leave idle-only
	// evaluation points near zero and burst-overlapping points visibly elevated, restoring the
	// p50/p99 spread the guard depends on, while still safely above this cluster's measured
	// scrape cadence (Phase 1: effectively >=60s between real samples for some series).
	usageQuery := fmt.Sprintf(
		`max(rate(container_cpu_usage_seconds_total{namespace=%q,pod=~%q,container=%q}[2m])) by (pod)`,
		ns, podSelector, c)
	matrix, err := b.Prom.QueryRange(ctx, usageQuery, now.Add(-evalWindow), now, step)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("cpu usage: %v", err))
	} else {
		src := MatrixSource{Matrix: matrix}
		ev.CPU.DataQuality, ev.CPU.Confidence, ev.CPU.ConfidenceReason = Classify(src, now)
		ev.CPU.Usage = src.Window(now, evalWindow)
		if ev.CPU.DataQuality.Status == analyzermodel.DataQualityOK && IsStale(ev.CPU.Usage, now) {
			ev.CPU.DataQuality.Status = analyzermodel.DataQualityStale
			ev.CPU.Confidence = ""
		}
	}

	requestVec, err := b.Prom.QueryInstant(ctx, fmt.Sprintf(
		`kube_pod_container_resource_requests{namespace=%q,pod=~%q,container=%q,resource="cpu"}`,
		ns, podSelector, c))
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("cpu request: %v", err))
	} else if v, ok := firstValue(requestVec); ok {
		ev.CPU.RequestCores, ev.CPU.HasRequest = v, true
	}

	limitVec, err := b.Prom.QueryInstant(ctx, fmt.Sprintf(
		`kube_pod_container_resource_limits{namespace=%q,pod=~%q,container=%q,resource="cpu"}`,
		ns, podSelector, c))
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("cpu limit: %v", err))
	} else if v, ok := firstValue(limitVec); ok {
		ev.CPU.LimitCores, ev.CPU.HasLimit = v, true
	}

	// Throttle ratio over a recent 30m window -- only meaningful (and only present at all) for
	// containers with a CPU limit set (verified Phase 1 Lab 06: cAdvisor emits no throttle
	// series otherwise). A query that returns no series here is expected, not an error.
	throttleQuery := fmt.Sprintf(
		`sum(rate(container_cpu_cfs_throttled_periods_total{namespace=%q,pod=~%q,container=%q}[30m])) / sum(rate(container_cpu_cfs_periods_total{namespace=%q,pod=~%q,container=%q}[30m]))`,
		ns, podSelector, c, ns, podSelector, c)
	throttleVec, err := b.Prom.QueryInstant(ctx, throttleQuery)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("cpu throttle ratio: %v", err))
	} else if v, ok := firstValue(throttleVec); ok {
		ev.CPU.ThrottledRatio, ev.CPU.HasThrottlingData = v, true
	}
}

func (b *Builder) buildMemory(ctx context.Context, ev *WorkloadEvidence, podSelector string, now time.Time) {
	ns, c := ev.Workload.Namespace, ev.Container

	usageQuery := fmt.Sprintf(
		`container_memory_working_set_bytes{namespace=%q,pod=~%q,container=%q}`,
		ns, podSelector, c)
	matrix, err := b.Prom.QueryRange(ctx, usageQuery, now.Add(-evalWindow), now, step)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("memory usage: %v", err))
	} else {
		src := MatrixSource{Matrix: matrix}
		ev.Mem.DataQuality, ev.Mem.Confidence, ev.Mem.ConfidenceReason = Classify(src, now)
		ev.Mem.Usage = src.Window(now, evalWindow)
		if ev.Mem.DataQuality.Status == analyzermodel.DataQualityOK && IsStale(ev.Mem.Usage, now) {
			ev.Mem.DataQuality.Status = analyzermodel.DataQualityStale
			ev.Mem.Confidence = ""
		}
	}

	requestVec, err := b.Prom.QueryInstant(ctx, fmt.Sprintf(
		`kube_pod_container_resource_requests{namespace=%q,pod=~%q,container=%q,resource="memory"}`,
		ns, podSelector, c))
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("memory request: %v", err))
	} else if v, ok := firstValue(requestVec); ok {
		ev.Mem.RequestBytes, ev.Mem.HasRequest = v, true
	}

	limitVec, err := b.Prom.QueryInstant(ctx, fmt.Sprintf(
		`kube_pod_container_resource_limits{namespace=%q,pod=~%q,container=%q,resource="memory"}`,
		ns, podSelector, c))
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("memory limit: %v", err))
	} else if v, ok := firstValue(limitVec); ok {
		ev.Mem.LimitBytes, ev.Mem.HasLimit = v, true
	}
}

func (b *Builder) buildRestarts(ctx context.Context, ev *WorkloadEvidence, podSelector string, now time.Time) {
	ns, c := ev.Workload.Namespace, ev.Container

	q1h := fmt.Sprintf(`sum(increase(kube_pod_container_status_restarts_total{namespace=%q,pod=~%q,container=%q}[1h]))`, ns, podSelector, c)
	if vec, err := b.Prom.QueryInstant(ctx, q1h); err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("restarts 1h: %v", err))
	} else if v, ok := firstValue(vec); ok {
		ev.RestartsIncrease1h, ev.HasRestartsData = v, true
	}

	q24h := fmt.Sprintf(`sum(increase(kube_pod_container_status_restarts_total{namespace=%q,pod=~%q,container=%q}[24h]))`, ns, podSelector, c)
	if vec, err := b.Prom.QueryInstant(ctx, q24h); err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("restarts 24h: %v", err))
	} else if v, ok := firstValue(vec); ok {
		ev.RestartsIncrease24h = v
		ev.HasRestartsData = true
	}
}

func (b *Builder) buildTerminationReasons(ctx context.Context, ev *WorkloadEvidence, podSelector string, now time.Time) {
	ns, c := ev.Workload.Namespace, ev.Container

	// last_terminated_reason is a 1/0 series per possible reason value; the reason label on the
	// series whose value is 1 is the one that actually applies (verified Phase 1 Lab 06 and
	// Phase 3 live: "oom" -> reason=OOMKilled, "crashloop" -> reason=Error, both == 1).
	termQuery := fmt.Sprintf(`kube_pod_container_status_last_terminated_reason{namespace=%q,pod=~%q,container=%q} == 1`, ns, podSelector, c)
	if vec, err := b.Prom.QueryInstant(ctx, termQuery); err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("last terminated reason: %v", err))
	} else if reason, ok := firstLabel(vec, "reason"); ok {
		ev.LastTerminatedReason = reason
	}

	// max_over_time over a short window: an instant query can fall between restarts and miss a
	// CrashLoopBackOff that is genuinely happening (Phase 1 Lab 06 finding).
	waitQuery := fmt.Sprintf(`max_over_time(kube_pod_container_status_waiting_reason{namespace=%q,pod=~%q,container=%q}[10m]) == 1`, ns, podSelector, c)
	if vec, err := b.Prom.QueryInstant(ctx, waitQuery); err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("waiting reason: %v", err))
	} else if reason, ok := firstLabel(vec, "reason"); ok {
		ev.WaitingReason = reason
	}
}

func (b *Builder) buildHPA(ctx context.Context, ev *WorkloadEvidence, now time.Time) {
	ns, kind, name := ev.Workload.Namespace, ev.Workload.Kind, ev.Workload.Name

	infoQuery := fmt.Sprintf(
		`kube_horizontalpodautoscaler_info{namespace=%q,scaletargetref_kind=%q,scaletargetref_name=%q}`,
		ns, kind, name)
	infoVec, err := b.Prom.QueryInstant(ctx, infoQuery)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("hpa info: %v", err))
		return
	}
	hpaName, ok := firstLabel(infoVec, "horizontalpodautoscaler")
	if !ok {
		return // no HPA targets this workload -- the normal case, not an error
	}
	ev.HPA.Present = true

	targetQuery := fmt.Sprintf(`kube_horizontalpodautoscaler_spec_target_metric{namespace=%q,horizontalpodautoscaler=%q}`, ns, hpaName)
	targetVec, err := b.Prom.QueryInstant(ctx, targetQuery)
	if err != nil {
		ev.QueryErrors = append(ev.QueryErrors, fmt.Sprintf("hpa target metric: %v", err))
		return
	}
	for _, sample := range targetVec {
		metricName := string(sample.Metric["metric_name"])
		targetType := string(sample.Metric["metric_target_type"])
		value := float64(sample.Value)
		if targetType != "utilization" {
			continue // Value/AverageValue-type targets are not coupled to the request the way Utilization is
		}
		switch metricName {
		case "cpu":
			ev.HPA.TargetsCPUUtilization = true
			ev.HPA.CPUTargetUtilizationPercent = value
		case "memory":
			ev.HPA.TargetsMemoryUtilization = true
			ev.HPA.MemoryTargetUtilizationPercent = value
		}
	}
}

// resolvePodNames finds the live pods currently owned by workload, via kube_pod_owner joined to
// kube_replicaset_owner on the ReplicaSet name -- the Prometheus-only substitute for listing a
// Deployment's pods directly (verified live against the real cluster before this was written;
// see docs/kubernetes-fundamentals.md and the Phase 4 commit for the exact query and its output).
func (b *Builder) resolvePodNames(ctx context.Context, workload analyzermodel.WorkloadRef) ([]string, error) {
	query := fmt.Sprintf(`label_replace(kube_pod_owner{namespace=%q, owner_kind="ReplicaSet"}, "rsname", "$1", "owner_name", "(.*)")
  * on(namespace, rsname) group_left()
label_replace(kube_replicaset_owner{namespace=%q, owner_kind=%q, owner_name=%q}, "rsname", "$1", "replicaset", "(.*)")`,
		workload.Namespace, workload.Namespace, workload.Kind, workload.Name)

	vec, err := b.Prom.QueryInstant(ctx, query)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(vec))
	for _, sample := range vec {
		if pod, ok := sample.Metric["pod"]; ok {
			names = append(names, string(pod))
		}
	}
	return names, nil
}

var podNameSafe = regexp.MustCompile(`^[a-z0-9.-]+$`)

// podRegex builds a pod=~"..." alternation from exact, already-resolved pod names. Kubernetes
// pod names are DNS-1123 subdomains (lowercase alphanumeric, '.', '-'), which contain no PromQL
// regex metacharacters -- podNameSafe asserts that rather than assuming it, so a name that
// somehow doesn't match is excluded rather than silently corrupting the query.
func podRegex(names []string) string {
	safe := make([]string, 0, len(names))
	for _, n := range names {
		if podNameSafe.MatchString(n) {
			safe = append(safe, regexp.QuoteMeta(n))
		}
	}
	return "^(" + strings.Join(safe, "|") + ")$"
}

func firstValue(vec model.Vector) (float64, bool) {
	if len(vec) == 0 {
		return 0, false
	}
	return float64(vec[0].Value), true
}

func firstLabel(vec model.Vector, label model.LabelName) (string, bool) {
	if len(vec) == 0 {
		return "", false
	}
	v, ok := vec[0].Metric[label]
	return string(v), ok
}
