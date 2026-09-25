package rules

import (
	"testing"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/evidence"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// okDQ is a convenience DataQuality value for synthetic fixtures that should pass the R005 gate.
func okDQ() model.DataQuality {
	return model.DataQuality{Status: model.DataQualityOK, Coverage: 0.9}
}

// These fixtures mirror the 9 demo/ scenarios in demo/expected-findings.yaml -- this file is the
// unit-level counterpart of that live ground truth: it proves the rule *logic* is correct given
// known inputs, independent of a live cluster (which cmd/verify checks separately against real
// data). Where a fixture's real observed numbers are known (measured 2026-09-25/26), they are
// used directly rather than invented round numbers.

func rightSizedEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "right-sized"},
		CPU: evidence.CPUEvidence{
			RequestCores: 0.5, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, P50: 0.40, P95: 0.40, P99: 0.41},
			DataQuality: okDQ(),
		},
		Mem: evidence.MemEvidence{
			RequestBytes: 128 * 1024 * 1024, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, Max: 72 * 1024 * 1024},
			DataQuality: okDQ(),
		},
		HasRestartsData: true,
	}
}

func cpuOverRequestedEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "cpu-over-requested"},
		CPU: evidence.CPUEvidence{
			RequestCores: 1.0, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, P50: 0.05, P95: 0.055, P99: 0.06},
			DataQuality: okDQ(),
		},
		Mem: evidence.MemEvidence{
			RequestBytes: 64 * 1024 * 1024, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, Max: 1.5 * 1024 * 1024},
			DataQuality: okDQ(),
		},
		HasRestartsData: true,
	}
}

func memoryOverRequestedEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "memory-over-requested"},
		CPU: evidence.CPUEvidence{
			RequestCores: 0.1, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, P50: 0.0001, P95: 0.0002, P99: 0.0002},
			DataQuality: okDQ(),
		},
		Mem: evidence.MemEvidence{
			RequestBytes: 1024 * 1024 * 1024, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, Max: 60 * 1024 * 1024},
			DataQuality: okDQ(),
		},
		HasRestartsData: true,
	}
}

func cpuSpikeEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "cpu-spike"},
		CPU: evidence.CPUEvidence{
			RequestCores: 0.5, HasRequest: true,
			// Idle more than half the time, bursts briefly: real live data for this fixture
			// (measured 2026-09-26 with a 2m rate window) showed P50 exactly 0 -- more than
			// half of all evaluated points had zero overlap with the 30s-every-300s burst.
			Usage:       evidence.SeriesStats{HasData: true, P50: 0, P95: 0.30, P99: 0.33},
			DataQuality: okDQ(),
		},
		Mem: evidence.MemEvidence{
			RequestBytes: 64 * 1024 * 1024, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, Max: 1 * 1024 * 1024},
			DataQuality: okDQ(),
		},
		HasRestartsData: true,
	}
}

func oomEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload:             model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "oom"},
		LastTerminatedReason: "OOMKilled",
		WaitingReason:        "CrashLoopBackOff",
		RestartsIncrease1h:   13,
		RestartsIncrease24h:  76,
		HasRestartsData:      true,
		CPU:                  evidence.CPUEvidence{RequestCores: 0.05, HasRequest: true, DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		Mem:                  evidence.MemEvidence{RequestBytes: 128 * 1024 * 1024, HasRequest: true, HasLimit: true, LimitBytes: 128 * 1024 * 1024, DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
	}
}

func crashloopEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload:             model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "crashloop"},
		LastTerminatedReason: "Error",
		WaitingReason:        "CrashLoopBackOff",
		RestartsIncrease1h:   13,
		RestartsIncrease24h:  77,
		HasRestartsData:      true,
		CPU:                  evidence.CPUEvidence{DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		Mem:                  evidence.MemEvidence{DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
	}
}

func hpaCoupledEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "hpa-coupled"},
		CPU: evidence.CPUEvidence{
			RequestCores: 0.4, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, P50: 0.005, P95: 0.006, P99: 0.006},
			DataQuality: okDQ(),
		},
		Mem: evidence.MemEvidence{
			RequestBytes: 64 * 1024 * 1024, HasRequest: true,
			Usage:       evidence.SeriesStats{HasData: true, Max: 5.4 * 1024 * 1024},
			DataQuality: okDQ(),
		},
		HasRestartsData: true,
		HPA:             evidence.HPAEvidence{Present: true, TargetsCPUUtilization: true, CPUTargetUtilizationPercent: 70},
	}
}

func newWorkloadEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload:        model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "new-workload"},
		CPU:             evidence.CPUEvidence{RequestCores: 0.1, HasRequest: true, DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		Mem:             evidence.MemEvidence{RequestBytes: 32 * 1024 * 1024, HasRequest: true, DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		HasRestartsData: true,
	}
}

func pendingEvidence() evidence.WorkloadEvidence {
	return evidence.WorkloadEvidence{
		Workload: model.WorkloadRef{Namespace: "demo", Kind: "Deployment", Name: "pending"},
		CPU:      evidence.CPUEvidence{DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		Mem:      evidence.MemEvidence{DataQuality: model.DataQuality{Status: model.DataQualityInsufficient}},
		// HasRestartsData false: a pod that never scheduled has no restart counter at all.
	}
}

func fired(ev evidence.WorkloadEvidence) map[string]bool {
	now := time.Now()
	got := map[string]bool{}
	for _, rule := range All {
		if f := rule(ev, "run-1", now); f != nil {
			got[f.RuleID] = true
		}
	}
	return got
}

func TestRules_RightSized_NoFindings(t *testing.T) {
	got := fired(rightSizedEvidence())
	if len(got) != 0 {
		t.Errorf("right-sized fired %v, want no findings (negative control)", got)
	}
}

func TestRules_CPUOverRequested_FiresR001Only(t *testing.T) {
	got := fired(cpuOverRequestedEvidence())
	want := map[string]bool{"R001": true}
	assertSameRules(t, got, want)
}

func TestRules_MemoryOverRequested_FiresR002Only(t *testing.T) {
	got := fired(memoryOverRequestedEvidence())
	want := map[string]bool{"R002": true}
	assertSameRules(t, got, want)
}

// TestRules_CPUSpike_SuppressedByBurstinessGuard is the direct proof of the bursty caveat: a
// workload whose p95 looks like classic over-provisioning must NOT fire R001 once its p99/p50
// ratio reveals a spike shape.
func TestRules_CPUSpike_SuppressedByBurstinessGuard(t *testing.T) {
	ev := cpuSpikeEvidence()
	if ratio := ev.CPU.Usage.Burstiness(); ratio <= burstinessGuard {
		t.Fatalf("test fixture invalid: burstiness ratio %.1f must exceed the %.1f guard for this test to mean anything", ratio, burstinessGuard)
	}
	got := fired(ev)
	if got["R001"] {
		t.Error("cpu-spike fired R001: want it suppressed by the burstiness guard")
	}
}

func TestRules_OOM_FiresR004AndR003(t *testing.T) {
	got := fired(oomEvidence())
	want := map[string]bool{"R004": true, "R003": true}
	assertSameRules(t, got, want)
}

func TestRules_Crashloop_FiresR003Only_NotR004(t *testing.T) {
	got := fired(crashloopEvidence())
	want := map[string]bool{"R003": true}
	assertSameRules(t, got, want)
	if got["R004"] {
		t.Error("crashloop (reason=Error) fired R004: R004 must only fire on reason=OOMKilled")
	}
}

// TestRules_HPACoupled_NoPlainR001_CaveatInstead is the direct proof that an HPA-managed
// workload never gets a plain resize recommendation (Phase 1 Lab 05: doing so inflated total
// allocation 1->3 replicas). It still emits a finding, just an info-severity, caveated one.
func TestRules_HPACoupled_NoPlainR001_CaveatInstead(t *testing.T) {
	ev := hpaCoupledEvidence()
	now := time.Now()
	f := R001CPUOverRequested(ev, "run-1", now)
	if f == nil {
		t.Fatal("R001 on an HPA-coupled workload: got nil, want an info-level HPA-coupled finding")
	}
	if f.Severity != model.SeverityInfo {
		t.Errorf("Severity = %q, want info (not a warning-level plain resize)", f.Severity)
	}
	found := false
	for _, c := range f.Caveats {
		if c == "hpa-coupled" {
			found = true
		}
	}
	if !found {
		t.Errorf("Caveats = %v, want to include \"hpa-coupled\"", f.Caveats)
	}
}

func TestRules_NewWorkload_InsufficientData_NoResourceFindings(t *testing.T) {
	got := fired(newWorkloadEvidence())
	if got["R001"] || got["R002"] {
		t.Errorf("new-workload (insufficient data) fired %v, want R001/R002 both suppressed", got)
	}
}

func TestRules_Pending_NoFindings(t *testing.T) {
	got := fired(pendingEvidence())
	if len(got) != 0 {
		t.Errorf("pending fired %v, want no findings (no MVP rule covers unschedulable pods yet)", got)
	}
}

// --- Targeted threshold-boundary tests, not tied to a demo fixture ---

func TestR001_DoesNotFire_WhenThrottled(t *testing.T) {
	ev := cpuOverRequestedEvidence()
	ev.CPU.HasThrottlingData = true
	ev.CPU.ThrottledRatio = 0.10 // 10%, above the 5% guard
	if f := R001CPUOverRequested(ev, "run-1", time.Now()); f != nil {
		t.Error("R001 fired despite >=5% CPU throttling: a throttled container needs more CPU, not less")
	}
}

func TestR001_DoesNotFire_WhenDataInsufficient(t *testing.T) {
	ev := cpuOverRequestedEvidence()
	ev.CPU.DataQuality.Status = model.DataQualityInsufficient
	if f := R001CPUOverRequested(ev, "run-1", time.Now()); f != nil {
		t.Error("R001 fired on insufficient data quality: the R005 gate must suppress it")
	}
}

func TestR002_DoesNotFire_AfterRecentOOM(t *testing.T) {
	ev := memoryOverRequestedEvidence()
	ev.LastTerminatedReason = "OOMKilled"
	ev.RestartsIncrease24h = 2
	if f := R002MemoryOverRequested(ev, "run-1", time.Now()); f != nil {
		t.Error("R002 fired right after a recent OOM: must never suggest shrinking memory in that case")
	}
}

func TestR003_DoesNotFire_BelowBothThresholds(t *testing.T) {
	ev := rightSizedEvidence()
	ev.RestartsIncrease1h = 1  // below 3
	ev.RestartsIncrease24h = 1 // below 10 -- e.g. a single SandboxChanged restart, observed live in Phase 4 verification
	if f := R003FrequentRestarts(ev, "run-1", time.Now()); f != nil {
		t.Error("R003 fired below both thresholds: one infrequent restart must not be flagged as instability")
	}
}

func assertSameRules(t *testing.T, got, want map[string]bool) {
	t.Helper()
	for id := range want {
		if !got[id] {
			t.Errorf("rule %s did not fire; got %v, want %v", id, got, want)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("rule %s fired unexpectedly; got %v, want %v", id, got, want)
		}
	}
}
