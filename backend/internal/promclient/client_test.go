package promclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// vectorResponse is a minimal, valid Prometheus /api/v1/query response body for a vector result
// with one sample -- enough to exercise the real HTTP round trip through client_golang's api
// package, not just this package's own logic.
const vectorResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {"metric": {"__name__": "up", "job": "prometheus"}, "value": [1700000000, "1"]}
    ]
  }
}`

func TestQueryInstant_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, vectorResponse)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New(%q): unexpected error: %v", srv.URL, err)
	}

	vec, err := c.QueryInstant(context.Background(), "up")
	if err != nil {
		t.Fatalf("QueryInstant(): unexpected error: %v", err)
	}
	if len(vec) != 1 {
		t.Fatalf("QueryInstant(): got %d samples, want 1", len(vec))
	}
	if got := string(vec[0].Metric["job"]); got != "prometheus" {
		t.Errorf("sample job label = %q, want prometheus", got)
	}
}

func TestHealthy_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, vectorResponse)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New(%q): unexpected error: %v", srv.URL, err)
	}
	if err := c.Healthy(context.Background()); err != nil {
		t.Errorf("Healthy(): unexpected error: %v", err)
	}
}

func TestHealthy_Unreachable(t *testing.T) {
	// A server that immediately closes every connection stands in for "Prometheus is down" --
	// this is the exact scenario docs/architecture.md's failure table names: "Prometheus down or
	// timing out ... keep last snapshot marked stale; /readyz fails". This test proves the
	// client surfaces that as an error rather than hanging or panicking.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	addr := srv.URL
	srv.Close() // close immediately: nothing is listening at all, the simplest "unreachable"

	c, err := New(addr)
	if err != nil {
		t.Fatalf("New(%q): unexpected error: %v", addr, err)
	}
	if err := c.Healthy(context.Background()); err == nil {
		t.Error("Healthy() against an unreachable server: want error, got nil")
	}
}

func TestQueryInstant_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New(%q): unexpected error: %v", srv.URL, err)
	}
	if _, err := c.QueryInstant(context.Background(), "up"); err == nil {
		t.Error("QueryInstant() against a 500-returning server: want error, got nil")
	}
}
