package evidence

import (
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// WorkloadEvidence is everything the rule engine (internal/rules) needs to evaluate one
// container of one workload. It is built entirely from Prometheus/kube-state-metrics data
// (Builder never touches the Kubernetes Pods API -- D-007 grants no `pods` access, and pod specs
// can carry secrets in plain env vars; see docs/decisions/ADR-007-k8s-api-scope.md). Pod names
// are resolved via the kube_pod_owner/kube_replicaset_owner join in resolvePodNames, which is
// the Prometheus-only substitute for listing a Deployment's live pods.
type WorkloadEvidence struct {
	Workload  model.WorkloadRef
	Container string
	AsOf      time.Time

	// PodNames is the live pod set resolved for this workload at AsOf. Empty means either the
	// workload has zero ready pods right now (e.g. Pending) or the owner-chain join itself
	// found nothing -- callers distinguish these via QueryErrors.
	PodNames []string

	CPU CPUEvidence
	Mem MemEvidence

	RestartsIncrease1h  float64
	RestartsIncrease24h float64
	HasRestartsData     bool

	LastTerminatedReason string // "" if none observed, e.g. "OOMKilled" | "Error" | "Completed"
	WaitingReason        string // "" if none observed, e.g. "CrashLoopBackOff"

	HPA HPAEvidence

	// QueryErrors records which sub-queries failed for this workload, without discarding
	// whatever evidence *did* come back successfully (AGENTS.md: "unavailable source ...
	// degraded result, logged, counted").
	QueryErrors []string
}

// CPUEvidence holds request/limit/usage facts for CPU. Kept separate from MemEvidence because
// the two use deliberately different logic (docs/requirements.md §3): too little memory kills
// the pod, too little CPU only throttles it.
type CPUEvidence struct {
	RequestCores float64
	HasRequest   bool
	LimitCores   float64
	HasLimit     bool

	Usage            SeriesStats
	DataQuality      model.DataQuality
	Confidence       model.Confidence
	ConfidenceReason string

	ThrottledRatio    float64 // 0..1 over the last 30m; only meaningful when HasThrottlingData
	HasThrottlingData bool    // false when no CPU limit is set -- cAdvisor emits no throttle series then (verified Phase 1 Lab 06)
}

// MemEvidence holds request/limit/usage facts for memory.
type MemEvidence struct {
	RequestBytes float64
	HasRequest   bool
	LimitBytes   float64
	HasLimit     bool

	Usage            SeriesStats // Max is the figure rules use, not P95 (a single memory spike can OOM; CPU spikes only throttle)
	DataQuality      model.DataQuality
	Confidence       model.Confidence
	ConfidenceReason string
}

// HPAEvidence records whether a HorizontalPodAutoscaler targets this workload, and on which
// resource(s) -- the single most important fact for R001/R002 to avoid recommending a plain
// resize on an HPA-coupled workload (Phase 1 Lab 05: doing so inflated total allocation).
type HPAEvidence struct {
	Present                        bool
	TargetsCPUUtilization          bool
	CPUTargetUtilizationPercent    float64
	TargetsMemoryUtilization       bool
	MemoryTargetUtilizationPercent float64
}
