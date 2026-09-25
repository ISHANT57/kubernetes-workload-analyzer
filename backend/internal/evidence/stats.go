// Package evidence builds per-workload facts (usage stats, requests/limits, restarts, HPA
// coupling) from Prometheus and Kubernetes, for the rule engine (internal/rules) to evaluate.
// It is the only package that queries Prometheus/Kubernetes for rule purposes -- rules
// themselves are pure functions over the types defined here.
package evidence

import (
	"math"
	"sort"
	"time"

	"github.com/prometheus/common/model"
)

// SeriesStats summarizes one Prometheus range query's result: percentile/max values plus a
// self-calibrated coverage figure.
//
// Coverage is deliberately NOT computed from an assumed scrape interval (e.g. "expected samples
// = window / 30s"). That was tried first and checked against real data from the demo/ fixtures:
// a workload alive for ~15h had 823 samples where a 30s-interval assumption predicted ~1816 --
// off by more than 2x, most likely because of scrape-job cadence differences and/or real gaps
// (kubelet restarts, laptop activity) that a fixed-interval formula can't see. Coverage here is
// instead derived only from the actual sample timestamps returned: how much of the requested
// window is spanned by data, penalized for the single largest gap found in it. This is honest
// about what "coverage" can mean without assuming facts about the scrape configuration that
// were not verified.
type SeriesStats struct {
	HasData      bool
	P50          float64
	P95          float64
	P99          float64
	Max          float64
	Count        int
	OldestSample time.Time
	NewestSample time.Time
	Coverage     float64       // 0..1: how much of Window is reliably covered by data
	Window       time.Duration // the window this was computed over
}

// Burstiness is the p99/p50 ratio docs/requirements.md §3 uses to detect a workload that is
// idle most of the time but genuinely spikes (e.g. the cpu-spike demo fixture): a plain
// over-provisioned check using only p95/max would misread that shape as waste. Returns 0 when
// there isn't a meaningful p50 to divide by (no data, or a p50 of exactly 0).
func (s SeriesStats) Burstiness() float64 {
	if !s.HasData {
		return 0
	}
	if s.P50 <= 0 {
		if s.P99 > 0 {
			// A real bug, caught live (Phase 4): the cpu-spike demo fixture is idle more than
			// half the time, so its true median is exactly 0 -- the single clearest possible
			// burstiness signal (p50=0 with a real p99 tail). An earlier version returned 0
			// here (the same value as "no signal at all"), which silently disabled the guard
			// for precisely the extreme case it exists to catch. Infinity correctly compares
			// greater than any finite guard threshold; it is the honest value for "no
			// meaningful median to divide by, but the tail is real".
			return math.Inf(1)
		}
		return 0 // genuinely no usage anywhere: not bursty, just idle/absent
	}
	return s.P99 / s.P50
}

// computeStats reduces a Prometheus range-query matrix (possibly several series, e.g. during a
// pod replacement where two pod names briefly both have data) into one SeriesStats over the
// requested window.
func computeStats(matrix model.Matrix, window time.Duration) SeriesStats {
	type point struct {
		t time.Time
		v float64
	}
	var points []point
	for _, series := range matrix {
		for _, sample := range series.Values {
			v := float64(sample.Value)
			if math.IsNaN(v) {
				continue
			}
			points = append(points, point{t: sample.Timestamp.Time(), v: v})
		}
	}
	if len(points) == 0 {
		return SeriesStats{HasData: false, Window: window}
	}

	sort.Slice(points, func(i, j int) bool { return points[i].t.Before(points[j].t) })

	oldest := points[0].t
	newest := points[len(points)-1].t
	span := newest.Sub(oldest)

	// Largest gap between consecutive samples -- a single long blackout (suspend, kubelet
	// restart, scrape outage) should visibly reduce coverage even if there is plenty of data
	// either side of it.
	var largestGap time.Duration
	for i := 1; i < len(points); i++ {
		if gap := points[i].t.Sub(points[i-1].t); gap > largestGap {
			largestGap = gap
		}
	}

	spanCoverage := float64(span) / float64(window)
	if spanCoverage > 1 {
		spanCoverage = 1
	}
	gapPenalty := float64(largestGap) / float64(window)
	if gapPenalty > 1 {
		gapPenalty = 1
	}
	coverage := spanCoverage * (1 - gapPenalty)
	if coverage < 0 {
		coverage = 0
	}

	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.v
	}

	return SeriesStats{
		HasData:      true,
		P50:          percentile(values, 0.50),
		P95:          percentile(values, 0.95),
		P99:          percentile(values, 0.99),
		Max:          max(values),
		Count:        len(points),
		OldestSample: oldest,
		NewestSample: newest,
		Coverage:     coverage,
		Window:       window,
	}
}

// percentile is a simple nearest-rank percentile over a copy of values (does not mutate the
// caller's slice). Good enough at the sample counts a single container's history produces
// (hundreds to low thousands); no need for a streaming/approximate algorithm here.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// computeStatsWindow computes SeriesStats over [end-window, end] from a matrix that may span a
// wider range -- this lets one Prometheus range query (fetched once, over the widest window
// needed) be reused to evaluate several confidence tiers (30m / 24h / 7d) without a separate
// network round trip per tier.
func computeStatsWindow(matrix model.Matrix, end time.Time, window time.Duration) SeriesStats {
	start := end.Add(-window)
	filtered := make(model.Matrix, 0, len(matrix))
	for _, series := range matrix {
		var kept []model.SamplePair
		for _, sample := range series.Values {
			t := sample.Timestamp.Time()
			if !t.Before(start) && !t.After(end) {
				kept = append(kept, sample)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, &model.SampleStream{Metric: series.Metric, Values: kept})
		}
	}
	return computeStats(filtered, window)
}

// MatrixSource adapts a Prometheus range-query result (fetched once, over the widest window a
// rule might need) into a SeriesStatsSource, so Classify (confidence.go) can re-derive stats at
// several window tiers without a separate network round trip per tier.
type MatrixSource struct {
	Matrix model.Matrix
}

func (s MatrixSource) Window(end time.Time, window time.Duration) SeriesStats {
	return computeStatsWindow(s.Matrix, end, window)
}

func max(values []float64) float64 {
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
