// Package findings orchestrates evidence gathering, rule evaluation and cost estimation into a
// ranked, stably-identified list of model.Finding -- the analyzer's actual output.
package findings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/cost"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/rules"
)

// CostHours is how many hours a CostImpact estimate covers. 730 = the conventional
// hours-per-month figure (365*24/12), so cost findings read as a monthly estimate -- a
// deliberate, documented choice (in every finding's cost.assumptions), not a hidden default.
const CostHours = 730

// Analyzer builds evidence and evaluates rules for a set of workloads.
type Analyzer struct {
	Builder *evidence.Builder
	Pricing cost.Pricing // zero value means "no pricing configured"; HasPricing reports which
	Logger  *slog.Logger
}

// HasPricing reports whether cost.LoadPricing succeeded when this Analyzer was constructed.
func (a *Analyzer) HasPricing() bool {
	return a.Pricing.CPUCoreHourUSD > 0 && a.Pricing.MemoryGiBHourUSD > 0
}

// Analyze evaluates every rule against every container of every given workload and returns a
// ranked finding list, plus every per-workload evidence-gathering failure encountered along the
// way. It never returns a fatal error itself: a failure to gather evidence for one workload is
// recorded (both logged and returned as a model.QueryError) and that workload is simply skipped
// for this run, exactly like the runner's own per-source degradation (AGENTS.md: "degraded
// result, logged, counted"). The caller (internal/runner) is responsible for folding the
// returned errors into the AnalysisRun so a run with successful-but-incomplete workload coverage
// is reported as partial, not complete -- Analyze itself has no notion of "the whole run's
// status", only "what happened while building evidence for these workloads".
func (a *Analyzer) Analyze(ctx context.Context, runID string, workloads []model.WorkloadRef, now time.Time) ([]model.Finding, []model.QueryError) {
	var out []model.Finding
	var errs []model.QueryError

	for _, wl := range workloads {
		containers, err := a.Builder.ResolveContainers(ctx, wl)
		if err != nil {
			a.Logger.Warn("resolving containers failed", "workload", wl.Name, "namespace", wl.Namespace, "error", err)
			errs = append(errs, model.QueryError{
				Source:  "evidence",
				Message: fmt.Sprintf("%s/%s: resolving containers: %s", wl.Namespace, wl.Name, err.Error()),
			})
			continue
		}
		for _, container := range containers {
			ev := a.Builder.Build(ctx, wl, container, now)
			if len(ev.QueryErrors) > 0 {
				a.Logger.Warn("evidence gathering had errors", "workload", wl.Name, "container", container, "errors", ev.QueryErrors)
				for _, qe := range ev.QueryErrors {
					errs = append(errs, model.QueryError{
						Source:  "evidence",
						Message: fmt.Sprintf("%s/%s/%s: %s", wl.Namespace, wl.Name, container, qe),
					})
				}
			}
			for _, rule := range rules.All {
				f := rule(ev, runID, now)
				if f == nil {
					continue
				}
				f.ID = stableID(f)
				if a.HasPricing() && f.Category == model.CategoryResource {
					f.Cost = costFor(a.Pricing, ev, f)
				}
				out = append(out, *f)
			}
		}
	}

	rank(out)
	return out, errs
}

// stableID hashes cluster+rule+namespace+workload+container so that re-running analysis on
// unchanged input produces the same ID (docs/requirements.md §6: "the same inputs produce the
// same finding IDs, so a repeated run updates findings instead of duplicating them").
func stableID(f *model.Finding) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s", f.ClusterID, f.RuleID, f.Workload.Namespace, f.Workload.Name, f.Container)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// costFor extracts the "suggested" value the rule already computed (present in its Evidence
// list under a well-known metric name) and turns it into a CostImpact. Looking it up by name
// rather than threading a Pricing parameter through every rule keeps cost estimation -- an
// optional, cross-cutting concern only 2 of 4 rules produce -- out of rules.go's otherwise pure,
// pricing-independent logic.
func costFor(pricing cost.Pricing, ev evidence.WorkloadEvidence, f *model.Finding) *model.CostImpact {
	switch f.RuleID {
	case "R001":
		suggested, ok := evidenceValue(f, "cpu_suggested_request")
		if !ok {
			return nil
		}
		allocation := max(ev.CPU.RequestCores, ev.CPU.Usage.P95)
		impact := cost.Estimate(pricing, allocation, suggested, 0, 0, CostHours)
		return &impact
	case "R002":
		suggested, ok := evidenceValue(f, "memory_suggested_request")
		if !ok {
			return nil
		}
		allocation := max(ev.Mem.RequestBytes, ev.Mem.Usage.Max)
		impact := cost.Estimate(pricing, 0, 0, allocation, suggested, CostHours)
		return &impact
	default:
		return nil // R003/R004 are health findings, not resource findings; no cost attached
	}
}

func evidenceValue(f *model.Finding, metric string) (float64, bool) {
	for _, e := range f.Evidence {
		if e.Metric == metric {
			return e.Value, true
		}
	}
	return 0, false
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

var severityRank = map[model.Severity]int{model.SeverityCritical: 0, model.SeverityWarning: 1, model.SeverityInfo: 2}
var categoryRank = map[model.Category]int{model.CategoryHealth: 0, model.CategoryResource: 1}

// rank sorts findings health-before-resource, most-severe-first, per docs/requirements.md's MVP
// done-condition ("ranked findings list (health first, then resource, then cost)"), with a
// final deterministic tiebreak on workload name so repeated runs over identical input produce
// identical output ordering.
func rank(fs []model.Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if categoryRank[fs[i].Category] != categoryRank[fs[j].Category] {
			return categoryRank[fs[i].Category] < categoryRank[fs[j].Category]
		}
		if severityRank[fs[i].Severity] != severityRank[fs[j].Severity] {
			return severityRank[fs[i].Severity] < severityRank[fs[j].Severity]
		}
		return fs[i].Workload.Name < fs[j].Workload.Name
	})
}
