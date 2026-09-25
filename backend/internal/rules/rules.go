// Package rules implements R001-R005 (docs/requirements.md §3) as pure functions over
// evidence.WorkloadEvidence. No package here calls Prometheus or Kubernetes directly (AGENTS.md:
// "Rule and cost logic are pure functions with unit tests"); internal/evidence gathers the
// input, this package only reasons about it.
//
// R005 (the data-quality gate) is not a separate function here: it is embedded in
// evidence.Classify, which every rule below checks first. A rule that finds DataQuality status
// other than "ok" returns nil rather than guessing.
package rules

import (
	"fmt"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// Thresholds, verbatim from docs/requirements.md §3. Named constants, never bare numbers in the
// logic below, so a threshold change is a one-line diff and every use of a given number is
// visibly the same decision.
const (
	overProvisionedRatio = 0.5              // usage must be below this fraction of the request
	cpuMinSlack          = 0.100            // request-usage gap must be at least 100m to bother flagging (cores)
	memMinSlackBytes     = 64 * 1024 * 1024 // 64Mi
	throttledRatioGuard  = 0.05             // >=5% throttled means "needs more CPU", never "over-provisioned"
	burstinessGuard      = 4.0              // p99/p50 above this suppresses R001 (bursty, not wasteful)
	suggestedCPUBuffer   = 1.20             // suggested request = p95 * this
	suggestedMemBuffer   = 1.15             // suggested request = max * this
	restarts1hThreshold  = 3
	restarts24hThreshold = 10
)

// R001CPUOverRequested detects a CPU request far above observed usage. Returns nil if the data
// quality gate fails, the workload is HPA-CPU-coupled (a different finding shape applies), the
// workload is too bursty to judge confidently, or usage is not actually low relative to request.
func R001CPUOverRequested(ev evidence.WorkloadEvidence, runID string, now time.Time) *model.Finding {
	if ev.CPU.DataQuality.Status != model.DataQualityOK || !ev.CPU.HasRequest {
		return nil
	}
	stats := ev.CPU.Usage

	if ev.HPA.TargetsCPUUtilization {
		return hpaCoupledFinding(ev, runID, now, stats)
	}

	if stats.Burstiness() > burstinessGuard {
		// A real spike shape (e.g. the cpu-spike demo fixture): p95/max alone would misread
		// this as waste. Suppressed rather than emitted at LOW confidence, so the dashboard
		// never shows a plain "over-provisioned" verdict for a workload that is about to burst.
		return nil
	}

	if ev.CPU.HasThrottlingData && ev.CPU.ThrottledRatio >= throttledRatioGuard {
		return nil // already CPU-starved; shrinking the request would make that worse
	}

	if stats.P95 >= overProvisionedRatio*ev.CPU.RequestCores {
		return nil // usage is not low enough relative to request
	}
	if ev.CPU.RequestCores-stats.P95 < cpuMinSlack {
		return nil // technically over the ratio but the absolute gap is too small to matter
	}

	suggested := stats.P95 * suggestedCPUBuffer
	f := newFinding(ev, "R001", runID, now, model.SeverityWarning, model.CategoryResource,
		fmt.Sprintf("CPU request (%.0fm) is far above observed usage (p95 %.0fm)", ev.CPU.RequestCores*1000, stats.P95*1000),
		fmt.Sprintf("p95 usage < %.0f%% of request, and request - p95 >= %.0fm", overProvisionedRatio*100, cpuMinSlack*1000),
		ev.CPU.DataQuality, ev.CPU.Confidence, ev.CPU.ConfidenceReason)
	f.Evidence = []model.EvidenceItem{
		{Metric: "cpu_request", Value: ev.CPU.RequestCores, Unit: "cores", Query: "kube_pod_container_resource_requests{resource=\"cpu\"}"},
		{Metric: "cpu_usage_p95", Value: stats.P95, Unit: "cores", Query: "quantile_over_time(0.95, rate(container_cpu_usage_seconds_total[5m]))"},
		{Metric: "cpu_suggested_request", Value: suggested, Unit: "cores", Query: "p95 * 1.2"},
	}
	return f
}

// hpaCoupledFinding replaces a plain resize with the HPA-neutral request (usage / target
// utilization) and an explicit caveat, per the Phase 1 Lab 05 finding that a naive resize under
// an HPA increased total allocation instead of reducing it.
func hpaCoupledFinding(ev evidence.WorkloadEvidence, runID string, now time.Time, stats evidence.SeriesStats) *model.Finding {
	if ev.HPA.CPUTargetUtilizationPercent <= 0 {
		return nil
	}
	neutral := stats.P95 / (ev.HPA.CPUTargetUtilizationPercent / 100)
	f := newFinding(ev, "R001", runID, now, model.SeverityInfo, model.CategoryResource,
		fmt.Sprintf("Workload is scaled by an HPA targeting %.0f%% CPU utilization; its request should not be resized independently of that target", ev.HPA.CPUTargetUtilizationPercent),
		"HPA present with a Utilization-type CPU target",
		ev.CPU.DataQuality, ev.CPU.Confidence, ev.CPU.ConfidenceReason)
	f.Caveats = []string{"hpa-coupled"}
	f.Evidence = []model.EvidenceItem{
		{Metric: "cpu_request", Value: ev.CPU.RequestCores, Unit: "cores", Query: "kube_pod_container_resource_requests{resource=\"cpu\"}"},
		{Metric: "cpu_usage_p95", Value: stats.P95, Unit: "cores", Query: "quantile_over_time(0.95, rate(container_cpu_usage_seconds_total[5m]))"},
		{Metric: "hpa_target_cpu_utilization_percent", Value: ev.HPA.CPUTargetUtilizationPercent, Unit: "percent", Query: "kube_horizontalpodautoscaler_spec_target_metric"},
		{Metric: "hpa_neutral_request", Value: neutral, Unit: "cores", Query: "p95 / (target_utilization / 100)"},
	}
	return f
}

// R002MemoryOverRequested detects a memory request far above observed peak usage. Memory uses
// Max, not P95, deliberately: a single memory spike can OOM-kill the container, so the rule must
// not "average away" the one sample that matters (docs/requirements.md §3).
func R002MemoryOverRequested(ev evidence.WorkloadEvidence, runID string, now time.Time) *model.Finding {
	if ev.Mem.DataQuality.Status != model.DataQualityOK || !ev.Mem.HasRequest {
		return nil
	}
	// Recently OOMed: never suggest shrinking memory right after that, regardless of how low
	// the current usage window now looks (it looks low partly *because* the OOM killed it).
	if ev.LastTerminatedReason == "OOMKilled" && ev.RestartsIncrease24h > 0 {
		return nil
	}

	stats := ev.Mem.Usage
	if stats.Max >= overProvisionedRatio*ev.Mem.RequestBytes {
		return nil
	}
	if ev.Mem.RequestBytes-stats.Max < memMinSlackBytes {
		return nil
	}

	suggested := stats.Max * suggestedMemBuffer
	f := newFinding(ev, "R002", runID, now, model.SeverityWarning, model.CategoryResource,
		fmt.Sprintf("Memory request (%.0fMi) is far above observed peak usage (%.0fMi)", ev.Mem.RequestBytes/1024/1024, stats.Max/1024/1024),
		fmt.Sprintf("max usage < %.0f%% of request, request - max >= 64Mi, no recent OOM", overProvisionedRatio*100),
		ev.Mem.DataQuality, ev.Mem.Confidence, ev.Mem.ConfidenceReason)
	f.Evidence = []model.EvidenceItem{
		{Metric: "memory_request", Value: ev.Mem.RequestBytes, Unit: "bytes", Query: "kube_pod_container_resource_requests{resource=\"memory\"}"},
		{Metric: "memory_usage_max", Value: stats.Max, Unit: "bytes", Query: "max_over_time(container_memory_working_set_bytes)"},
		{Metric: "memory_suggested_request", Value: suggested, Unit: "bytes", Query: "max * 1.15"},
	}
	return f
}

// R003FrequentRestarts fires on a restart rate that indicates instability, independent of why
// the container restarted (Phase 1 Lab 02: a liveness-probe kill reports reason=Completed,
// exit 0 -- indistinguishable from a clean exit by reason alone. Only the restart *rate* is a
// reliable signal here).
func R003FrequentRestarts(ev evidence.WorkloadEvidence, runID string, now time.Time) *model.Finding {
	if !ev.HasRestartsData {
		return nil
	}
	if ev.RestartsIncrease1h < restarts1hThreshold && ev.RestartsIncrease24h < restarts24hThreshold {
		return nil
	}

	f := newFinding(ev, "R003", runID, now, model.SeverityWarning, model.CategoryHealth,
		fmt.Sprintf("Container restarted %.0f times in the last hour, %.0f in the last 24h", ev.RestartsIncrease1h, ev.RestartsIncrease24h),
		fmt.Sprintf(">=%d restarts/1h or >=%d restarts/24h", restarts1hThreshold, restarts24hThreshold),
		model.DataQuality{Status: model.DataQualityOK, AsOf: now}, "", "restart counter present")
	f.Evidence = []model.EvidenceItem{
		{Metric: "restarts_1h", Value: ev.RestartsIncrease1h, Unit: "count", Query: "increase(kube_pod_container_status_restarts_total[1h])"},
		{Metric: "restarts_24h", Value: ev.RestartsIncrease24h, Unit: "count", Query: "increase(kube_pod_container_status_restarts_total[24h])"},
	}
	if ev.WaitingReason != "" {
		f.Evidence = append(f.Evidence, model.EvidenceItem{Metric: "waiting_reason", Value: 1, Unit: ev.WaitingReason, Query: "kube_pod_container_status_waiting_reason"})
	}
	return f
}

// R004OOM fires when the container's most recent termination was a real out-of-memory kill and
// a restart actually happened recently -- both conditions together, so a week-old OOM does not
// keep firing forever on an otherwise-healthy container (docs/requirements.md §3: "at least 1
// OOM termination in window"). Verified against a real container_oom_events_total that stayed
// at 0 through 5 real OOMs (Phase 1 Lab 06): this rule intentionally does not use that metric.
func R004OOM(ev evidence.WorkloadEvidence, runID string, now time.Time) *model.Finding {
	if ev.LastTerminatedReason != "OOMKilled" || ev.RestartsIncrease24h <= 0 {
		return nil
	}

	f := newFinding(ev, "R004", runID, now, model.SeverityCritical, model.CategoryHealth,
		"Container was OOMKilled and has restarted within the last 24h",
		"last_terminated_reason == OOMKilled and restarts increased in the last 24h",
		model.DataQuality{Status: model.DataQualityOK, AsOf: now}, "", "termination reason and restart counter both present")
	f.Evidence = []model.EvidenceItem{
		{Metric: "last_terminated_reason", Value: 1, Unit: "OOMKilled", Query: "kube_pod_container_status_last_terminated_reason"},
		{Metric: "restarts_24h", Value: ev.RestartsIncrease24h, Unit: "count", Query: "increase(kube_pod_container_status_restarts_total[24h])"},
	}
	if ev.Mem.HasLimit {
		f.Evidence = append(f.Evidence, model.EvidenceItem{Metric: "memory_limit", Value: ev.Mem.LimitBytes, Unit: "bytes", Query: "kube_pod_container_resource_limits{resource=\"memory\"}"})
	}
	return f
}

// All is every MVP rule, in the order findings should be evaluated (health-oriented facts first
// so a caller can short-circuit resource suggestions when something is actively broken, though
// nothing here currently does that -- ranking by severity happens in internal/findings).
var All = []func(evidence.WorkloadEvidence, string, time.Time) *model.Finding{
	R004OOM,
	R003FrequentRestarts,
	R001CPUOverRequested,
	R002MemoryOverRequested,
}

func newFinding(ev evidence.WorkloadEvidence, ruleID, runID string, now time.Time, sev model.Severity, cat model.Category, problem, threshold string, dq model.DataQuality, conf model.Confidence, confReason string) *model.Finding {
	// Window is left "" for counter-based health findings (R003/R004), whose DataQuality.Window
	// is the zero value -- they are not evaluated over a percentile-coverage tier, so a duration
	// string there would misleadingly imply one.
	window := ""
	if dq.Window > 0 {
		window = dq.Window.String()
	}
	return &model.Finding{
		ClusterID:        ev.Workload.ClusterID,
		RuleID:           ruleID,
		AnalysisRunID:    runID,
		GeneratedAt:      now,
		Severity:         sev,
		Category:         cat,
		Workload:         ev.Workload,
		Container:        ev.Container,
		Problem:          problem,
		Threshold:        threshold,
		Window:           window,
		DataQuality:      dq,
		Confidence:       conf,
		ConfidenceReason: confReason,
	}
}
