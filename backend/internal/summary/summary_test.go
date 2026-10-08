package summary

import (
	"strings"
	"testing"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

func finding(rule string, ev ...model.EvidenceItem) model.Finding {
	return model.Finding{
		RuleID:      rule,
		Workload:    model.WorkloadRef{Kind: "Deployment", Namespace: "demo", Name: "app"},
		Container:   "c",
		Evidence:    ev,
		DataQuality: model.DataQuality{Status: model.DataQualityOK},
	}
}

func ev(metric string, v float64, unit string) model.EvidenceItem {
	return model.EvidenceItem{Metric: metric, Value: v, Unit: unit}
}

func contains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("want %q in %q", want, got)
	}
}

func TestOOMUsesEvidenceAndLimit(t *testing.T) {
	s := For(finding("R004", ev("restarts_24h", 10, "count"), ev("memory_limit", 128*1024*1024, "bytes")))
	contains(t, s.WhatHappened, "10 times")
	contains(t, s.WhatHappened, "128Mi")
	contains(t, s.WhatHappened, "Deployment demo/app (container c)")
	if len(s.NextSteps) == 0 {
		t.Error("no next steps")
	}
}

func TestOOMWithoutLimitSaysSo(t *testing.T) {
	s := For(finding("R004", ev("restarts_24h", 2, "count")))
	contains(t, s.NextSteps[0], "No memory limit was found")
}

func TestCPUOverRequestedIncludesEstimatedCostOnlyWhenPresent(t *testing.T) {
	f := finding("R001", ev("cpu_request", 1, "cores"), ev("cpu_usage_p95", 0.057, "cores"), ev("cpu_suggested_request", 0.069, "cores"))
	s := For(f)
	contains(t, s.WhatHappened, "1.00 cores")
	contains(t, s.WhatHappened, "57m")
	for _, step := range s.NextSteps {
		if strings.Contains(step, "$") {
			t.Errorf("cost line without cost: %q", step)
		}
	}
	f.Cost = &model.CostImpact{PotentialDifferenceUSD: 21.49}
	s = For(f)
	last := s.NextSteps[len(s.NextSteps)-1]
	contains(t, last, "Estimated potential difference")
	contains(t, last, "$21.49")
	if strings.Contains(strings.ToLower(last), "saving") && !strings.Contains(last, "not realised savings") {
		t.Errorf("cost must not be called savings: %q", last)
	}
}

func TestHPACoupledDoesNotAdviseIndependentResize(t *testing.T) {
	f := finding("R001", ev("cpu_request", 0.5, "cores"), ev("cpu_usage_p95", 0.1, "cores"), ev("hpa_target_cpu_utilization_percent", 50, "percent"), ev("hpa_neutral_request", 0.2, "cores"))
	f.Caveats = []string{"hpa-coupled"}
	s := For(f)
	contains(t, s.WhatHappened, "50% CPU target")
	contains(t, s.NextSteps[0], "Do not resize")
	if len(s.Notes) == 0 {
		t.Error("want an HPA note")
	}
}

func TestRestartsStatesCauseUnknown(t *testing.T) {
	s := For(finding("R003", ev("restarts_1h", 7, "count"), ev("restarts_24h", 7, "count"), ev("waiting_reason", 1, "CrashLoopBackOff")))
	contains(t, s.WhatHappened, "7 times in the last hour")
	contains(t, s.WhatHappened, "CrashLoopBackOff")
	contains(t, strings.Join(s.Notes, " "), "does not know why")
}

func TestUnschedulableStatesNoEventsAccess(t *testing.T) {
	s := For(finding("R006", ev("cpu_request", 64, "cores"), ev("memory_request", 256*1024*1024*1024, "bytes")))
	contains(t, s.WhatHappened, "64.00 cores")
	contains(t, s.WhatHappened, "256.00Gi")
	contains(t, strings.Join(s.Notes, " "), "no access to Kubernetes Events")
}

func TestNotesCarryConfidenceAndDataQuality(t *testing.T) {
	f := finding("R002", ev("memory_request", 512*1024*1024, "bytes"), ev("memory_usage_max", 100*1024*1024, "bytes"), ev("memory_suggested_request", 115*1024*1024, "bytes"))
	f.Confidence, f.ConfidenceReason = "LOW", "only 2h of data"
	f.DataQuality.Status = model.DataQualityInsufficient
	joined := strings.Join(For(f).Notes, " ")
	contains(t, joined, "Confidence is LOW: only 2h of data.")
	contains(t, joined, "Data quality is insufficient")
}

func TestUnknownRuleFallsBackToProblemText(t *testing.T) {
	f := finding("R999")
	f.Problem = "Something odd"
	s := For(f)
	contains(t, s.WhatHappened, "Something odd")
	if len(s.NextSteps) == 0 {
		t.Error("fallback needs a next step")
	}
}

func TestDeterministic(t *testing.T) {
	f := finding("R004", ev("restarts_24h", 3, "count"))
	a, b := For(f), For(f)
	if a.WhatHappened != b.WhatHappened || strings.Join(a.NextSteps, "|") != strings.Join(b.NextSteps, "|") {
		t.Error("summary not deterministic")
	}
}

func TestEmptyListsAreNotNil(t *testing.T) {
	s := For(finding("R999"))
	if s.Notes == nil || s.NextSteps == nil {
		t.Error("lists must be non-nil so JSON encodes [] not null")
	}
}
