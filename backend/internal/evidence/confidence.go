package evidence

import (
	"fmt"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
)

// Confidence tier windows and the coverage bar each must clear, per docs/requirements.md §3.
// MinWindow is the shortest history a finding is allowed to be based on at all -- below it, R005
// gates the rule off entirely rather than emitting a LOW-confidence guess.
const (
	minWindow    = 30 * time.Minute
	mediumWindow = 24 * time.Hour
	highWindow   = 7 * 24 * time.Hour
	coverageBar  = 0.8
	staleAfter   = 5 * time.Minute
)

// Classify turns one metric's SeriesStats (already fetched over at least highWindow) into the
// DataQuality/Confidence pair a Finding reports. It tries the longest window first: HIGH needs
// 7d at >=80% coverage, MEDIUM needs 24h at >=80%, otherwise 30m at >=80% still allows a LOW
// finding. Below that, the metric is `insufficient` and no resource finding may be based on it.
func Classify(matrix7d SeriesStatsSource, now time.Time) (model.DataQuality, model.Confidence, string) {
	statsHigh := matrix7d.Window(now, highWindow)
	if statsHigh.HasData && statsHigh.Coverage >= coverageBar {
		return dq(model.DataQualityOK, statsHigh, now),
			model.ConfidenceHigh,
			fmt.Sprintf("%.0f%% coverage over %s", statsHigh.Coverage*100, highWindow)
	}

	statsMedium := matrix7d.Window(now, mediumWindow)
	if statsMedium.HasData && statsMedium.Coverage >= coverageBar {
		return dq(model.DataQualityOK, statsMedium, now),
			model.ConfidenceMedium,
			fmt.Sprintf("%.0f%% coverage over %s, but not %s", statsMedium.Coverage*100, mediumWindow, highWindow)
	}

	statsLow := matrix7d.Window(now, minWindow)
	if statsLow.HasData && statsLow.Coverage >= coverageBar {
		return dq(model.DataQualityOK, statsLow, now),
			model.ConfidenceLow,
			fmt.Sprintf("only %.0f%% coverage over %s (not enough for %s)", statsLow.Coverage*100, minWindow, mediumWindow)
	}

	// Not even the minimum window clears the bar: this metric cannot support a resource finding.
	reason := fmt.Sprintf("less than %.0f%% coverage over the minimum %s window", coverageBar*100, minWindow)
	if !statsLow.HasData {
		reason = fmt.Sprintf("no data at all in the last %s", minWindow)
	}
	return model.DataQuality{Status: model.DataQualityInsufficient, Coverage: statsLow.Coverage, Window: minWindow, AsOf: now},
		"", reason
}

// IsStale reports whether the newest sample is old enough that this evidence should not be
// trusted even if its historical coverage looked fine -- e.g. scraping broke an hour ago but
// there is still a week of good history behind that.
func IsStale(stats SeriesStats, now time.Time) bool {
	return stats.HasData && now.Sub(stats.NewestSample) > staleAfter
}

func dq(status model.DataQualityStatus, s SeriesStats, now time.Time) model.DataQuality {
	return model.DataQuality{Status: status, Coverage: s.Coverage, Window: s.Window, AsOf: now}
}

// SeriesStatsSource lets Classify re-derive stats at several window tiers from data already
// fetched once (see computeStatsWindow) without this file needing to import model.Matrix
// directly -- keeping the confidence-tiering *policy* (the windows/bar above) separate from how
// the underlying data was fetched.
type SeriesStatsSource interface {
	Window(end time.Time, window time.Duration) SeriesStats
}
