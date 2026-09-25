// Command verify is a manual debugging tool: it builds evidence.WorkloadEvidence for each demo/
// fixture directly against a live Prometheus (assumes `kubectl -n monitoring port-forward
// svc/kps-kube-prometheus-stack-prometheus 9090:9090` is running) and prints the raw numbers,
// without going through the rule engine. It is what Phase 4's evidence layer was checked against
// live before the rule engine was built on top of it, and stayed useful afterwards for
// diagnosing a specific workload's numbers directly (e.g. "why is R001 not firing for X").
//
// For checking the rule engine's actual output, run the real binary and query its API instead:
// `go run ./cmd/analyzer` then `curl localhost:8080/api/findings`.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/promclient"
)

func main() {
	prom, err := promclient.New("http://localhost:9090")
	if err != nil {
		panic(err)
	}
	b := &evidence.Builder{Prom: prom}
	now := time.Now()

	workloads := []string{"right-sized", "cpu-over-requested", "memory-over-requested", "cpu-spike", "oom", "crashloop", "hpa-coupled", "new-workload", "pending"}
	for _, name := range workloads {
		wl := model.WorkloadRef{ClusterID: "workload-analyzer", Namespace: "demo", Kind: "Deployment", Name: name}
		ev := b.Build(context.Background(), wl, "c", now)
		fmt.Printf("\n=== %s ===\n", name)
		fmt.Printf("pods: %v\n", ev.PodNames)
		fmt.Printf("errors: %v\n", ev.QueryErrors)
		fmt.Printf("cpu: req=%.3f(%v) limit=%.3f(%v) p50=%.4f p95=%.4f p99=%.4f burstiness=%.2f dq=%s conf=%s (%s) throttle=%.3f(%v)\n",
			ev.CPU.RequestCores, ev.CPU.HasRequest, ev.CPU.LimitCores, ev.CPU.HasLimit,
			ev.CPU.Usage.P50, ev.CPU.Usage.P95, ev.CPU.Usage.P99, ev.CPU.Usage.Burstiness(), ev.CPU.DataQuality.Status, ev.CPU.Confidence, ev.CPU.ConfidenceReason,
			ev.CPU.ThrottledRatio, ev.CPU.HasThrottlingData)
		fmt.Printf("mem: req=%.0f(%v) limit=%.0f(%v) max=%.0f dq=%s conf=%s (%s)\n",
			ev.Mem.RequestBytes, ev.Mem.HasRequest, ev.Mem.LimitBytes, ev.Mem.HasLimit,
			ev.Mem.Usage.Max, ev.Mem.DataQuality.Status, ev.Mem.Confidence, ev.Mem.ConfidenceReason)
		fmt.Printf("restarts: 1h=%.0f 24h=%.0f hasData=%v lastTerm=%q waiting=%q\n",
			ev.RestartsIncrease1h, ev.RestartsIncrease24h, ev.HasRestartsData, ev.LastTerminatedReason, ev.WaitingReason)
		fmt.Printf("hpa: present=%v cpu=%v(%.0f%%) mem=%v(%.0f%%)\n",
			ev.HPA.Present, ev.HPA.TargetsCPUUtilization, ev.HPA.CPUTargetUtilizationPercent, ev.HPA.TargetsMemoryUtilization, ev.HPA.MemoryTargetUtilizationPercent)
	}
}
