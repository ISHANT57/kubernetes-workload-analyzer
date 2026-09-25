// Package metrics defines the analyzer's own Prometheus metrics -- the project must be
// observable about itself, not just about the cluster it watches (see original project brief
// §15 "Observability of our own platform"). Exposed at /metrics via internal/httpserver.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RunsTotal counts every completed analysis run, labelled by its outcome. A dashboard or
	// alert can watch rate(analyzer_analysis_runs_total{status="failed"}[15m]) without needing
	// to scrape the JSON API at all.
	RunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analyzer_analysis_runs_total",
		Help: "Total analysis runs, labelled by outcome status (complete|partial|failed).",
	}, []string{"status"})

	// RunDurationSeconds tracks how long each run takes -- the project brief's "analysis
	// duration" self-observability requirement.
	RunDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "analyzer_analysis_run_duration_seconds",
		Help:    "Duration of each analysis run, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	// QueryErrorsTotal counts connectivity/query failures by source, so "collector failures"
	// (brief §15) has a real counter, not just a log line.
	QueryErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analyzer_query_errors_total",
		Help: "Total query/connectivity errors during analysis runs, labelled by source (prometheus|kubernetes).",
	}, []string{"source"})

	// WorkloadsSeen is the "number of monitored resources" self-observability metric (brief
	// §15). It reflects the sticky, carried-forward count the runner keeps (see runner.go), so
	// it does not spuriously drop to zero on a single transient Kubernetes-listing failure.
	WorkloadsSeen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "analyzer_workloads_seen",
		Help: "Number of workloads observed in the most recent successful listing.",
	})
)
