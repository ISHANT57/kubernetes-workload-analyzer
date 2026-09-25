package findings

import (
	"testing"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/cost"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

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
