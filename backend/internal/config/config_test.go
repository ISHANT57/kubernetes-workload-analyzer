package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with no env set: unexpected error: %v", err)
	}
	if cfg.ClusterID == "" {
		t.Error("ClusterID default must not be empty")
	}
	if cfg.PrometheusURL != "http://localhost:9090" {
		t.Errorf("PrometheusURL default = %q, want http://localhost:9090", cfg.PrometheusURL)
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr default = %q, want loopback-only 127.0.0.1:8080", cfg.ListenAddr)
	}
	if cfg.AnalysisInterval != 5*time.Minute {
		t.Errorf("AnalysisInterval default = %s, want 5m", cfg.AnalysisInterval)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLUSTER_ID", "test-cluster")
	t.Setenv("PROMETHEUS_URL", "http://prom.example:9090")
	t.Setenv("ANALYSIS_INTERVAL", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): unexpected error: %v", err)
	}
	if cfg.ClusterID != "test-cluster" {
		t.Errorf("ClusterID = %q, want test-cluster", cfg.ClusterID)
	}
	if cfg.PrometheusURL != "http://prom.example:9090" {
		t.Errorf("PrometheusURL = %q, want http://prom.example:9090", cfg.PrometheusURL)
	}
	if cfg.AnalysisInterval != 30*time.Second {
		t.Errorf("AnalysisInterval = %s, want 30s", cfg.AnalysisInterval)
	}
}

// TestLoad_EmptyClusterID_FallsBackToDefault documents the actual, intended behavior: an
// explicitly empty CLUSTER_ID is treated the same as an unset one and gets the default, the
// same rule Load applies uniformly to every optional string field. It is not an error case --
// see the comment in Load() next to the removed "must not be empty" check for why an earlier
// version of this test (asserting an error here) was itself testing an unreachable code path.
func TestLoad_EmptyClusterID_FallsBackToDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLUSTER_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with CLUSTER_ID=\"\": unexpected error: %v", err)
	}
	if cfg.ClusterID == "" {
		t.Error("Load() with CLUSTER_ID=\"\": ClusterID is still empty, want the default")
	}
}

func TestLoad_RejectsInvalidDuration(t *testing.T) {
	clearEnv(t)
	t.Setenv("ANALYSIS_INTERVAL", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Error("Load() with ANALYSIS_INTERVAL=\"not-a-duration\": want error, got nil")
	}
}

func TestLoad_RejectsNonPositiveInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("ANALYSIS_INTERVAL", "0s")

	if _, err := Load(); err == nil {
		t.Error("Load() with ANALYSIS_INTERVAL=0s: want error, got nil")
	}
}

func TestLoad_ReportsAllErrorsAtOnce(t *testing.T) {
	clearEnv(t)
	t.Setenv("ANALYSIS_INTERVAL", "not-a-duration")
	t.Setenv("PROMETHEUS_TIMEOUT", "also-not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load(): want error, got nil")
	}
	// Both problems should be visible in one error, not just the first -- a config with two
	// typos should not require two restarts to find them both.
	msg := err.Error()
	if !contains(msg, "ANALYSIS_INTERVAL") || !contains(msg, "PROMETHEUS_TIMEOUT") {
		t.Errorf("Load() error = %q, want it to mention both ANALYSIS_INTERVAL and PROMETHEUS_TIMEOUT", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// clearEnv resets every env var Load() reads, so tests do not leak state between each other or
// pick up the real developer's environment.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"CLUSTER_ID", "PROMETHEUS_URL", "KUBECONFIG_PATH", "KUBE_CONTEXT",
		"LISTEN_ADDR", "ANALYSIS_INTERVAL", "PROMETHEUS_TIMEOUT", "KUBERNETES_TIMEOUT", "ANALYSIS_TIMEOUT",
	} {
		t.Setenv(k, "")
	}
}
