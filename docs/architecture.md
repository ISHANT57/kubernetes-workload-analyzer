# Architecture (decisions accepted; Phase 3 backend skeleton built and verified)

## Local environment (re-measured 2026-09-23, 15:05; supersedes first pass)

| Item | Value | Label |
|---|---|---|
| OS | Ubuntu 26.04.1 LTS, kernel 7.0.0-31-generic, x86_64 | MEASURED |
| CPU | Intel i5-1235U: 10 physical cores (2 P-cores with HT = CPUs 0-3, 8 E-cores = CPUs 4-11), 12 logical CPUs | MEASURED |
| RAM | 14.79 GiB total; 6.97 GiB available with browser + IDE open; 4 GiB swap file (0.11 GiB used); no zram | MEASURED |
| Disk | ext4 root, 140 GB, **75 GB free (45% used)** | MEASURED |
| cgroup | v2 (`cgroup2fs`) | MEASURED |
| Docker | 29.1.3, overlayfs via **containerd image store** (images live under `/var/lib/containerd`), root `/var/lib/docker`, 0 running containers | MEASURED |
| Kubernetes tooling | none installed; no `~/.kube`; `~/.local/bin` does not exist and is not on `PATH` | MEASURED |
| Runtimes | Go 1.24.0 (`GOTOOLCHAIN=auto`), Python 3.14.4, Node 24.19.0, OpenJDK 17.0.20, GNU Make 4.4.1, jq 1.8.1, git 2.53.0 | MEASURED |
| inotify | `max_user_instances=128`, `max_user_watches=65536` (per UID; kind node processes count against **root**) | MEASURED |
| Sleep | on battery at measurement; battery idle-suspend after 6000 s; lid close suspends (logind default); 68 suspends in last 14 days | MEASURED |

Budget: about **4.5 GiB** is realistic for the whole cluster while the desktop stays in use; the
monitoring stack footprint is still an ESTIMATE until Phase 1 measures it.

### Measured cluster footprint (2026-09-24)

| Item | Value | Label |
|---|---|---|
| Cluster | kind v0.33.0, single node `workload-analyzer`, Kubernetes v1.37.0, containerd 2.3.4 | MEASURED |
| Idle cluster memory (node container, all 9 system pods Running) | ~550 MiB (547 / 548 / 551 MiB samples) | MEASURED |
| Idle cluster CPU | 21–25% of one core | MEASURED |
| Host available memory before → after | 9951 → ~9400 MiB | MEASURED |
| Create time | 2 min 49 s total; control plane Ready after 16 s (rest = first image pull) | MEASURED |
| Node image | `kindest/node:v1.37.0`, 1.34 GB on disk | MEASURED |
| Node allocatable | 12 CPU, 14.79 GiB (the whole laptop; workload memory limits protect the desktop) | MEASURED |
| Storage | `standard` StorageClass (local-path), default, `WaitForFirstConsumer` | MEASURED |
| Node container restart policy | `on-failure:1`; survival across host reboot UNVERIFIED (Phase 7 test) | MEASURED |

### Measured monitoring footprint (2026-09-24, kube-prometheus-stack 91.5.0, Helm v4.3.0)

| Item | Value | Label |
|---|---|---|
| Install time | 4 min 32 s (`helm install --wait`); Prometheus pod Ready ~4.5 min after start | MEASURED |
| Grafana pod | ~400 MiB total: grafana 246 MiB + dashboard sidecar 73 MiB + datasource sidecar 72 MiB | MEASURED |
| Prometheus | 85 MiB at start → 126 MiB after ~20 min (+10 MiB config-reloader); limit 1Gi | MEASURED |
| Operator / kube-state-metrics / node-exporter | ~28 / ~20 / ~11 MiB | MEASURED |
| Monitoring pods total | ~580 MiB (metrics-server `top`) | MEASURED |
| metrics-server | 15 MiB used (requests 200Mi) | MEASURED |
| Node container (`docker stats`) | ~770 MiB before install → ~2.2 GiB after; includes reclaimable file cache from image pulls, so it overstates the real cost | MEASURED |
| Host available memory | 7098 → 6274 MiB (−824 MiB; desktop apps also changed in between) | MEASURED |
| Active series / ingestion | 13.7k → 15.5k series; ~600 samples/s | MEASURED |
| Prometheus disk | 2.8 MB after ~20 min; ~80 MB/day projected from 600 samples/s × ~1.5 B/sample → ~0.6 GB for 7d, well under the 5GB cap | ESTIMATED |
| Scrape targets | coredns, grafana, operator, prometheus, kubelet (incl. cAdvisor), kube-state-metrics, node-exporter: all `up` | MEASURED |

## Shape: modular monolith

```
 ┌──────────────────────────── kind cluster (1 node) ─────────────────────────────┐
 │  demo ns (test workloads)   kube-state-metrics ─┐                             │
 │                             kubelet/cAdvisor ───┼──► Prometheus (PVC, 7d) ──► Grafana │
 │                             node-exporter ──────┘          │                  │
 │  Kubernetes API (structure + Events) ───────────┐          │                  │
 └──────────────────────────────────────────────────┼──────────┼──────────────────┘
                                   client-go (D-007)│          │PromQL (fixed queries)
                              ┌─────────────────────▼──────────▼──────────────┐
                              │ Go backend (single process)                   │
                              │  source/k8s   read-only discovery + Events    │
                              │  source/prom  fixed queries, timeouts         │
                              │  evidence     builds evidence per workload    │
                              │  rules        R001–R005, pure functions       │
                              │  cost         pricing interface               │
                              │  findings     rank, stable IDs                │
                              │  runner       analysis runs → snapshot        │
                              │  api          JSON: /api/findings, /api/runs  │
                              │  web          serves built React files        │
                              │  ops          /healthz /readyz /metrics, logs │
                              └───────────────────────▲───────────────────────┘
                                                      │ localhost only
                                           React + Vite + TS + uPlot (browser)
```

### Why a monolith
There is one data source, one user and one cluster. Separate collector/analyzer/API services would
add deployment, networking and failure modes without solving a real problem. The module
boundaries (`source`, `rules`, `cost`, `findings`, `web`) keep a later split possible.

**Phase 3 built and verified** (`backend/`, see backend/README.md): `promclient`, `k8sclient`,
`runner`, `httpserver`, `config`, `logging`, `metrics`, `model` — the sketch's `source/prom`,
`source/k8s`, `runner`, `api`+`ops` respectively, under slightly different package names; `web`
(serving the built React app) is added when the frontend exists (Phase 5), and `evidence`,
`rules`, `cost`, `findings` are added in Phase 4. Verified end-to-end against the live cluster,
including killing and restoring the Prometheus port-forward mid-run (see PHASES.md's Phase 3 log
entry and `docs/reports/` once its report is published).

### Analysis loop (small design choice, open to change)
An in-process ticker (for example every 5 min) runs queries and rules, then keeps the latest
**snapshot** in memory. API requests read the snapshot. This keeps page latency independent of
PromQL cost. Each run produces an analysis-run object (`complete | partial | failed`); a failed run
keeps the previous snapshot, marked stale. No queue, no worker process, no database.

### Kubernetes API access (D-007)
Read-only ClusterRole (`get/list/watch`) on namespaces, nodes, deployments, replicasets,
statefulsets, daemonsets, HPAs and events. No pods, secrets or configmaps, so the backend never sees
pod specs or env vars. All metrics (usage, requests, restarts, OOM) come from Prometheus.

**Workload discovery covers all three workload kinds** (`internal/k8sclient`'s `ListWorkloads`,
PR18): Deployment, StatefulSet and DaemonSet, each contributing to `AnalysisRun.WorkloadsSeen`
and going through the same rule engine. Pod resolution (Prometheus-only, no `pods` access) uses a
different owner chain per Kind, since the two-hop Deployment path structurally cannot work for
the other two: a Deployment owns pods through an intermediate ReplicaSet
(`kube_pod_owner`→`kube_replicaset_owner` join), while a StatefulSet or DaemonSet owns its pods
directly (`kube_pod_owner` already reports the right owner straight on the pod, one query, no
join). Verified live against real objects, not just demo fixtures: the monitoring namespace's own
Prometheus (a StatefulSet, deployed by the Prometheus Operator) and its node-exporter and
kube-system's kube-proxy (DaemonSets) all resolve to real pods, containers and usage data through
this path.

### Where it runs
During development the backend runs on the host and reaches Prometheus via `kubectl port-forward`.
It also runs in-cluster (Phase 7, `deploy/analyzer/`): a distroless multi-stage image
(`backend/Dockerfile`, ~46 MB, non-root, read-only root filesystem, no shell), served as one
Deployment + Service in its own `analyzer` namespace, using its own ServiceAccount (D-007) with
no kubeconfig at all -- `rest.InClusterConfig()` picks up the mounted token automatically.
Prometheus is reached via its in-cluster Service DNS name instead of a port-forward. Verified
live: RBAC (see threat-model.md), Service routing, and real measured resource usage (idle ~1m
CPU / 10Mi memory, peaks to ~18m CPU during an analysis run) used to set the Deployment's own
requests -- the analyzer's own advice applied to itself.

### Failure behaviour (summary)
| Failure | Detection | Behaviour |
|---|---|---|
| Prometheus down or timing out | query error, per-query timeout | keep last snapshot marked stale; `/readyz` fails; error counter increments |
| Series missing for a workload | empty result | finding suppressed, reported as "insufficient data", never guessed |
| Low data coverage | coverage < threshold | confidence lowered or rule skipped |
| Pod disappears mid-run | series ends | uses data up to disappearance; no crash |
| Analysis runs twice | — | rules are pure; stable finding IDs, so findings are updated, not duplicated |
| Kubernetes API unreachable or `403 Forbidden` | client error | run marked `partial`; findings still built from Prometheus; error logged and counted |
| One query fails | per-query error | only that workload gets `query_error`; the rest of the run continues |
| Browser tries to reach Prometheus | — | not possible: the API returns only findings JSON; Prometheus is ClusterIP-only |

## Dependency classification

| Dependency | Class | Status |
|---|---|---|
| Docker Engine | FREE LOCAL | installed |
| kind v0.33.0 (single node) | FREE LOCAL | not installed; D-005 ACCEPTED |
| kubectl v1.37.0, Helm v4.3.0 | FREE LOCAL | not installed; Phase 1 |
| kube-prometheus-stack 91.5.0 (Prometheus, Operator, KSM, node-exporter, Grafana) | FREE SELF-HOSTED | D-003 ACCEPTED |
| Grafana (AGPL-3.0; free to self-host) | FREE SELF-HOSTED | D-002/D-003 ACCEPTED |
| Go 1.27 toolchain; client-go, Prometheus client_golang | FREE LOCAL | D-001 ACCEPTED |
| Node 24 + React, Vite, TypeScript, uPlot (build time only) | FREE LOCAL | D-002 ACCEPTED |
| metrics-server (only so the HPA can scale in Phase 1) | FREE SELF-HOSTED | optional; ask first |
| Docker Hub pulls (kindest/node, busybox) | FREE-TIER: anonymous pull rate limits | pull once, then `kind load` |
| PostgreSQL | FREE SELF-HOSTED | D-004 (proposed: not in MVP) |
| stress-ng / busybox images for demo workloads | FREE LOCAL (public images) | Phase 2 |
| `golang:1.27-alpine`, `node:24-alpine`, `gcr.io/distroless/static-debian12:nonroot` (Docker build stages) | FREE LOCAL (public images) | Phase 7; built and verified locally, `kind load docker-image` (no registry needed) |
| GitHub Actions (later CI) | FREE-TIER: free minutes for public repos; limited minutes for private repos | Phase 7; optional |
| OpenCost, KRR | FREE SELF-HOSTED / FREE LOCAL | reference and comparison only |

## Multi-cluster (D-008)
One analyzer instance per cluster, unchanged from v1's single-cluster logic. Each instance's
`GET /api/clusters` returns its own `ClusterID` plus any peers from `PEER_CLUSTERS`
(`id=url` pairs); the dashboard's top bar renders a switcher that navigates to a peer's URL. This
is link metadata only -- no analyzer instance ever calls another one over the network, so D-007's
read-only scope and each cluster's trust boundary are independent and unaffected. A combined,
single ranked list merged across clusters is deliberately not built (no aggregator exists) until
there's a concrete need for it.

## Not yet designed (on purpose)
Authentication, a combined multi-cluster findings list, persistent finding history, alerting, CI
and deployment packaging.
