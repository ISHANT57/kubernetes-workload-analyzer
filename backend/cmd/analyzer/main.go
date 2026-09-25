// Command analyzer is the Kubernetes Workload Analyzer backend. Phase 3 scope: wire up config,
// logging, a Prometheus client, a Kubernetes client, the analysis loop, and the operational HTTP
// endpoints. No rule engine yet -- that is Phase 4.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/config"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/httpserver"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/k8sclient"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/logging"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/model"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/promclient"
	"github.com/ISHANT57/kubernetes-workload-analyzer/backend/internal/runner"
)

func main() {
	logger := logging.New(slog.LevelInfo)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration loaded",
		"cluster_id", cfg.ClusterID,
		"prometheus_url", cfg.PrometheusURL,
		"listen_addr", cfg.ListenAddr,
		"analysis_interval", cfg.AnalysisInterval,
	)

	// Client construction here does not dial the network -- both libraries only parse config
	// and build local objects, so an error at this point means genuine misconfiguration (a bad
	// URL, an unreadable kubeconfig), not a transient outage. That is worth failing fast on;
	// transient outages are what the runner's per-run error handling exists for.
	promClient, err := promclient.New(cfg.PrometheusURL)
	if err != nil {
		logger.Error("failed to build prometheus client", "error", err)
		os.Exit(1)
	}

	k8sClient, err := k8sclient.New(model.ClusterID(cfg.ClusterID), cfg.Kubeconfig, cfg.KubeContext)
	if err != nil {
		logger.Error("failed to build kubernetes client", "error", err)
		os.Exit(1)
	}

	analysisRunner := runner.New(
		model.ClusterID(cfg.ClusterID),
		cfg.AnalysisInterval,
		cfg.PrometheusTimeout,
		cfg.KubernetesTimeout,
		promClient,
		k8sClient,
		logger,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go analysisRunner.Run(ctx)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpserver.New(analysisRunner, logger),
		ReadHeaderTimeout: 5 * time.Second, // never accept a client that trickles headers forever
	}

	serverErrs := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrs <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErrs:
		logger.Error("http server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown did not complete cleanly", "error", err)
	}
	logger.Info("shutdown complete")
}
