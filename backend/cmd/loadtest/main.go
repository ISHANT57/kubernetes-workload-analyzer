// Command loadtest is a manual benchmarking tool: it fires concurrent GET requests at a running
// analyzer's read endpoints for a fixed duration and reports p50/p95/p99/max latency, throughput
// and error rate per endpoint. Stdlib only -- no new dependency for a one-off measurement tool
// (project brief §18 "performance: do not optimize blindly, first measure").
//
// Usage (against a port-forwarded analyzer):
//
//	kubectl -n analyzer port-forward svc/analyzer 8080:8080 &
//	go run ./cmd/loadtest -base http://localhost:8080 -concurrency 20 -duration 20s
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type result struct {
	path    string
	latency time.Duration
	status  int
	err     error
}

func main() {
	base := flag.String("base", "http://localhost:8080", "analyzer base URL")
	concurrency := flag.Int("concurrency", 20, "concurrent workers per endpoint")
	duration := flag.Duration("duration", 20*time.Second, "how long to load each endpoint")
	flag.Parse()

	endpoints := []string{
		"/healthz",
		"/readyz",
		"/api/runs/latest",
		"/api/findings",
		"/api/timeseries?namespace=demo&workload=cpu-over-requested&container=c&metric=cpu&window=24h",
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, path := range endpoints {
		results := run(client, *base+path, path, *concurrency, *duration)
		report(path, results)
	}
}

func run(client *http.Client, url, path string, concurrency int, duration time.Duration) []result {
	var mu sync.Mutex
	var results []result
	var wg sync.WaitGroup

	stop := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				start := time.Now()
				resp, err := client.Get(url)
				lat := time.Since(start)
				status := 0
				if err == nil {
					status = resp.StatusCode
					resp.Body.Close()
				}
				mu.Lock()
				results = append(results, result{path: path, latency: lat, status: status, err: err})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

func report(path string, results []result) {
	if len(results) == 0 {
		fmt.Printf("%-70s no requests completed\n", path)
		return
	}
	var lats []time.Duration
	errs := 0
	nonOK := 0
	for _, r := range results {
		if r.err != nil {
			errs++
			continue
		}
		if r.status != http.StatusOK {
			nonOK++
		}
		lats = append(lats, r.latency)
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		idx := int(p * float64(len(lats)-1))
		return lats[idx]
	}

	fmt.Printf("%s\n", path)
	fmt.Printf("  requests=%d errors=%d non_200=%d\n", len(results), errs, nonOK)
	if len(lats) > 0 {
		fmt.Printf("  p50=%s p95=%s p99=%s max=%s\n", pct(0.50), pct(0.95), pct(0.99), lats[len(lats)-1])
	}
	fmt.Fprintln(os.Stderr, "---")
}
