// Package runner implements the analysis loop: on a timer, it checks Prometheus and Kubernetes
// connectivity, evaluates the rule engine (Phase 4: internal/findings) against every discovered
// workload, and keeps the latest AnalysisRun and finding list in memory (docs/architecture.md
// "Analysis loop"): a failed run never wipes out the last good snapshot, it only marks it stale.
package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/findings"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/k8sclient"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/metrics"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/promclient"
)

// Runner owns the analysis loop and the latest snapshot.
type Runner struct {
	clusterID       model.ClusterID
	interval        time.Duration
	promTimeout     time.Duration // bounds every Prometheus call this loop makes
	k8sTimeout      time.Duration // bounds every Kubernetes call this loop makes
	analysisTimeout time.Duration // bounds one whole findings pass, not just one query in it
	prom            promclient.Client
	k8s             k8sclient.Client
	analyzer        *findings.Analyzer // nil is valid: rule evaluation is skipped, connectivity checks still run
	logger          *slog.Logger

	mu                    sync.RWMutex
	latest                *model.AnalysisRun // nil until the first run completes
	latestFindings        []model.Finding    // sticky: only replaced when a run's K8s+Prometheus checks both succeed
	lastGoodWorkloadsSeen int                // sticky across a failed K8s call; see runOnce
}

// New builds a Runner. It does not start the loop; call Run for that. promTimeout and
// k8sTimeout bound every individual call the loop makes to each source (AGENTS.md: "explicit
// error handling ... never a request that can hang forever") -- they come from
// config.Config.PrometheusTimeout/KubernetesTimeout, not from interval, so a slow query cannot
// by itself stall the whole analysis loop past its next scheduled tick. analyzer may be nil
// (e.g. no cost pricing yet or findings deliberately disabled); the loop then still performs its
// connectivity checks but never runs the rule engine.
func New(clusterID model.ClusterID, interval, promTimeout, k8sTimeout, analysisTimeout time.Duration, prom promclient.Client, k8s k8sclient.Client, analyzer *findings.Analyzer, logger *slog.Logger) *Runner {
	return &Runner{
		clusterID:       clusterID,
		interval:        interval,
		promTimeout:     promTimeout,
		k8sTimeout:      k8sTimeout,
		analysisTimeout: analysisTimeout,
		prom:            prom,
		k8s:             k8s,
		analyzer:        analyzer,
		logger:          logger,
	}
}

// Run blocks, running one pass immediately and then one every interval, until ctx is cancelled.
// This is meant to be called in its own goroutine from main.
func (r *Runner) Run(ctx context.Context) {
	r.runOnce(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("analysis loop stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// Latest returns the most recent completed run, or nil if none has completed yet (e.g. the
// process just started and the first run is still in flight, or failed before producing
// anything usable).
func (r *Runner) Latest() *model.AnalysisRun {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.latest
}

// Findings returns the most recent successfully-computed finding list. Sticky across a run that
// could not gather workload data (Prometheus or Kubernetes down): the caller still sees the last
// good findings rather than an empty list that would look like "everything is fine now".
// AnalysisRun.Status on the concurrently-returned Latest() is what tells a caller whether this
// list is fresh or stale.
func (r *Runner) Findings() []model.Finding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.latestFindings
}

func (r *Runner) runOnce(ctx context.Context) {
	start := time.Now()
	run := &model.AnalysisRun{
		ID:        newRunID(start),
		ClusterID: r.clusterID,
		StartedAt: start,
		// Initialized non-nil so a run with no errors serializes as "query_errors": [] rather
		// than "query_errors": null -- a JSON API consumer (the Phase 5 dashboard) should never
		// need a null-check before iterating this field.
		QueryErrors: []model.QueryError{},
	}

	// Prometheus check. A failure here does not abort the run -- it is recorded and the run
	// continues, per AGENTS.md: "Handle failure explicitly ... degraded result, logged, counted
	// in a metric" and docs/architecture.md's failure table ("Prometheus down or timing out:
	// keep last snapshot marked stale ... error counter increments"). Bounded by promTimeout so
	// a hung Prometheus cannot stall this run past its own timeout, let alone the whole process.
	promCtx, cancelProm := context.WithTimeout(ctx, r.promTimeout)
	promErr := r.prom.Healthy(promCtx)
	cancelProm()
	if promErr != nil {
		r.logger.Warn("prometheus health check failed", "error", promErr)
		run.QueryErrors = append(run.QueryErrors, model.QueryError{Source: "prometheus", Message: promErr.Error()})
		metrics.QueryErrorsTotal.WithLabelValues("prometheus").Inc()
	}

	// Kubernetes check + workload count. Same non-aborting, timeout-bounded treatment. On
	// failure, WorkloadsSeen carries forward the last successful count rather than dropping to
	// 0: per docs/architecture.md, a failed check must degrade the *freshness* of the snapshot,
	// not silently erase it. The run's own Status and QueryErrors still truthfully say this
	// cycle's listing failed -- a caller wanting "is this number fresh" reads those, not
	// WorkloadsSeen in isolation.
	k8sCtx, cancelK8s := context.WithTimeout(ctx, r.k8sTimeout)
	refs, k8sErr := r.k8s.ListWorkloads(k8sCtx)
	cancelK8s()
	if k8sErr != nil {
		r.logger.Warn("kubernetes workload listing failed", "error", k8sErr)
		run.QueryErrors = append(run.QueryErrors, model.QueryError{Source: "kubernetes", Message: k8sErr.Error()})
		metrics.QueryErrorsTotal.WithLabelValues("kubernetes").Inc()
		run.WorkloadsSeen = r.lastGoodWorkloadsSeen
	} else {
		run.WorkloadsSeen = len(refs)
		r.lastGoodWorkloadsSeen = run.WorkloadsSeen
	}

	// Rule evaluation only runs when both sources are healthy this cycle and an Analyzer is
	// configured: evidence gathering needs a live Prometheus to query and a live workload list
	// to iterate, so attempting it during an outage would just be wasted queries producing
	// mostly query_error evidence. The last good finding list stays as-is (sticky), and this
	// run's Status/QueryErrors already say honestly that nothing fresh was computed.
	//
	// sourceFailures counts only the two top-level connectivity checks above (prometheus,
	// kubernetes) -- statusFor uses this, not len(run.QueryErrors), to decide "failed" vs
	// "partial", so per-workload evidence failures (appended below) can never by themselves push
	// a run to "failed": both top-level sources being reachable means *some* usable data exists
	// this cycle, even if a subset of workloads' evidence could not be gathered.
	sourceFailures := 0
	if promErr != nil {
		sourceFailures++
	}
	if k8sErr != nil {
		sourceFailures++
	}
	workloadFailures := 0

	if r.analyzer != nil && promErr == nil && k8sErr == nil {
		analysisCtx, cancelAnalysis := context.WithTimeout(ctx, r.analysisTimeout)
		fresh, workloadErrs := r.analyzer.Analyze(analysisCtx, run.ID, refs, start)
		cancelAnalysis()
		// Surfaced in the run's own QueryErrors (safely: the same Source/Message shape already
		// used for the prometheus/kubernetes checks above, no new exposure) instead of only a
		// log line -- a workload whose evidence could not be gathered must be visible to an API
		// consumer, not silently dropped from a run that otherwise looks clean.
		run.QueryErrors = append(run.QueryErrors, workloadErrs...)
		workloadFailures = len(workloadErrs)
		run.FindingsCount = len(fresh)
		r.mu.Lock()
		r.latestFindings = fresh
		r.mu.Unlock()
	} else if r.analyzer != nil {
		r.mu.RLock()
		run.FindingsCount = len(r.latestFindings)
		r.mu.RUnlock()
	}

	run.Status = statusFor(sourceFailures, workloadFailures)
	run.DurationMillis = time.Since(start).Milliseconds()

	metrics.RunsTotal.WithLabelValues(string(run.Status)).Inc()
	metrics.RunDurationSeconds.Observe(time.Since(start).Seconds())
	metrics.WorkloadsSeen.Set(float64(run.WorkloadsSeen))

	r.logger.Info("analysis run complete",
		"run_id", run.ID,
		"status", run.Status,
		"duration_ms", run.DurationMillis,
		"workloads_seen", run.WorkloadsSeen,
		"query_errors", len(run.QueryErrors),
	)

	r.mu.Lock()
	r.latest = run
	r.mu.Unlock()
}

// statusFor decides the run's overall status.
//   - no source failures, no workload failures  -> complete
//   - some degradation, but not total           -> partial (a snapshot still exists, just degraded)
//   - every top-level source failed              -> failed (nothing usable was produced this run)
//
// sourceFailures counts only the two top-level connectivity checks (Prometheus, Kubernetes) --
// Phase 3 checks exactly two, so "every source failed" means both; this generalizes correctly
// once a later phase adds more sources without needing a rewrite. workloadFailures counts
// per-workload evidence-gathering errors (Phase 4's rule engine, once both top-level sources are
// reachable): these can only ever degrade a run to "partial", never "failed" -- both top-level
// sources being reachable means this run produced *some* usable data, even if a subset of
// workloads' evidence could not be gathered. This is what makes "some workload evidence fails
// while sources stay up" report as partial instead of the misleadingly clean "complete" it used
// to report before this distinction existed.
func statusFor(sourceFailures, workloadFailures int) model.RunStatus {
	const totalSources = 2
	switch {
	case sourceFailures >= totalSources:
		return model.RunFailed
	case sourceFailures > 0 || workloadFailures > 0:
		return model.RunPartial
	default:
		return model.RunComplete
	}
}

// newRunID builds a short, sufficiently-unique run identifier without pulling in a UUID
// dependency: a run happens at most every few seconds in practice (AnalysisInterval), so a
// timestamp plus 4 random bytes gives no realistic collision risk for this process's lifetime.
func newRunID(t time.Time) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf) // crypto/rand.Read on Linux/Darwin/Windows never returns an error in
	// practice; if it somehow did, the zero buffer still yields a valid (just less random) ID
	// rather than crashing the analysis loop over a non-essential ID's uniqueness.
	return "run-" + t.UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf)
}
