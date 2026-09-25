// Package config loads the analyzer's configuration from environment variables (AGENTS.md:
// "config through environment variables"), with explicit validation and no silent defaults for
// anything security-relevant.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config is the analyzer's full runtime configuration.
type Config struct {
	// ClusterID identifies this cluster in every emitted fact (AGENTS.md: "Model ClusterID in
	// core types even though v1 has one cluster"). Defaults to "kind-workload-analyzer" if
	// unset; set CLUSTER_ID explicitly before ever pointing this at a second real cluster, so
	// facts from two clusters are never silently mislabelled as the same one.
	ClusterID string

	// PrometheusURL is the base URL of the Prometheus HTTP API. Defaults to
	// http://localhost:9090, matching local development via `kubectl port-forward`; set it to
	// an in-cluster Service URL once deployed (Phase 7).
	PrometheusURL string

	// Kubeconfig is the path to a kubeconfig file. Empty means "use in-cluster config" (Phase 7
	// deployment); during local development it defaults to $KUBECONFIG or ~/.kube/config via
	// client-go's own clientcmd loading rules (see internal/k8sclient).
	Kubeconfig string

	// KubeContext optionally selects a specific context from the kubeconfig, so a shared
	// kubeconfig with multiple clusters (e.g. this laptop's) never silently targets the wrong
	// one. Empty means "use the kubeconfig's current-context".
	KubeContext string

	// ListenAddr is the address the HTTP server (/healthz, /readyz, /metrics) binds to.
	// Defaults to loopback-only, matching the threat model's "local-only until auth exists".
	ListenAddr string

	// AnalysisInterval is how often the analysis loop runs.
	AnalysisInterval time.Duration

	// PrometheusTimeout bounds every individual Prometheus query (AGENTS.md: explicit failure
	// handling, never a request that can hang forever).
	PrometheusTimeout time.Duration

	// KubernetesTimeout bounds every individual Kubernetes API call.
	KubernetesTimeout time.Duration

	// AnalysisTimeout bounds one whole findings pass (evidence gathering + rule evaluation
	// across every workload/container), not just one query within it. A single pass can issue
	// well over a hundred Prometheus queries for a modest number of workloads (verified: ~12-14
	// per container), so a per-query timeout alone does not stop the whole pass from running
	// arbitrarily long if Prometheus is merely slow rather than fully down.
	AnalysisTimeout time.Duration
}

// Load reads configuration from the environment, applies defaults for optional fields, and
// returns an error naming every problem found (not just the first) so a misconfiguration is
// fixed in one pass instead of one restart per typo.
func Load() (Config, error) {
	cfg := Config{
		ClusterID:     getEnv("CLUSTER_ID", "kind-workload-analyzer"),
		PrometheusURL: getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		Kubeconfig:    os.Getenv("KUBECONFIG_PATH"), // empty is valid: means "use default loading rules"
		KubeContext:   os.Getenv("KUBE_CONTEXT"),
		ListenAddr:    getEnv("LISTEN_ADDR", "127.0.0.1:8080"),
	}

	var errs []error

	interval, err := getDuration("ANALYSIS_INTERVAL", 5*time.Minute)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AnalysisInterval = interval

	promTimeout, err := getDuration("PROMETHEUS_TIMEOUT", 10*time.Second)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.PrometheusTimeout = promTimeout

	k8sTimeout, err := getDuration("KUBERNETES_TIMEOUT", 10*time.Second)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.KubernetesTimeout = k8sTimeout

	analysisTimeout, err := getDuration("ANALYSIS_TIMEOUT", 2*time.Minute)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AnalysisTimeout = analysisTimeout

	// ClusterID and PrometheusURL have no "must not be empty" check here: getEnv already
	// substitutes their defaults for an empty/unset value, so cfg.ClusterID and
	// cfg.PrometheusURL can never actually be "" once Load reaches this point. An earlier
	// version of this function checked for it anyway -- a validation branch that can
	// structurally never fire, which is worse than no check at all (it looks like protection
	// that is not really there). If either field is ever changed to have no default, this is
	// exactly where that check belongs again.
	if cfg.AnalysisInterval <= 0 {
		errs = append(errs, fmt.Errorf("ANALYSIS_INTERVAL must be positive, got %s", cfg.AnalysisInterval))
	}
	if cfg.PrometheusTimeout <= 0 {
		errs = append(errs, fmt.Errorf("PROMETHEUS_TIMEOUT must be positive, got %s", cfg.PrometheusTimeout))
	}
	if cfg.KubernetesTimeout <= 0 {
		errs = append(errs, fmt.Errorf("KUBERNETES_TIMEOUT must be positive, got %s", cfg.KubernetesTimeout))
	}
	if cfg.AnalysisTimeout <= 0 {
		errs = append(errs, fmt.Errorf("ANALYSIS_TIMEOUT must be positive, got %s", cfg.AnalysisTimeout))
	}

	if len(errs) > 0 {
		return Config{}, joinErrors(errs)
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, v, err)
	}
	return d, nil
}

// joinErrors combines multiple config errors into one, in place of errors.Join (stdlib since
// go1.20) purely so the message stays readable on one screen without extra formatting help.
func joinErrors(errs []error) error {
	msg := "invalid configuration:"
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
