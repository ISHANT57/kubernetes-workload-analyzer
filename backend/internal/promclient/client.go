// Package promclient wraps the official Prometheus API client behind a small interface, so the
// rule engine (Phase 4) and this package's own tests never depend on a live Prometheus
// (AGENTS.md: "External calls (Prometheus, Kubernetes) sit behind interfaces").
package promclient

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Client is the analyzer's read-only access to Prometheus.
type Client interface {
	// Healthy checks that Prometheus is reachable and answering queries. Used by the analysis
	// loop and by /readyz.
	Healthy(ctx context.Context) error

	// QueryInstant runs one PromQL instant query (evaluated at time.Now()) and returns its
	// vector result.
	QueryInstant(ctx context.Context, query string) (model.Vector, error)
}

type httpClient struct {
	api apiv1.API
}

// New builds a Client talking to the Prometheus HTTP API at baseURL, e.g.
// "http://localhost:9090" during local development via `kubectl port-forward` (see
// docs/architecture.md "Where it runs").
func New(baseURL string) (Client, error) {
	c, err := api.NewClient(api.Config{Address: baseURL})
	if err != nil {
		return nil, fmt.Errorf("promclient: building client for %s: %w", baseURL, err)
	}
	return &httpClient{api: apiv1.NewAPI(c)}, nil
}

func (c *httpClient) Healthy(ctx context.Context) error {
	// "up" is always a cheap, always-present query on any Prometheus with at least one scrape
	// target -- this checks the whole query path, not just a TCP connection.
	_, err := c.QueryInstant(ctx, "up")
	if err != nil {
		return fmt.Errorf("promclient: health check failed: %w", err)
	}
	return nil
}

func (c *httpClient) QueryInstant(ctx context.Context, query string) (model.Vector, error) {
	// Warnings (e.g. partial results) are intentionally not surfaced here: they are not fatal,
	// and Phase 3 has nothing yet that would act on them. Phase 4's evidence builder is expected
	// to lower confidence when warnings are present, at which point this signature should grow
	// a return value for them rather than silently discarding as now.
	value, _, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("promclient: query %q: %w", query, err)
	}
	vector, ok := value.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("promclient: query %q: unexpected result type %T (want vector)", query, value)
	}
	return vector, nil
}
