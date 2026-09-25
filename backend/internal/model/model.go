// Package model holds the analyzer's core domain types. Kept dependency-free (no Prometheus or
// Kubernetes client imports) so it can be used by every other package without a cycle, and so
// rule/cost logic (Phase 4) can depend on it without pulling in any I/O.
package model

import "time"

// ClusterID identifies the Kubernetes cluster a fact was observed on. The MVP monitors exactly
// one cluster, but every type that describes cluster-scoped state carries this field from day
// one (AGENTS.md: "Model ClusterID in core types even though v1 has one cluster"), so adding a
// second cluster later is a config change, not a schema change.
type ClusterID string

// RunStatus is the outcome of one analysis run.
type RunStatus string

const (
	RunComplete RunStatus = "complete" // every planned check succeeded
	RunPartial  RunStatus = "partial"  // some checks failed; the run still produced a snapshot
	RunFailed   RunStatus = "failed"   // the run could not produce a usable snapshot at all
)

// QueryError records one failed check during a run, without aborting the rest of it (AGENTS.md:
// "unavailable source, timeout, missing series, insufficient permissions → degraded result,
// logged, counted in a metric").
type QueryError struct {
	Source  string `json:"source"`  // "prometheus" | "kubernetes"
	Message string `json:"message"` // human-readable, never a raw credential or token
}

// AnalysisRun is the result of one pass of the analysis loop. Phase 3 populates it from
// connectivity smoke-checks only (no rules yet); Phase 4 adds Findings.
type AnalysisRun struct {
	ID             string       `json:"id"`
	ClusterID      ClusterID    `json:"cluster_id"`
	StartedAt      time.Time    `json:"started_at"`
	DurationMillis int64        `json:"duration_ms"`
	Status         RunStatus    `json:"status"`
	WorkloadsSeen  int          `json:"workloads_seen"`
	QueryErrors    []QueryError `json:"query_errors"`
}

// WorkloadRef identifies one Kubernetes workload object. Deliberately minimal (Phase 3 only
// needs enough to count and log workloads seen); Phase 4's evidence builder will carry richer
// workload facts (requests, limits, owner chain) in its own types built on top of this.
type WorkloadRef struct {
	ClusterID ClusterID `json:"cluster_id"`
	Namespace string    `json:"namespace"`
	Kind      string    `json:"kind"` // "Deployment" | "StatefulSet" | "DaemonSet"
	Name      string    `json:"name"`
}
