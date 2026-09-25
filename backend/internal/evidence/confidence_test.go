package evidence

import (
	"testing"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// fakeStatsSource returns a fixed SeriesStats per requested window, so these tests can exercise
// Classify's tier-selection *policy* directly without constructing a synthetic Prometheus
// matrix with exact timestamps for every scenario (that mechanics-level testing already lives
// in stats_test.go).
type fakeStatsSource map[time.Duration]SeriesStats

func (f fakeStatsSource) Window(end time.Time, window time.Duration) SeriesStats {
	if s, ok := f[window]; ok {
		return s
	}
	return SeriesStats{HasData: false, Window: window}
}

func TestClassify_HighConfidence(t *testing.T) {
	src := fakeStatsSource{highWindow: {HasData: true, Coverage: 0.95, Window: highWindow}}
	dq, conf, reason := Classify(src, time.Now())

	if conf != model.ConfidenceHigh {
		t.Errorf("Confidence = %q, want HIGH", conf)
	}
	if dq.Status != model.DataQualityOK {
		t.Errorf("Status = %q, want ok", dq.Status)
	}
	if reason == "" {
		t.Error("reason is empty, want an explanation")
	}
}

func TestClassify_MediumConfidence_WhenOnly24hClears(t *testing.T) {
	src := fakeStatsSource{
		highWindow:   {HasData: true, Coverage: 0.3, Window: highWindow}, // 7d exists but sparse
		mediumWindow: {HasData: true, Coverage: 0.9, Window: mediumWindow},
	}
	_, conf, _ := Classify(src, time.Now())
	if conf != model.ConfidenceMedium {
		t.Errorf("Confidence = %q, want MEDIUM", conf)
	}
}

func TestClassify_LowConfidence_WhenOnlyMinWindowClears(t *testing.T) {
	src := fakeStatsSource{
		highWindow:   {HasData: true, Coverage: 0.1, Window: highWindow},
		mediumWindow: {HasData: true, Coverage: 0.5, Window: mediumWindow},
		minWindow:    {HasData: true, Coverage: 0.85, Window: minWindow},
	}
	_, conf, reason := Classify(src, time.Now())
	if conf != model.ConfidenceLow {
		t.Errorf("Confidence = %q, want LOW", conf)
	}
	// Regression test for a real bug caught in live verification (Phase 4): the message
	// reported the coverage figure (measured over minWindow=30m) but labelled it as being about
	// mediumWindow=24h, which read as self-contradictory ("87% coverage over 24h (not enough
	// for 24h)") right next to a genuinely low 24h-tier number elsewhere in the same finding.
	if !contains(reason, "30m0s") {
		t.Errorf("reason = %q, want it to name the window the 85%% figure was actually measured over (30m0s)", reason)
	}
	if !contains(reason, "24h0m0s") {
		t.Errorf("reason = %q, want it to name the tier that was NOT reached (24h0m0s)", reason)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestClassify_Insufficient_BelowMinWindow is the direct proof of R005: a workload too young or
// too sparse even for the 30-minute minimum must be gated off, not given a LOW-confidence guess.
func TestClassify_Insufficient_BelowMinWindow(t *testing.T) {
	src := fakeStatsSource{
		minWindow: {HasData: true, Coverage: 0.2, Window: minWindow}, // some data, not enough
	}
	dq, conf, reason := Classify(src, time.Now())

	if dq.Status != model.DataQualityInsufficient {
		t.Errorf("Status = %q, want insufficient", dq.Status)
	}
	if conf != "" {
		t.Errorf("Confidence = %q, want empty (no confidence claim on insufficient data)", conf)
	}
	if reason == "" {
		t.Error("reason is empty, want an explanation")
	}
}

func TestClassify_Insufficient_NoDataAtAll(t *testing.T) {
	dq, _, reason := Classify(fakeStatsSource{}, time.Now())
	if dq.Status != model.DataQualityInsufficient {
		t.Errorf("Status = %q, want insufficient", dq.Status)
	}
	if reason == "" {
		t.Error("reason is empty, want an explanation distinguishing 'no data' from 'some data'")
	}
}

func TestIsStale_RecentSample_NotStale(t *testing.T) {
	now := time.Now()
	stats := SeriesStats{HasData: true, NewestSample: now.Add(-1 * time.Minute)}
	if IsStale(stats, now) {
		t.Error("IsStale() with a 1-minute-old sample: got true, want false")
	}
}

func TestIsStale_OldSample_IsStale(t *testing.T) {
	now := time.Now()
	stats := SeriesStats{HasData: true, NewestSample: now.Add(-10 * time.Minute)}
	if !IsStale(stats, now) {
		t.Error("IsStale() with a 10-minute-old sample: got false, want true")
	}
}

func TestIsStale_NoData_NotFlaggedStale(t *testing.T) {
	// Absence of data is the `insufficient` case, a distinct concept from `stale` (data existed
	// but stopped arriving) -- IsStale must not conflate the two.
	if IsStale(SeriesStats{HasData: false}, time.Now()) {
		t.Error("IsStale() on HasData=false: got true, want false (that's `insufficient`, not `stale`)")
	}
}
