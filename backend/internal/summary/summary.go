// Package summary turns a model.Finding into a plain-language Summary using fixed per-rule
// templates. It is a pure function of the finding: every number it prints is read from the
// finding's evidence, and where a cause cannot be known from Prometheus data alone it says so
// instead of guessing (AGENTS.md: findings are deterministic; missing data is a caveat).
package summary

import (
	"fmt"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// For returns the summary for f. A rule without a template gets a generic summary built from the
// finding's own problem and threshold text, so a newly added rule is never shown with no summary.
func For(f model.Finding) model.Summary {
	var s model.Summary
	switch f.RuleID {
	case "R001":
		s = cpuOverRequested(f)
	case "R002":
		s = memoryOverRequested(f)
	case "R003":
		s = frequentRestarts(f)
	case "R004":
		s = oomKilled(f)
	case "R006":
		s = unschedulable(f)
	default:
		s = model.Summary{
			WhatHappened: f.Problem + ".",
			WhyItMatters: "See the threshold and evidence below for how this was detected.",
			NextSteps:    []string{"Review the evidence table and run each query to confirm the values."},
		}
	}
	s.Notes = append(s.Notes, notes(f)...)
	// JSON must carry [] not null for empty lists, so clients can iterate without a nil check.
	if s.Notes == nil {
		s.Notes = []string{}
	}
	if s.NextSteps == nil {
		s.NextSteps = []string{}
	}
	return s
}

func target(f model.Finding) string {
	t := fmt.Sprintf("%s %s/%s", f.Workload.Kind, f.Workload.Namespace, f.Workload.Name)
	if f.Container != "" {
		t += fmt.Sprintf(" (container %s)", f.Container)
	}
	return t
}

func notes(f model.Finding) []string {
	var out []string
	if f.Confidence != "" {
		out = append(out, fmt.Sprintf("Confidence is %s: %s.", f.Confidence, f.ConfidenceReason))
	}
	if f.DataQuality.Status != model.DataQualityOK {
		out = append(out, fmt.Sprintf("Data quality is %s, so treat the numbers with care.", f.DataQuality.Status))
	}
	for _, c := range f.Caveats {
		if c == "hpa-coupled" {
			out = append(out, "A HorizontalPodAutoscaler scales this workload, so changing its request also changes when it scales.")
		}
	}
	return out
}

func value(f model.Finding, metric string) (float64, bool) {
	for _, e := range f.Evidence {
		if e.Metric == metric {
			return e.Value, true
		}
	}
	return 0, false
}

func formatCores(c float64) string {
	if c >= 1 {
		return fmt.Sprintf("%.2f cores", c)
	}
	return fmt.Sprintf("%.0fm", c*1000)
}

func formatBytes(b float64) string {
	const mi = 1024 * 1024
	const gi = 1024 * mi
	if b >= gi {
		return fmt.Sprintf("%.2fGi", b/gi)
	}
	return fmt.Sprintf("%.0fMi", b/mi)
}

func costLine(f model.Finding) []string {
	if f.Cost == nil || f.Cost.PotentialDifferenceUSD <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("Estimated potential difference: about $%.2f per month at the configured example prices. This is an estimate of allocation cost, not realised savings.", f.Cost.PotentialDifferenceUSD)}
}

func cpuOverRequested(f model.Finding) model.Summary {
	req, _ := value(f, "cpu_request")
	p95, _ := value(f, "cpu_usage_p95")
	if neutral, ok := value(f, "hpa_neutral_request"); ok {
		hpaTarget, _ := value(f, "hpa_target_cpu_utilization_percent")
		return model.Summary{
			WhatHappened: fmt.Sprintf("%s requests %s of CPU, its p95 usage is %s, and an HPA scales it at a %.0f%% CPU target.", target(f), formatCores(req), formatCores(p95), hpaTarget),
			WhyItMatters: "With an HPA, a smaller request makes the same usage look like a higher utilisation, so the HPA adds replicas. Resizing the request alone can raise total allocation instead of lowering it.",
			NextSteps: []string{
				fmt.Sprintf("Do not resize the request on its own. If you change it, change the HPA target together; the request that keeps the HPA's behaviour unchanged is about %s.", formatCores(neutral)),
				"Check the HPA's min and max replicas and its recent scaling history before changing anything.",
			},
		}
	}
	suggested, _ := value(f, "cpu_suggested_request")
	return model.Summary{
		WhatHappened: fmt.Sprintf("%s requests %s of CPU, but its p95 usage over the window is %s (%.0f%% of the request).", target(f), formatCores(req), formatCores(p95), pct(p95, req)),
		WhyItMatters: "The scheduler reserves the full request on a node whether or not the container uses it. Unused requested CPU blocks other workloads from being placed and raises the allocation cost.",
		NextSteps: append([]string{
			fmt.Sprintf("Consider lowering the CPU request toward %s (p95 usage plus a 20%% buffer), after confirming the window covered your busiest period.", formatCores(suggested)),
			"Keep the request at or below the CPU limit, and check the container is not being throttled before reducing it further.",
		}, costLine(f)...),
	}
}

func memoryOverRequested(f model.Finding) model.Summary {
	req, _ := value(f, "memory_request")
	peak, _ := value(f, "memory_usage_max")
	suggested, _ := value(f, "memory_suggested_request")
	return model.Summary{
		WhatHappened: fmt.Sprintf("%s requests %s of memory, but its peak usage over the window is %s (%.0f%% of the request).", target(f), formatBytes(req), formatBytes(peak), pct(peak, req)),
		WhyItMatters: "The scheduler reserves the full memory request on a node. Memory that is requested but never used cannot be given to other workloads.",
		NextSteps: append([]string{
			fmt.Sprintf("Consider lowering the memory request toward %s (peak usage plus a 15%% buffer). Memory is sized from the peak, not the average, because one spike can get the container killed.", formatBytes(suggested)),
			"Confirm the window includes your heaviest workload (batch jobs, traffic peaks) before reducing it.",
		}, costLine(f)...),
		Notes: []string{"This rule does not fire while the container has a recent out-of-memory kill, so no recent OOM was seen for it."},
	}
}

func frequentRestarts(f model.Finding) model.Summary {
	h1, _ := value(f, "restarts_1h")
	h24, _ := value(f, "restarts_24h")
	what := fmt.Sprintf("%s restarted %.0f times in the last hour and %.0f times in the last 24 hours.", target(f), h1, h24)
	for _, e := range f.Evidence {
		if e.Metric == "waiting_reason" && e.Unit != "" {
			what += fmt.Sprintf(" Its current waiting reason is %s.", e.Unit)
		}
	}
	return model.Summary{
		WhatHappened: what,
		WhyItMatters: "Each restart interrupts the container and drops in-flight work. A steady restart rate usually means the container is crashing or being killed by a failing health probe, and Kubernetes slows restarts down with a back-off delay.",
		NextSteps: []string{
			"Run kubectl describe on one of the pods and read its Events and the last state of the container.",
			"Run kubectl logs --previous on the pod to see the output just before the last restart.",
			"Check whether a recent deploy or config change lines up with when the restarts started.",
			"Review the liveness probe: a probe that is too strict kills a healthy container.",
		},
		Notes: []string{"This rule counts restarts only. It does not know why the container restarted, so the cause above is something to check, not a finding."},
	}
}

func oomKilled(f model.Finding) model.Summary {
	h24, _ := value(f, "restarts_24h")
	what := fmt.Sprintf("%s was terminated for running out of memory (OOMKilled) and has restarted %.0f times in the last 24 hours.", target(f), h24)
	steps := []string{
		"Compare the container's real peak memory with its limit. If the peak is near the limit, raise the memory limit and request together.",
		"If memory climbs steadily until the kill, look for a leak or an unbounded cache or queue in the application.",
		"Do not lower the memory request or limit on this workload until the kills stop.",
	}
	if limit, ok := value(f, "memory_limit"); ok {
		what += fmt.Sprintf(" Its memory limit is %s.", formatBytes(limit))
	} else {
		steps = append([]string{"No memory limit was found in the metrics for this container; check the pod spec to see what limit applies."}, steps...)
	}
	return model.Summary{
		WhatHappened: what,
		WhyItMatters: "An OOM kill means the container used more memory than its limit allows and the kernel stopped it. Work in progress is lost, and repeated kills lead to a crash loop where the workload is mostly unavailable.",
		NextSteps:    steps,
	}
}

func unschedulable(f model.Finding) model.Summary {
	var asks string
	if c, ok := value(f, "cpu_request"); ok {
		asks = fmt.Sprintf(" It requests %s of CPU", formatCores(c))
		if m, ok := value(f, "memory_request"); ok {
			asks += fmt.Sprintf(" and %s of memory", formatBytes(m))
		}
		asks += "."
	} else if m, ok := value(f, "memory_request"); ok {
		asks = fmt.Sprintf(" It requests %s of memory.", formatBytes(m))
	}
	return model.Summary{
		WhatHappened: fmt.Sprintf("A pod of %s is Pending and the scheduler reports it as unschedulable, so the workload is not running.%s", target(f), asks),
		WhyItMatters: "No pod is serving traffic or doing work for this workload until it is placed on a node. The scheduler retries on its own, but it will not succeed until the blocking reason is removed.",
		NextSteps: []string{
			"Run kubectl describe on the Pending pod. The FailedScheduling event names the exact reason, such as insufficient CPU or memory, a taint, or a node selector that matches no node.",
			"Compare the requests above with the allocatable capacity of your nodes (kubectl describe nodes). A request larger than any single node can never be scheduled.",
			"Check for node selectors, affinity rules, taints without a matching toleration, and unbound PersistentVolumeClaims.",
		},
		Notes: []string{"The analyzer has no access to Kubernetes Events, so it cannot tell you which of these is the cause. The steps above are where to look."},
	}
}

func targetName(f model.Finding) string { return target(f) }

func pct(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return part / whole * 100
}
