# Performance

Project brief §18: "do not optimize blindly, first measure. Never invent numbers." Every number
below is a real measurement taken against the live in-cluster analyzer (`svc/analyzer`,
`deploy/analyzer/deployment.yaml`) on 2026-09-26, on the single-node `kind` cluster described in
`docs/architecture.md`. This is a laptop, single-node measurement, not an isolated benchmark rig —
treat these as order-of-magnitude indicators for a portfolio deployment, not an SLA.

## Method

```bash
kubectl -n analyzer port-forward svc/analyzer 8080:8080 &
cd backend && go run ./cmd/loadtest -base http://localhost:8080 -concurrency 20 -duration 15s
kubectl -n analyzer top pod                      # CPU/memory during and after the load
curl -s localhost:8080/metrics | grep ^analyzer_analysis_run_duration_seconds
```
`cmd/loadtest` (`backend/cmd/loadtest/main.go`, new, stdlib only) fires 20 concurrent workers per
endpoint for 15s and reports p50/p95/p99/max latency, request count and error count — the "basic
load test" the brief's §17 testing section asks for.

## API latency (5 endpoints, 20 concurrent workers, 15s each)

| Endpoint | Requests | p50 | p95 | p99 | max | Errors |
|---|---|---|---|---|---|---|
| `/healthz` | 68,229 | 3.3 ms | 10.5 ms | 15.0 ms | 42.7 ms | 0 |
| `/readyz` | 46,589 | 4.8 ms | 14.9 ms | 19.8 ms | 52.5 ms | 0 |
| `/api/runs/latest` | 49,113 | 4.6 ms | 14.3 ms | 18.2 ms | 37.5 ms | 0 |
| `/api/findings` | 42,028 | 5.6 ms | 15.8 ms | 20.3 ms | 32.1 ms | 0 |
| `/api/timeseries` (cpu, 24h) | 5,862 | 47.5 ms | 83.8 ms | 113.1 ms | 198.0 ms | 0 |

Zero errors and zero non-200 responses across all ~212,000 requests. `/api/timeseries` is roughly
an order of magnitude slower than the other endpoints because it queries Prometheus live on every
request (`internal/evidence/builder.go`'s `CPUUsageSeries`/`MemoryUsageSeries`) instead of reading
the in-memory analysis snapshot that `/api/findings` and `/api/runs/latest` serve from
(`internal/runner`). That difference is expected and by design (§4 of `docs/DECISIONS.md` D-002:
the browser never sends PromQL, but the server still has to ask Prometheus for a fresh series each
time) — not investigated further as a bug.

## Resource usage under load

```
NAME                        CPU(cores)   MEMORY(bytes)
analyzer-57fb6b8948-4wf6g   1614m        26Mi
```
Measured with `kubectl top pod` immediately after the load test above. The pod's configured
resources (`deploy/analyzer/deployment.yaml`, set in Phase 7 from idle `kubectl top` readings):
```
requests: {cpu: 25m, memory: 32Mi}
limits:   {memory: 128Mi}        # no CPU limit set
```
**Finding:** under this synthetic load (100 concurrent workers total, ~14,000 req/s combined),
measured CPU usage reached 1614m — about 65x the 25m request. No CPU limit is set, so nothing
throttled and the pod did not restart or OOM (memory stayed at 26Mi, well under the 128Mi limit,
`RESTARTS: 0`). The 25m/32Mi request reflects idle steady-state polling, not burst read traffic;
this synthetic test used far more concurrent API load than the single-page dashboard in
`frontend/` generates in normal use. **Recommendation** (not applied — a resource-limit change to
an already-hardened Deployment is a real trade-off, left for the owner): if this analyzer were
exposed to real concurrent traffic rather than one dashboard tab, add a CPU request/limit sized
from this measurement (e.g. `request: 100m, limit: 1`) rather than leaving CPU unbounded.

## Analysis run duration (self-observability, `analyzer_analysis_run_duration_seconds`)

Pulled from the analyzer's own `/metrics` after 29 real analysis cycles (30s-interval loop,
in-cluster):
```
analyzer_analysis_run_duration_seconds_count 29
analyzer_analysis_run_duration_seconds_sum   18.644171357
analyzer_analysis_runs_total{status="complete"} 28
analyzer_analysis_runs_total{status="partial"}  1   # the deliberate Phase 7 Prometheus outage test
```
Mean duration ≈ 0.643s. All 29 runs completed in under 1 second (the `le="1"` bucket already
covers all 29; the `le="0.5"` bucket covers 6). The one `partial` run is the Phase 7 outage test,
not an unexplained failure — `docs/runbook.md` "Failure-path demo" reproduces it.

## What was not measured

- **Sustained, long-duration load** (this test ran the endpoints hard for 15s each, not hours).
- **Multi-client / multi-tab dashboard load** — the load test hits the API directly, not through
  the React frontend's polling hook.
- **Cross-node or multi-replica behavior** — single node, single analyzer replica, matching the
  v1 scope (`docs/DECISIONS.md` D-005).
