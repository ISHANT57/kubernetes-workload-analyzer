# backend

Go analyzer skeleton (Phase 3). No rule engine yet — see `docs/requirements.md` §3 for R001–R005,
which land in Phase 4. This phase is the plumbing: config, structured logging, a Prometheus
client, a Kubernetes client (scoped to D-007), an analysis loop that smoke-tests both and keeps
an in-memory snapshot, and the operational HTTP endpoints.

## Layout

```
cmd/analyzer/main.go        wiring only: config -> clients -> runner -> http server
internal/config/            env-var configuration, validated
internal/logging/           structured JSON logging (stdlib log/slog)
internal/model/             dependency-free domain types (AnalysisRun, WorkloadRef, ...)
internal/promclient/        Prometheus client behind an interface
internal/k8sclient/         Kubernetes client behind an interface, scoped to D-007
internal/runner/            the analysis loop: timer, failure handling, in-memory snapshot
internal/httpserver/        /healthz, /readyz, /metrics, /api/runs/latest
internal/metrics/           the analyzer's own Prometheus metrics (self-observability)
```

Every external call (Prometheus, Kubernetes) sits behind a small interface, so `internal/runner`
is tested entirely with fakes — no live cluster needed for `go test`.

## Run it locally

Requires the kind cluster and monitoring stack from Phase 1 (see the top-level README), and a
port-forward to Prometheus in a separate terminal:

```bash
kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090
```

Then, from `backend/`:

```bash
CLUSTER_ID=workload-analyzer \
KUBE_CONTEXT=kind-workload-analyzer \
ANALYSIS_INTERVAL=30s \
go run ./cmd/analyzer
```

`PROMETHEUS_URL` defaults to `http://localhost:9090` (matching the port-forward above);
`LISTEN_ADDR` defaults to `127.0.0.1:8080` (loopback-only, per the threat model). See
`internal/config/config.go` for every variable and its default.

Check it's working:
```bash
curl localhost:8080/healthz              # process liveness -- always ok
curl localhost:8080/readyz               # 200 only when the last run was fully clean
curl localhost:8080/api/runs/latest      # the current snapshot as JSON
curl localhost:8080/metrics | grep ^analyzer_
```

## Test

```bash
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

All tests use fakes/the k8s.io/client-go fake clientset — none require a live cluster or
Prometheus. `internal/runner`'s tests are the direct proof of the Phase 3 done-criterion
"degrades cleanly with Prometheus down": see `TestRunOnce_PrometheusDown` and
`TestLatest_WorkloadsSeenSurvivesAFailedRun`.

## Kubernetes access

Local development uses the default kubeconfig (whatever `kubectl` itself uses), full admin
access to the kind cluster. `deploy/rbac/analyzer.yaml` defines the actual D-007-scoped
ClusterRole the backend will run under once deployed in-cluster (Phase 7) — see that file's
comments for how to try it locally ahead of time.
