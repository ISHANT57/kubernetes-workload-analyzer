# Phases

Status values: `NOT STARTED` · `IN PROGRESS` · `WAITING FOR DECISION` · `DONE`

| # | Phase | Goal | Done when | Status |
|---|---|---|---|---|
| 0 | Foundation | Environment check, git, docs, decisions | D-001..D-007 recorded | DONE (D-004 still PROPOSED) |
| 1 | Kubernetes + monitoring | Install kubectl/kind/helm; create 1-node kind cluster; hands-on fundamentals (Pod, Deployment, ReplicaSet, Service, Namespace, ConfigMap, Secret, requests/limits, probes, labels/selectors, Events, RBAC, HPA, scheduling, failure/recovery); **install trimmed kube-prometheus-stack at the end so history starts accumulating** | Every concept has an entry in `docs/kubernetes-fundamentals.md` (experiment, observation, command); Prometheus footprint MEASURED; all metrics in the measurement model return data; KSM cannot list secrets. Time limit: ~2 weeks | DONE |
| 2 | Demo workloads | Deterministic workloads in `demo/`, each mapped to an expected result in `demo/expected-findings.yaml` | Each workload shows its intended behaviour in Prometheus | DONE |
| 3 | Backend skeleton | Go module: config, Prometheus client, Kubernetes client (D-007 scope), analysis-run object, `/healthz` `/readyz` `/metrics`, structured logs | Runs against the cluster; degrades cleanly with Prometheus down | DONE |
| 4 | Findings (R001–R005) | Evidence builder, rule engine, finding model, stable IDs, cost calculator, REST API | Output matches `expected-findings.yaml` | DONE (KRR comparison deferred) |
| 5 | Dashboard | React + Vite + TS + uPlot on recorded real output | Overview, Findings, Detail, Workloads, Platform Status pages | NOT STARTED |
| 6 | Grafana integration | Links from findings to Grafana panels | Each resource finding links to its panel | NOT STARTED |
| 7 | Production engineering | Retries, idempotency, partial-failure handling, structured errors, RBAC hardening, in-cluster deploy; tracing only if a need appears | Failure tests pass (Prometheus down, partial query failure, duplicate run, insufficient history) | NOT STARTED |
| 8 | Final demonstration | Scripted demo covering the 8 points of the brief | Demo runbook in `docs/runbook.md` | NOT STARTED |

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

## MVP done-condition
See `docs/requirements.md` → MVP.
