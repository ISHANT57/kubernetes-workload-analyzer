// Package model holds the analyzer's core domain types. Kept dependency-free (no Prometheus or
// Kubernetes client imports) so it can be used by every other package without a cycle, and so
// rule/cost logic (Phase 4) can depend on it without pulling in any I/O.
package model

import (
	"encoding/json"
	"time"
)

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
	FindingsCount  int          `json:"findings_count"`
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

// Severity is how urgently a Finding needs attention.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Category groups Findings for display ordering (docs/requirements.md: health first, then
// resource, then cost).
type Category string

const (
	CategoryHealth   Category = "health"
	CategoryResource Category = "resource"
)

// Confidence is never a bare float (docs/requirements.md §3: that would claim precision the
// rules don't have) -- it is one of three explainable tiers, always paired with a reason.
type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"   // >=7d history, >=80% coverage
	ConfidenceMedium Confidence = "MEDIUM" // >=24h history, >=80% coverage
	ConfidenceLow    Confidence = "LOW"    // less than that, but still enough to say something
)

// DataQualityStatus is the R005 gate's verdict for one metric on one workload
// (docs/requirements.md §3 "R005 Data quality gate").
type DataQualityStatus string

const (
	DataQualityOK           DataQualityStatus = "ok"
	DataQualityInsufficient DataQualityStatus = "insufficient" // history too short or too sparse
	DataQualityStale        DataQualityStatus = "stale"        // newest sample older than 5m
	DataQualityQueryError   DataQualityStatus = "query_error"  // the query itself failed
)

// DataQuality reports how trustworthy a piece of evidence is, so a caller never has to guess
// whether a missing value means "checked, found nothing" or "never checked". Coverage is a
// self-calibrated 0..1 fraction of Window that actually had data, not derived from an assumed
// scrape interval (see internal/evidence: an assumed-interval calculation was checked against
// real fixture data and found wrong).
type DataQuality struct {
	Status   DataQualityStatus `json:"status"`
	Coverage float64           `json:"coverage"` // 0..1
	Window   time.Duration     `json:"-"`        // kept as a real Duration for Go code; see MarshalJSON
	AsOf     time.Time         `json:"as_of"`    // when this was computed
}

// dataQualityJSON is DataQuality's wire shape. A bare time.Duration marshals to its raw
// nanosecond int64 by default (e.g. "window": 1800000000000 for 30m) -- caught live in Phase 4
// verification via the analyzer's actual JSON output, not by a unit test, since no test asserted
// the JSON shape directly. window_seconds is unambiguous and matches the existing
// AnalysisRun.DurationMillis convention of "plain number, unit in the field name".
type dataQualityJSON struct {
	Status        DataQualityStatus `json:"status"`
	Coverage      float64           `json:"coverage"`
	WindowSeconds float64           `json:"window_seconds"`
	AsOf          time.Time         `json:"as_of"`
}

func (d DataQuality) MarshalJSON() ([]byte, error) {
	return json.Marshal(dataQualityJSON{Status: d.Status, Coverage: d.Coverage, WindowSeconds: d.Window.Seconds(), AsOf: d.AsOf})
}

func (d *DataQuality) UnmarshalJSON(b []byte) error {
	var w dataQualityJSON
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	d.Status, d.Coverage, d.AsOf = w.Status, w.Coverage, w.AsOf
	d.Window = time.Duration(w.WindowSeconds * float64(time.Second))
	return nil
}

// EvidenceItem is one concrete number backing a Finding, always with the query that produced
// it -- so a finding is checkable, not just assertable.
type EvidenceItem struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Query  string  `json:"query"`
}

// CostImpact is always an *estimate* from configurable unit prices (AGENTS.md: "Cost output is
// always labelled Estimated; a request reduction is a 'potential difference', never 'savings'").
// A nil *CostImpact on a Finding means "not a resource finding", not "zero cost".
type CostImpact struct {
	AllocationCostUSD      float64  `json:"allocation_cost_usd"`
	OptimizedCostUSD       float64  `json:"optimized_cost_usd"`
	PotentialDifferenceUSD float64  `json:"potential_difference_usd"` // AllocationCostUSD - OptimizedCostUSD; never called "savings"
	Assumptions            []string `json:"assumptions"`
}

// Finding is one ranked, evidence-based result of the analyzer, per docs/requirements.md §6.
type Finding struct {
	ID               string         `json:"id"`
	ClusterID        ClusterID      `json:"cluster_id"`
	RuleID           string         `json:"rule_id"`
	AnalysisRunID    string         `json:"analysis_run_id"`
	GeneratedAt      time.Time      `json:"generated_at"`
	Severity         Severity       `json:"severity"`
	Category         Category       `json:"category"`
	Workload         WorkloadRef    `json:"workload"`
	Container        string         `json:"container"`
	Problem          string         `json:"problem"`
	Evidence         []EvidenceItem `json:"evidence"`
	Threshold        string         `json:"threshold"`
	Window           string         `json:"window"`
	DataQuality      DataQuality    `json:"data_quality"`
	Confidence       Confidence     `json:"confidence"`
	ConfidenceReason string         `json:"confidence_reason"`
	Caveats          []string       `json:"caveats"`
	Cost             *CostImpact    `json:"cost"`
}
