# backend

Go analyzer: Kubernetes discovery, Prometheus queries, the rule engine (R001–R004; R005 is the
data-quality gate embedded in `internal/evidence`), cost estimation, and the JSON API the
`frontend/` dashboard consumes.

## Layout

```
cmd/analyzer/main.go        wiring only: config -> clients -> analyzer -> runner -> http server
cmd/verify/main.go          manual debug tool: prints raw evidence for demo/ fixtures
internal/config/            env-var configuration, validated
internal/logging/           structured JSON logging (stdlib log/slog)
internal/model/             dependency-free domain types (AnalysisRun, Finding, WorkloadRef, ...)
internal/promclient/        Prometheus client behind an interface (instant + range queries)
internal/k8sclient/         Kubernetes client behind an interface, scoped to D-007
internal/evidence/          builds WorkloadEvidence from Prometheus only (no `pods` access);
                             self-calibrated data-quality/confidence tiering
internal/rules/             R001-R004 as pure functions over WorkloadEvidence
internal/cost/              estimated cost impact -- explicit "Estimated", never "savings"
internal/findings/          orchestrates evidence+rules+cost into a ranked, stably-ID'd list
internal/runner/            the analysis loop: timer, failure handling, in-memory snapshot
internal/httpserver/        /healthz /readyz /metrics /api/findings /api/runs/latest
                             /api/timeseries, and (if STATIC_DIR is set) the built frontend
internal/metrics/           the analyzer's own Prometheus metrics (self-observability)
```

Every external call (Prometheus, Kubernetes) sits behind a small interface, so everything except
`cmd/*` is tested entirely with fakes/synthetic evidence — no live cluster needed for `go test`.

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
CPU_CORE_HOUR_USD=0.0316 MEMORY_GIB_HOUR_USD=0.0042 PRICE_SOURCE="your source here" \
go run ./cmd/analyzer
```

`PROMETHEUS_URL` defaults to `http://localhost:9090` (matching the port-forward above);
`LISTEN_ADDR` defaults to `127.0.0.1:8080` (loopback-only, per the threat model).
`CPU_CORE_HOUR_USD`/`MEMORY_GIB_HOUR_USD` are optional: findings work without them, just with no
`cost` field -- there is no built-in default price (docs/requirements.md §4: a default would
imply a real cloud price that isn't yours). See `internal/config/config.go` for every variable.

**To also serve the dashboard from this same binary** (see `frontend/README.md` for building it):
```bash
STATIC_DIR=$(pwd)/../frontend/dist go run ./cmd/analyzer
# then open http://localhost:8080/
```
Without `STATIC_DIR`, the backend is still fully usable as an API-only server -- useful during
frontend development, where `npm run dev` proxies to it instead (frontend/vite.config.ts).

Check it's working:
```bash
curl localhost:8080/healthz              # process liveness -- always ok
curl localhost:8080/readyz               # 200 only when the last run was fully clean
curl localhost:8080/api/findings         # the ranked finding list
curl 'localhost:8080/api/timeseries?namespace=demo&workload=X&container=c&metric=cpu&window=24h'
curl localhost:8080/metrics | grep ^analyzer_
```

## Test

```bash
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

All tests use fakes or synthetic `evidence.WorkloadEvidence` -- none require a live cluster or
Prometheus. Notable ones:
- `internal/runner`: `TestRunOnce_PrometheusDown`, `TestLatest_WorkloadsSeenSurvivesAFailedRun`
  -- the "degrades cleanly" behavior, both live-verified against the real cluster too.
- `internal/rules`: one fixture per `demo/` scenario, matching `demo/expected-findings.yaml`.
- `internal/evidence`: `TestBurstiness_ZeroMedianWithRealTail_IsInfinite` -- a regression test
  for a real bug caught live (see docs/requirements.md §3 implementation notes).

## Kubernetes access

Local development uses the default kubeconfig (whatever `kubectl` itself uses), full admin
access to the kind cluster. In-cluster (below), it uses only the D-007-scoped ServiceAccount.

## Deploy in-cluster (Phase 7)

```bash
# from the repo root -- builds the frontend too (backend/Dockerfile is multi-stage)
docker build -f backend/Dockerfile -t k8s-workload-analyzer:local .
kind load docker-image k8s-workload-analyzer:local --name workload-analyzer

kubectl apply -f deploy/rbac/analyzer.yaml         # namespace, ServiceAccount, D-007 ClusterRole
kubectl apply -f deploy/analyzer/deployment.yaml   # Deployment + Service
kubectl -n analyzer rollout status deploy/analyzer

kubectl -n analyzer port-forward svc/analyzer 8080:8080   # or open through an Ingress later
```

No kubeconfig is mounted -- `rest.InClusterConfig()` picks up the ServiceAccount token
automatically. `PROMETHEUS_URL` points at the in-cluster Prometheus Service DNS name instead of
`localhost` (see `deploy/analyzer/deployment.yaml`). Verified live: pod reaches `Running 1/1`,
`/readyz` returns 200, `/api/findings` returns real findings through the Service, and the D-007
RBAC restriction holds for a real token (see `docs/threat-model.md`).

**Known gotcha, hit and fixed:** distroless's `nonroot` image sets `USER nonroot` (a name), which
the kubelet cannot resolve to a UID to satisfy `runAsNonRoot: true` without an explicit numeric
`runAsUser`/`runAsGroup: 65532` in the pod's securityContext -- omitting it fails with
`CreateContainerConfigError: cannot verify user is non-root`.

Rollback: `kubectl delete -f deploy/analyzer/deployment.yaml -f deploy/rbac/analyzer.yaml`.
