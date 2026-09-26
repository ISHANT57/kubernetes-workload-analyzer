# Phases

Status values: `NOT STARTED` · `IN PROGRESS` · `WAITING FOR DECISION` · `DONE`

| # | Phase | Goal | Done when | Status |
|---|---|---|---|---|
| 0 | Foundation | Environment check, git, docs, decisions | D-001..D-007 recorded | DONE (D-004 still PROPOSED) |
| 1 | Kubernetes + monitoring | Install kubectl/kind/helm; create 1-node kind cluster; hands-on fundamentals (Pod, Deployment, ReplicaSet, Service, Namespace, ConfigMap, Secret, requests/limits, probes, labels/selectors, Events, RBAC, HPA, scheduling, failure/recovery); **install trimmed kube-prometheus-stack at the end so history starts accumulating** | Every concept has an entry in `docs/kubernetes-fundamentals.md` (experiment, observation, command); Prometheus footprint MEASURED; all metrics in the measurement model return data; KSM cannot list secrets. Time limit: ~2 weeks | DONE |
| 2 | Demo workloads | Deterministic workloads in `demo/`, each mapped to an expected result in `demo/expected-findings.yaml` | Each workload shows its intended behaviour in Prometheus | DONE |
| 3 | Backend skeleton | Go module: config, Prometheus client, Kubernetes client (D-007 scope), analysis-run object, `/healthz` `/readyz` `/metrics`, structured logs | Runs against the cluster; degrades cleanly with Prometheus down | DONE |
| 4 | Findings (R001–R005) | Evidence builder, rule engine, finding model, stable IDs, cost calculator, REST API | Output matches `expected-findings.yaml` | DONE (KRR comparison deferred) |
| 5 | Dashboard | React + Vite + TS + uPlot on recorded real output | Overview, Findings, Detail, Workloads, Platform Status pages | DONE |
| 6 | Grafana integration | Links from findings to Grafana panels | Each resource finding links to its panel | DONE |
| 7 | Production engineering | Retries, idempotency, partial-failure handling, structured errors, RBAC hardening, in-cluster deploy; tracing only if a need appears | Failure tests pass (Prometheus down, partial query failure, duplicate run, insufficient history) | DONE |
| 8 | Final demonstration | Scripted demo covering the 8 points of the brief | Demo runbook in `docs/runbook.md` | DONE |

## Log
- 2026-09-23: Environment inspected; git initialized; docs created; decisions proposed.
- 2026-09-23: Environment re-measured after owner cleanup (75 GB free, 0 containers running).
- 2026-09-24: Owner accepted D-001, D-002, D-003, D-005, D-006, D-007. Phases reordered: Prometheus installed at end of Phase 1.
- 2026-09-24: Phase 1 done. kind v1.37.0 cluster; labs 01–05; metrics-server v0.9.0; kube-prometheus-stack 91.5.0 (~580 MiB pods).
  All measurement-model metrics verified with real conditions. `container_oom_events_total` stayed 0 through real OOMs → not used.
  KSM secrets access removed and verified; Grafana auth verified. Cluster-level recovery (Docker restart / reboot) moved to Phase 7.
- 2026-09-25: 6 ADRs written (D-001,002,003,005,006,007) with Phase 1 evidence; branches cleaned up.
- 2026-09-25: Phase 2 done. 9 demo/ workloads (agnhost/resource-consumer, no new image pulls) verified live
  against Prometheus: OOMKilled/137, CrashLoopBackOff/exit 1, Pending/Insufficient cpu, HPA object present,
  duty-cycled CPU and held memory all confirmed in expected ranges. `demo/expected-findings.yaml` is the
  ground truth for the Phase 4 integration test.
- 2026-09-25: Phase 3 done. Go 1.27 module (`backend/`): config, structured logging, Prometheus client,
  Kubernetes client scoped to D-007, analysis loop with per-source timeouts and a sticky in-memory snapshot,
  `/healthz` `/readyz` `/metrics` `/api/runs/latest`. All unit tests pass with fakes (no live cluster needed);
  additionally run live against the real cluster: counted 16 deployments correctly, then Prometheus access
  was killed and restored live -- `/readyz` correctly went 503 and recovered, `workloads_seen` stayed at 16
  throughout (carried forward, never dropped to 0), `/healthz` stayed unaffected, graceful SIGTERM shutdown
  confirmed. `deploy/rbac/analyzer.yaml` written (not yet applied; in-cluster deployment is Phase 7).
- 2026-09-26: Phase 4 done. Rule engine (R001-R004; R005 is the data-quality gate embedded in
  evidence.Classify, not a separate rule): evidence builder (Prometheus-only, pod resolution via
  KSM owner-chain join -- D-007 grants no `pods` access), self-calibrated coverage (an assumed
  scrape interval was checked live and found >2x wrong, so coverage comes from actual sample
  timestamps instead), cost estimator (explicit `Estimated`, never "savings", no built-in default
  price), findings orchestrator (stable IDs, ranking), `/api/findings`. All unit tests pass with
  synthetic evidence matching all 9 demo/ scenarios; additionally run live end-to-end against the
  real cluster with real pricing configured -- output matched `demo/expected-findings.yaml`
  exactly for all 9 scenarios, plus found a genuine, non-fixture finding on
  `kube-system/metrics-server` (confirms the rules generalize beyond the demo fixtures). Verified
  idempotent (same finding IDs across independent runs) and graceful degradation with real
  pricing wired in (Prometheus killed and restored live: findings stayed sticky, `/readyz` 503'd
  and recovered, SIGTERM shutdown clean). 3 real bugs found via live verification and fixed, not
  just made to pass a test written after the fact -- see docs/requirements.md §3 implementation
  notes: a `rate()` window that silently defeated the bursty guard, a p50=0 case that inverted
  the burstiness signal, and a misleading confidence-reason message. KRR comparison deferred to a
  later phase (not blocking; no demo fixture needs it).
- 2026-09-26: Phase 5 done. React + Vite + TS dashboard (`frontend/`): Overview, Findings (filterable),
  Finding Detail (full evidence, cost, and a real uPlot usage-vs-request chart for R001/R002),
  Workloads, Platform Status. New small backend addition: GET /api/timeseries (reuses the evidence
  builder's existing Prometheus queries; fixed, parameterized -- the browser still never sends
  PromQL). Backend now also serves the built frontend directly (STATIC_DIR + SPA fallback) as one
  deployable binary. Verified live with a real backend/cluster via Playwright, not just built and
  assumed: desktop, mobile (390px), and dark mode all checked with zero console errors; one real
  mobile CSS bug (long PromQL text overflowing the evidence table) found and fixed the same way.
- 2026-09-26: Phase 6 done. Resource findings (R001/R002) and the Workloads page link to
  kube-prometheus-stack's bundled "Kubernetes / Compute Resources / Workload" Grafana dashboard,
  pre-filtered to the exact namespace/workload/type via its own template variables (confirmed via
  the real Grafana API, not guessed). Built entirely client-side (frontend/src/grafana.ts) -- no
  backend change needed, since it's a URL, not a query result. Grafana's existing login
  requirement is left as-is (no anonymous-access change), matching the threat model's
  read-only/least-privilege posture.
- 2026-09-26: Phase 7 done. `backend/Dockerfile` (new): multi-stage build, `node:24-alpine` for the
  frontend then `golang:1.27-alpine` (`CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`) onto
  `gcr.io/distroless/static-debian12:nonroot` -- 45.7 MB image, no shell, no package manager.
  `deploy/rbac/analyzer.yaml` gained a dedicated `analyzer` namespace (ServiceAccount moved out of
  `default`); D-007 ClusterRole/ClusterRoleBinding content unchanged. `deploy/analyzer/deployment.yaml`
  (new): Deployment + Service, `runAsNonRoot: true` with explicit `runAsUser`/`runAsGroup: 65532`
  (distroless's `nonroot` user is a name, not a UID -- kubelet cannot verify non-root without one,
  hit as a real `CreateContainerConfigError` first), `allowPrivilegeEscalation: false`,
  `readOnlyRootFilesystem: true`, all capabilities dropped, `seccompProfile: RuntimeDefault`.
  Resource requests/limits measured live with `kubectl top` (~1-18m CPU / 10Mi memory observed)
  then set with headroom (`25m`/`32Mi` request, `128Mi` memory limit), not guessed. Verified live,
  in-cluster, twice: (1) normal path -- pod `Running 1/1`, `/readyz` 200, `/api/findings` returns
  real findings through the ClusterIP Service, D-007 RBAC re-verified with the real in-cluster
  ServiceAccount token (`kubectl auth whoami`, isolated `KUBECONFIG=/dev/null`) confirming no
  `pods`/`secrets`/write access; (2) failure path -- Prometheus taken down by patching its own
  Operator-managed CR to `replicas: 0` (a direct StatefulSet scale-down was found to be silently
  reverted by the operator within ~52s, a real methodology bug caught by a false-negative result,
  fixed by using the CR instead) produced a genuine in-cluster DNS-path connection-refused error,
  `status: partial`, sticky findings (stayed at 3, never dropped), `/readyz` 503; restoring the CR
  to `replicas: 1` brought Prometheus back and the analyzer's next cycle returned to `status:
  complete` and `/readyz` 200 on its own, no restart needed. `docs/threat-model.md` and
  `docs/architecture.md` updated with the real in-cluster deployment shape and RBAC verification
  detail.
- 2026-09-26: Phase 8 done. `docs/runbook.md`: one section per demonstrable success criterion from
  the original brief (§24 items 1-8), each with real commands run against the actual in-cluster
  analyzer, not invented output. Writing it surfaced a real, live observation, not a fabricated
  one: right after the Phase 7 outage test, `demo/cpu-over-requested` and `demo/memory-over-requested`
  showed `data_quality: insufficient` ("less than 80% coverage over the minimum 30m window") instead
  of firing R001/R002 -- confirmed via `cmd/verify` -- because the deliberate Prometheus outage had
  just created a real gap in that window. This is R005 (the data-quality gate) working as designed,
  not a bug: it self-clears as the gap ages out of the 30-minute lookback. The runbook documents
  this live rather than papering over it, and the cost-impact section is worked out from the real
  evidence and the real deployed formula (`internal/findings/findings.go`, `CostHours = 730`)
  rather than showing a fabricated API response, per AGENTS.md's "never invent metrics" rule.
- 2026-09-26: Post-Phase-8 gap closure (not a new phase; three documentation/testing gaps found
  during an honest completion review against the original brief). `backend/cmd/loadtest/main.go`
  (new, stdlib only): concurrent GET load generator, the brief §17 "basic load test". Run live
  against the in-cluster analyzer: 5 endpoints, 20 concurrent workers, 15s each, ~212k total
  requests, zero errors. `docs/performance.md` (new): the real measured results --
  `/healthz`/`/readyz`/`/api/runs/latest`/`/api/findings` at 3-6ms p50 / 15-20ms p99;
  `/api/timeseries` at 47ms p50 / 113ms p99 (queries Prometheus live per request, by design, unlike
  the other endpoints which read the in-memory snapshot); a genuine finding that CPU usage spiked
  to 1614m under this load against a 25m request with no CPU limit set (no restart, no OOM,
  documented as a recommendation, not silently applied); and the real
  `analyzer_analysis_run_duration_seconds` histogram from `/metrics` (29 runs, mean 0.643s, all
  under 1s). `docs/reports/phase-3-report.{html,pdf}` through `phase-8-report.{html,pdf}` (new):
  the PDF write-up convention from Phases 1-2 had lapsed for Phases 3-8; backfilled from the
  already-verified facts in this file and `docs/*`, not reconstructed from memory, in the same
  template.

## MVP done-condition
See `docs/requirements.md` → MVP.
