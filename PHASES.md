# Phases

Status values: `NOT STARTED` · `IN PROGRESS` · `WAITING FOR DECISION` · `DONE`

| # | Phase | Goal | Done when | Status |
|---|---|---|---|---|
| 0 | Foundation | Environment check, git, docs, decisions | D-001..D-007 recorded | DONE (D-004 still PROPOSED) |
| 1 | Kubernetes + monitoring | Install kubectl/kind/helm; create 1-node kind cluster; hands-on fundamentals (Pod, Deployment, ReplicaSet, Service, Namespace, ConfigMap, Secret, requests/limits, probes, labels/selectors, Events, RBAC, HPA, scheduling, failure/recovery); **install trimmed kube-prometheus-stack at the end so history starts accumulating** | Every concept has an entry in `docs/kubernetes-fundamentals.md` (experiment, observation, command); Prometheus footprint MEASURED; all metrics in the measurement model return data; KSM cannot list secrets. Time limit: ~2 weeks | NOT STARTED |
| 2 | Demo workloads | Deterministic workloads in `demo/`, each mapped to an expected result in `demo/expected-findings.yaml` | Each workload shows its intended behaviour in Prometheus; images pinned by digest | NOT STARTED |
| 3 | Backend skeleton | Go module: config, Prometheus client, Kubernetes client (D-007 scope), analysis-run object, `/healthz` `/readyz` `/metrics`, structured logs | Runs against the cluster; degrades cleanly with Prometheus down | NOT STARTED |
| 4 | Findings (R001–R005) | Evidence builder, rule engine, finding model, stable IDs, cost calculator, REST API | Output matches `expected-findings.yaml`; compared against KRR | NOT STARTED |
| 5 | Dashboard | React + Vite + TS + uPlot on recorded real output | Overview, Findings, Detail, Workloads, Platform Status pages | NOT STARTED |
| 6 | Grafana integration | Links from findings to Grafana panels | Each resource finding links to its panel | NOT STARTED |
| 7 | Production engineering | Retries, idempotency, partial-failure handling, structured errors, RBAC hardening, in-cluster deploy; tracing only if a need appears | Failure tests pass (Prometheus down, partial query failure, duplicate run, insufficient history) | NOT STARTED |
| 8 | Final demonstration | Scripted demo covering the 8 points of the brief | Demo runbook in `docs/runbook.md` | NOT STARTED |

## Log
- 2026-09-23: Environment inspected; git initialized; docs created; decisions proposed.
- 2026-09-23: Environment re-measured after owner cleanup (75 GB free, 0 containers running).
- 2026-09-24: Owner accepted D-001, D-002, D-003, D-005, D-006, D-007. Phases reordered: Prometheus installed at end of Phase 1.

## MVP done-condition
See `docs/requirements.md` → MVP.
