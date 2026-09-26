package findings

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	promcommon "github.com/prometheus/common/model"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/cost"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// scriptedPromClient answers PromQL queries by substring match on the query text, so a test can
// make exactly one workload's evidence gathering fail (or one metric fire a rule) without having
// to replicate every query the evidence builder issues. Unmatched queries return an empty,
// error-free result -- the same "no data" outcome a real Prometheus gives for a series that
// doesn't exist, never used to force a false failure.
type scriptedPromClient struct {
	instant func(query string) (promcommon.Vector, error)
}

func (f *scriptedPromClient) Healthy(ctx context.Context) error { return nil }
func (f *scriptedPromClient) QueryInstant(ctx context.Context, query string) (promcommon.Vector, error) {
	if f.instant != nil {
		return f.instant(query)
	}
	return nil, nil
}
func (f *scriptedPromClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (promcommon.Matrix, error) {
	return nil, nil
}

func testAnalyzerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// scriptedBuilder wires a scriptedPromClient's canned answers to the query shapes actually
// issued by evidence.Builder: pod resolution (resolvePodNames, shared by ResolveContainers and
// Build) and the container-listing query always succeed unless the workload's name matches
// failWorkload (pod resolution then errors, exactly like a real connectivity failure). When
// fireRestarts is true, every surviving workload's restart-count query returns a value above
// R003's threshold, so a "good" workload really produces a finding -- not just succeeds with
// zero findings, which would under-test "findings from successful workloads are preserved".
func scriptedBuilder(failWorkload string, fireRestarts bool) *evidence.Builder {
	return &evidence.Builder{Prom: &scriptedPromClient{
		instant: func(query string) (promcommon.Vector, error) {
			switch {
			case failWorkload != "" && strings.Contains(query, `owner_name="`+failWorkload+`"`):
				return nil, errors.New("dial tcp: connection refused")
			case strings.Contains(query, "kube_pod_owner"):
				return promcommon.Vector{{Metric: promcommon.Metric{"pod": "pod-1"}, Value: 1}}, nil
			case strings.Contains(query, "count by (container)"):
				return promcommon.Vector{{Metric: promcommon.Metric{"container": "c"}, Value: 1}}, nil
			case fireRestarts && strings.Contains(query, "kube_pod_container_status_restarts_total"):
				return promcommon.Vector{{Value: 5}}, nil
			default:
				return nil, nil
			}
		},
	}}
}

func TestStableID_DeterministicForSameInput(t *testing.T) {
	f1 := &model.Finding{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "c"}
	f2 := &model.Finding{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "c"}

	if stableID(f1) != stableID(f2) {
		t.Error("stableID differs for identical (cluster, rule, namespace, workload, container): a repeated run would duplicate findings instead of updating them")
	}
}

func TestStableID_DiffersOnAnyKeyField(t *testing.T) {
	base := &model.Finding{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "c"}
	variants := []*model.Finding{
		{ClusterID: "c2", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "c"},
		{ClusterID: "c1", RuleID: "R002", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "c"},
		{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "other", Name: "w1"}, Container: "c"},
		{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w2"}, Container: "c"},
		{ClusterID: "c1", RuleID: "R001", Workload: model.WorkloadRef{Namespace: "demo", Name: "w1"}, Container: "other"},
	}
	baseID := stableID(base)
	for i, v := range variants {
		if stableID(v) == baseID {
			t.Errorf("variant %d: stableID matched the base despite differing in one key field", i)
		}
	}
}

func TestRank_HealthBeforeResource(t *testing.T) {
	fs := []model.Finding{
		{RuleID: "R001", Category: model.CategoryResource, Severity: model.SeverityWarning, Workload: model.WorkloadRef{Name: "a"}},
		{RuleID: "R004", Category: model.CategoryHealth, Severity: model.SeverityCritical, Workload: model.WorkloadRef{Name: "b"}},
	}
	rank(fs)
	if fs[0].Category != model.CategoryHealth {
		t.Errorf("rank()[0].Category = %q, want health first regardless of severity", fs[0].Category)
	}
}

func TestRank_MostSevereFirstWithinCategory(t *testing.T) {
	fs := []model.Finding{
		{RuleID: "R003", Category: model.CategoryHealth, Severity: model.SeverityWarning, Workload: model.WorkloadRef{Name: "a"}},
		{RuleID: "R004", Category: model.CategoryHealth, Severity: model.SeverityCritical, Workload: model.WorkloadRef{Name: "b"}},
	}
	rank(fs)
	if fs[0].Severity != model.SeverityCritical {
		t.Errorf("rank()[0].Severity = %q, want critical first", fs[0].Severity)
	}
}

func TestRank_DeterministicTiebreak(t *testing.T) {
	fs := []model.Finding{
		{RuleID: "R001", Category: model.CategoryResource, Severity: model.SeverityWarning, Workload: model.WorkloadRef{Name: "zebra"}},
		{RuleID: "R001", Category: model.CategoryResource, Severity: model.SeverityWarning, Workload: model.WorkloadRef{Name: "apple"}},
	}
	rank(fs)
	if fs[0].Workload.Name != "apple" {
		t.Errorf("rank()[0].Workload.Name = %q, want apple (alphabetical tiebreak within equal category/severity)", fs[0].Workload.Name)
	}
}

func TestCostFor_R001_UsesMaxOfRequestAndP95(t *testing.T) {
	pricing := cost.Pricing{CPUCoreHourUSD: 0.10, MemoryGiBHourUSD: 0.02, Source: "test"}
	ev := evidence.WorkloadEvidence{
		CPU: evidence.CPUEvidence{RequestCores: 1.0, Usage: evidence.SeriesStats{P95: 0.05}},
	}
	f := &model.Finding{RuleID: "R001", Evidence: []model.EvidenceItem{{Metric: "cpu_suggested_request", Value: 0.06}}}

	impact := costFor(pricing, ev, f)
	if impact == nil {
		t.Fatal("costFor() returned nil, want a CostImpact")
	}
	wantAllocation := 1.0 * 0.10 * CostHours // allocation = max(1.0 request, 0.05 usage) = 1.0
	if impact.AllocationCostUSD != wantAllocation {
		t.Errorf("AllocationCostUSD = %v, want %v (allocation should use the request, since it's larger than p95 usage)", impact.AllocationCostUSD, wantAllocation)
	}
}

func TestCostFor_R003_NoCost(t *testing.T) {
	pricing := cost.Pricing{CPUCoreHourUSD: 0.10, MemoryGiBHourUSD: 0.02, Source: "test"}
	f := &model.Finding{RuleID: "R003"}
	if impact := costFor(pricing, evidence.WorkloadEvidence{}, f); impact != nil {
		t.Error("costFor() on a health finding (R003): want nil, health findings carry no cost")
	}
}

func TestCostFor_MissingSuggestedEvidence_ReturnsNil(t *testing.T) {
	// Defensive: if a rule's Evidence shape ever changes and the expected metric name
	// disappears, costFor must fail closed (no cost) rather than silently compute $0.
	pricing := cost.Pricing{CPUCoreHourUSD: 0.10, MemoryGiBHourUSD: 0.02, Source: "test"}
	f := &model.Finding{RuleID: "R001", Evidence: []model.EvidenceItem{{Metric: "something_else", Value: 1}}}
	if impact := costFor(pricing, evidence.WorkloadEvidence{}, f); impact != nil {
		t.Error("costFor() with no cpu_suggested_request evidence: want nil, not a fabricated cost")
	}
}

// --- Analyze() failure accounting -------------------------------------------------------
//
// The bug this covers: a per-workload evidence-gathering failure (a bad Prometheus query, a
// broken owner-chain join) used to be logged and silently dropped, so a run where some workloads
// failed and others succeeded still reported "complete" -- indistinguishable from a fully clean
// run. Analyze must instead return every such failure so the caller (internal/runner) can mark
// the run partial, while still returning findings for every workload that did succeed.

func TestAnalyze_AllWorkloadsSucceed_NoErrors(t *testing.T) {
	workloads := []model.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "good-a"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "good-b"},
	}
	a := &Analyzer{Builder: scriptedBuilder("", true), Logger: testAnalyzerLogger()}

	fs, errs := a.Analyze(context.Background(), "run-1", workloads, time.Now())

	if len(errs) != 0 {
		t.Errorf("errs = %v, want none: no workload's evidence gathering failed", errs)
	}
	if len(fs) == 0 {
		t.Error("findings = [], want at least one: both workloads have restart data above the R003 threshold")
	}
}

func TestAnalyze_SomeWorkloadFails_ErrorReturnedForThatWorkloadOnly(t *testing.T) {
	workloads := []model.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "broken"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "healthy"},
	}
	a := &Analyzer{Builder: scriptedBuilder("broken", true), Logger: testAnalyzerLogger()}

	_, errs := a.Analyze(context.Background(), "run-1", workloads, time.Now())

	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly 1 (only the broken workload failed)", errs)
	}
	if errs[0].Source != "evidence" {
		t.Errorf("errs[0].Source = %q, want %q", errs[0].Source, "evidence")
	}
	if !strings.Contains(errs[0].Message, "broken") {
		t.Errorf("errs[0].Message = %q, want it to name the failing workload so the failure is traceable", errs[0].Message)
	}
}

func TestAnalyze_SuccessfulWorkloadFindingsArePreserved(t *testing.T) {
	workloads := []model.WorkloadRef{
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "broken"},
		{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "healthy"},
	}
	a := &Analyzer{Builder: scriptedBuilder("broken", true), Logger: testAnalyzerLogger()}

	fs, errs := a.Analyze(context.Background(), "run-1", workloads, time.Now())

	if len(errs) != 1 {
		t.Fatalf("setup: errs = %v, want exactly 1", errs)
	}
	if len(fs) == 0 {
		t.Fatal("findings = [], want at least one from the healthy workload despite the broken one failing")
	}
	for _, f := range fs {
		if f.Workload.Name == "broken" {
			t.Errorf("got a finding for the broken workload (%+v): it should have been skipped, not partially evaluated", f)
		}
	}
	found := false
	for _, f := range fs {
		if f.Workload.Name == "healthy" {
			found = true
		}
	}
	if !found {
		t.Error("no finding for the healthy workload: a failure elsewhere must not suppress it")
	}
}

func TestAnalyze_QueryErrorsExposedSafely(t *testing.T) {
	workloads := []model.WorkloadRef{{ClusterID: "c1", Namespace: "demo", Kind: "Deployment", Name: "broken"}}
	a := &Analyzer{Builder: scriptedBuilder("broken", false), Logger: testAnalyzerLogger()}

	_, errs := a.Analyze(context.Background(), "run-1", workloads, time.Now())

	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly 1", errs)
	}
	// "Exposed safely": identifiable (which workload, which stage) without leaking anything a
	// credential/token error message could carry -- the underlying error here is a plain
	// connection-refused, same shape already accepted for the runner's own prometheus/kubernetes
	// QueryErrors, not a new exposure surface.
	msg := errs[0].Message
	if !strings.Contains(msg, "demo") || !strings.Contains(msg, "broken") {
		t.Errorf("Message = %q, want it to identify namespace and workload", msg)
	}
	if strings.Contains(strings.ToLower(msg), "authorization") || strings.Contains(strings.ToLower(msg), "bearer") {
		t.Errorf("Message = %q, must never contain credential material", msg)
	}
}
