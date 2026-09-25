package evidence

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func matrixOf(baseTime time.Time, step time.Duration, values ...float64) model.Matrix {
	samples := make([]model.SamplePair, len(values))
	for i, v := range values {
		samples[i] = model.SamplePair{
			Timestamp: model.TimeFromUnixNano(baseTime.Add(time.Duration(i) * step).UnixNano()),
			Value:     model.SampleValue(v),
		}
	}
	return model.Matrix{{Metric: model.Metric{"pod": "p"}, Values: samples}}
}

func TestComputeStats_NoData(t *testing.T) {
	stats := computeStats(model.Matrix{}, time.Hour)
	if stats.HasData {
		t.Error("computeStats on an empty matrix: HasData = true, want false")
	}
}

func TestComputeStats_DenseFullWindow_HighCoverage(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	// 121 samples at 30s over exactly 1 hour, no gaps.
	values := make([]float64, 121)
	for i := range values {
		values[i] = 0.4
	}
	m := matrixOf(base, 30*time.Second, values...)

	stats := computeStats(m, time.Hour)
	if !stats.HasData {
		t.Fatal("HasData = false, want true")
	}
	if stats.Coverage < 0.95 {
		t.Errorf("Coverage = %.3f, want >= 0.95 for dense full-window data", stats.Coverage)
	}
	if stats.Max != 0.4 || stats.P95 != 0.4 {
		t.Errorf("Max=%.3f P95=%.3f, want both 0.4 (constant series)", stats.Max, stats.P95)
	}
}

func TestComputeStats_OneBigGap_LowCoverage(t *testing.T) {
	base := time.Now().Add(-2 * time.Hour)
	// A cluster of samples right at the start, then nothing until near the end -- simulates a
	// laptop suspend: data exists at both ends of the window but with one large blackout.
	m := model.Matrix{{
		Metric: model.Metric{"pod": "p"},
		Values: []model.SamplePair{
			{Timestamp: model.TimeFromUnixNano(base.UnixNano()), Value: 0.1},
			{Timestamp: model.TimeFromUnixNano(base.Add(30 * time.Second).UnixNano()), Value: 0.1},
			{Timestamp: model.TimeFromUnixNano(base.Add(115 * time.Minute).UnixNano()), Value: 0.1},
			{Timestamp: model.TimeFromUnixNano(base.Add(116 * time.Minute).UnixNano()), Value: 0.1},
		},
	}}

	stats := computeStats(m, 2*time.Hour)
	if stats.Coverage > 0.2 {
		t.Errorf("Coverage = %.3f, want a low value: one ~113-minute gap dominates a 120-minute window", stats.Coverage)
	}
}

func TestComputeStats_PartialWindow_ProportionalCoverage(t *testing.T) {
	// Data exists densely, but only for the first quarter of the requested window -- e.g. a
	// workload created partway through what would otherwise be a 24h evaluation window.
	base := time.Now().Add(-24 * time.Hour)
	values := make([]float64, 6*60/1) // 6 hours of 1-per-minute samples
	for i := range values {
		values[i] = 0.5
	}
	m := matrixOf(base, time.Minute, values...)

	stats := computeStats(m, 24*time.Hour)
	if stats.Coverage < 0.20 || stats.Coverage > 0.30 {
		t.Errorf("Coverage = %.3f, want roughly 0.25 (6h of dense data inside a 24h window)", stats.Coverage)
	}
}

func TestComputeStats_IgnoresNaN(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	m := model.Matrix{{
		Metric: model.Metric{"pod": "p"},
		Values: []model.SamplePair{
			{Timestamp: model.TimeFromUnixNano(base.UnixNano()), Value: model.SampleValue(0.3)},
			{Timestamp: model.TimeFromUnixNano(base.Add(30 * time.Second).UnixNano()), Value: model.SampleValue(math.NaN())},
			{Timestamp: model.TimeFromUnixNano(base.Add(60 * time.Second).UnixNano()), Value: model.SampleValue(0.5)},
		},
	}}

	stats := computeStats(m, time.Hour)
	if stats.Count != 2 {
		t.Errorf("Count = %d, want 2 (the NaN sample must be excluded)", stats.Count)
	}
	if stats.Max != 0.5 {
		t.Errorf("Max = %.3f, want 0.5", stats.Max)
	}
}

// TestComputeStatsWindow_TiersFromOneFetch is the direct proof of the "fetch once, evaluate at
// several window tiers" design: 7 days of dense data should look like high coverage at every
// tier (30m/24h/7d), all computed from the same matrix.
func TestComputeStatsWindow_TiersFromOneFetch(t *testing.T) {
	now := time.Now()
	base := now.Add(-7 * 24 * time.Hour)
	n := int(7 * 24 * time.Hour / time.Minute) // one sample per minute for 7 days
	values := make([]float64, n)
	for i := range values {
		values[i] = 0.3
	}
	m := matrixOf(base, time.Minute, values...)

	for _, tc := range []struct {
		window time.Duration
		name   string
	}{
		{30 * time.Minute, "30m"},
		{24 * time.Hour, "24h"},
		{7 * 24 * time.Hour, "7d"},
	} {
		stats := computeStatsWindow(m, now, tc.window)
		if !stats.HasData {
			t.Errorf("[%s] HasData = false, want true", tc.name)
			continue
		}
		if stats.Coverage < 0.9 {
			t.Errorf("[%s] Coverage = %.3f, want >= 0.9 for dense per-minute data", tc.name, stats.Coverage)
		}
	}
}

func TestComputeStatsWindow_OnlyRecentData_LowCoverageAt7d(t *testing.T) {
	// A workload with only 2 hours of real history (e.g. the "new-workload" demo fixture)
	// queried over a 7d window must show low, not fabricated, coverage at that tier -- this is
	// the exact mechanism R005 depends on to gate R001/R002 for young workloads.
	now := time.Now()
	base := now.Add(-2 * time.Hour)
	values := make([]float64, 120) // 1/min for 2h
	for i := range values {
		values[i] = 0.1
	}
	m := matrixOf(base, time.Minute, values...)

	stats7d := computeStatsWindow(m, now, 7*24*time.Hour)
	if stats7d.Coverage > 0.05 {
		t.Errorf("7d-tier Coverage = %.3f, want near 0 (only 2h of a 168h window has data)", stats7d.Coverage)
	}
	stats30m := computeStatsWindow(m, now, 30*time.Minute)
	if stats30m.Coverage < 0.9 {
		t.Errorf("30m-tier Coverage = %.3f, want high (the recent 30m is densely covered)", stats30m.Coverage)
	}
}

// TestBurstiness_ZeroMedianWithRealTail_IsInfinite is a regression test for a real bug caught
// live in Phase 4: the cpu-spike demo fixture's true P50 is exactly 0 (idle more than half the
// time), and an earlier version of Burstiness() returned 0 for that case -- identical to "no
// signal at all" -- which silently disabled the R001 burstiness guard for precisely the
// workload shape it exists to catch.
func TestBurstiness_ZeroMedianWithRealTail_IsInfinite(t *testing.T) {
	s := SeriesStats{HasData: true, P50: 0, P99: 0.3}
	got := s.Burstiness()
	if !math.IsInf(got, 1) {
		t.Errorf("Burstiness() with P50=0, P99=0.3 = %v, want +Inf", got)
	}
	if got <= burstinessGuardForTest {
		t.Errorf("Burstiness() = %v must compare greater than any finite guard threshold", got)
	}
}

const burstinessGuardForTest = 4.0 // mirrors rules.burstinessGuard without importing internal/rules (would create an import cycle)

func TestBurstiness_NoUsageAtAll_IsZero(t *testing.T) {
	s := SeriesStats{HasData: true, P50: 0, P99: 0}
	if got := s.Burstiness(); got != 0 {
		t.Errorf("Burstiness() with P50=0, P99=0 (genuinely no usage) = %v, want 0, not a false bursty signal", got)
	}
}

func TestBurstiness_NoData_IsZero(t *testing.T) {
	if got := (SeriesStats{HasData: false}).Burstiness(); got != 0 {
		t.Errorf("Burstiness() on HasData=false = %v, want 0", got)
	}
}

func TestPercentile_P95(t *testing.T) {
	// 100 values 1..100 -> nearest-rank p95 is the 95th value = 95.
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i + 1)
	}
	if got := percentile(values, 0.95); got != 95 {
		t.Errorf("percentile(1..100, 0.95) = %v, want 95", got)
	}
}

func TestPercentile_DoesNotMutateInput(t *testing.T) {
	values := []float64{5, 1, 3}
	_ = percentile(values, 0.5)
	if values[0] != 5 || values[1] != 1 || values[2] != 3 {
		t.Errorf("percentile() mutated its input: got %v, want [5 1 3] unchanged", values)
	}
}
