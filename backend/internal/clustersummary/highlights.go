// Package clustersummary builds the cluster-wide capacity and health snapshot shown on the
// dashboard's Cluster page. Highlights is a pure function of the numbers; Provider does the
// Prometheus queries behind an interface.
package clustersummary

import (
	"fmt"
	"math"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// Thresholds, named so each number in a highlight is visibly one decision.
const (
	requestPressureRatio = 0.80 // requested / allocatable at or above this: little scheduling headroom
	underUseRatio        = 0.50 // usage / requested below this: requests are far above real use
	minRequestedCPU      = 0.1  // cores; below this the ratio is noise
	minRequestedMemBytes = 256 * 1024 * 1024
)

// Highlights turns a summary into short, numeric statements. It only reads fields that are
// present: a nil metric produces no highlight, never a guess. An empty result means nothing
// crossed a threshold, which the page states explicitly.
func Highlights(s model.ClusterSummary) []string {
	var out []string

	if s.Nodes != nil && s.NodesReady != nil && *s.NodesReady < *s.Nodes {
		out = append(out, fmt.Sprintf("%s not Ready (%.0f of %.0f).", count(*s.Nodes-*s.NodesReady, "node is", "nodes are"), *s.Nodes-*s.NodesReady, *s.Nodes))
	}
	if s.PodsPending != nil && *s.PodsPending > 0 {
		out = append(out, fmt.Sprintf("%s Pending, so not running yet.", count(*s.PodsPending, "pod is", "pods are")))
	}
	if s.PodsFailed != nil && *s.PodsFailed > 0 {
		out = append(out, fmt.Sprintf("%s in the Failed phase.", count(*s.PodsFailed, "pod is", "pods are")))
	}

	out = append(out, resourceHighlights("CPU", s.CPUAllocatableCores, s.CPURequestedCores, s.CPUUsageCores, minRequestedCPU, cores)...)
	out = append(out, resourceHighlights("Memory", s.MemAllocatableBytes, s.MemRequestedBytes, s.MemUsageBytes, minRequestedMemBytes, bytesFmt)...)

	if s.Restarts24h != nil && *s.Restarts24h >= 1 {
		out = append(out, fmt.Sprintf("Containers restarted %s in the last 24 hours across the cluster.", count(*s.Restarts24h, "time", "times")))
	}
	return out
}

func resourceHighlights(name string, alloc, req, use *float64, minReq float64, f func(float64) string) []string {
	var out []string
	if alloc != nil && req != nil && *alloc > 0 && *req / *alloc >= requestPressureRatio {
		out = append(out, fmt.Sprintf("%s requests use %.0f%% of allocatable (%s of %s), which leaves little room to schedule more pods (threshold %.0f%%).",
			name, *req / *alloc * 100, f(*req), f(*alloc), requestPressureRatio*100))
	}
	if req != nil && use != nil && *req >= minReq && *use / *req < underUseRatio {
		out = append(out, fmt.Sprintf("Pods use %.0f%% of the %s they request (%s used of %s requested); requests are well above real use (threshold %.0f%%).",
			*use / *req * 100, name, f(*use), f(*req), underUseRatio*100))
	}
	return out
}

// count renders "1 pod is" / "4 pods are": n rounded to a whole number, then the singular or
// plural phrase.
func count(n float64, singular, plural string) string {
	r := math.Round(n)
	if r == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%.0f %s", r, plural)
}

func cores(v float64) string {
	if v >= 1 {
		return fmt.Sprintf("%.2f cores", v)
	}
	return fmt.Sprintf("%.0fm", v*1000)
}

func bytesFmt(v float64) string {
	const mi, gi = 1024 * 1024, 1024 * 1024 * 1024
	if v >= gi {
		return fmt.Sprintf("%.2fGi", v/gi)
	}
	return fmt.Sprintf("%.0fMi", v/mi)
}
