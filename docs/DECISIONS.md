# Decisions

Status values: **PROPOSED** (recommendation, not decided) · **ACCEPTED** (owner decided) ·
**SUPERSEDED**. Accepted decisions get a full ADR in `docs/decisions/ADR-NNN-*.md` after Phase 1,
so the ADRs can cite measured evidence. Written 2026-09-25: [ADR-001](decisions/ADR-001-backend-language.md),
[ADR-002](decisions/ADR-002-dashboard.md), [ADR-003](decisions/ADR-003-metrics-source.md),
[ADR-005](decisions/ADR-005-local-cluster.md), [ADR-006](decisions/ADR-006-positioning.md),
[ADR-007](decisions/ADR-007-k8s-api-scope.md). ADR-004 is deferred until D-004 is accepted.

| ID | Decision | Choice | Status |
|---|---|---|---|
| D-001 | Backend language | Go (toolchain 1.27) | ACCEPTED 2026-09-24 |
| D-002 | Dashboard approach | React + Vite + TypeScript + uPlot, built to static files served by the Go backend; Grafana for raw time series | ACCEPTED 2026-09-24 |
| D-003 | Prometheus from MVP | Trimmed kube-prometheus-stack (no Alertmanager, control-plane monitors off, 8Gi PVC, 7d / 5GB retention, 1Gi memory limit, KSM without `secrets`/`configmaps`) | ACCEPTED 2026-09-24 |
| D-004 | PostgreSQL in MVP | No application database in MVP | PROPOSED |
| D-005 | Local cluster tool | kind, single node; switch to k3d if available RAM < 1.5 GiB after the monitoring stack is installed | ACCEPTED 2026-09-24 |
| D-006 | Project positioning | Portfolio/demo project: unified, explainable findings; KRR and OpenCost used as comparison baselines | ACCEPTED 2026-09-24 |
| D-007 | Backend Kubernetes API access | ClusterRole `get/list/watch` on namespaces, nodes, deployments, replicasets, statefulsets, daemonsets, HPAs, events. No pods, secrets or configmaps. Metrics come from Prometheus only | ACCEPTED 2026-09-24 |
| D-008 | Multi-cluster (2+ clusters) | One analyzer instance per cluster (unchanged, no code split); dashboard gets a client-side cluster switcher (`GET /api/clusters`, link metadata only, no server-to-server calls) | ACCEPTED 2026-09-26 |

---

## D-001 Backend language  
*Full ADR: [decisions/ADR-001-backend-language.md](decisions/ADR-001-backend-language.md)*
**Problem:** Choose the language for the monolith (Prometheus client, Kubernetes client, rules, cost, HTTP API).

| Option | For | Against |
|---|---|---|
| **Go** | Native Kubernetes ecosystem (client-go, official Prometheus API and instrumentation clients); single static binary; small image and low memory; strong signal for platform-engineering roles | More verbose data wrangling; installed Go 1.24.0 is out of support (1.26/1.27 are supported) |
| Python | Fastest to write analysis logic; 3.14 installed | Higher memory per process; weaker "infra tooling" signal |
| TypeScript (Node) | Same language as the UI | Weakest K8s/Prometheus ecosystem of the three |

**Decision:** Go. client-go `master` requires `go 1.27.0`; with `GOTOOLCHAIN=auto` the toolchain is
fetched automatically, but installing 1.27 directly avoids confusion. · **Status: ACCEPTED 2026-09-24**

## D-002 Dashboard approach  
*Full ADR: [decisions/ADR-002-dashboard.md](decisions/ADR-002-dashboard.md)*
**Problem:** The UI must answer "what is wrong?" first; trend charts are secondary. The project is also a portfolio piece, so a custom UI has value of its own.

| Option | For | Against |
|---|---|---|
| Grafana only | Zero UI code | Bad at ranked, explained findings; not "a dashboard you built" |
| Server-rendered HTML (+ htmx) | One toolchain, least code | Weaker frontend signal for a portfolio |
| **React + Vite + TypeScript SPA, served as static files by the backend** | Strong portfolio signal; still one deployable (no Node server at runtime); consumes the same JSON API | Second toolchain (Node, used at build time only); API contract must be kept in sync |

**Decision:** React + Vite + TypeScript, uPlot for the few charts that answer a question (usage vs
request with p95/max; restart/OOM timeline). Grafana remains for raw time-series investigation. The
browser never talks to Prometheus. Built only after the findings contract and real backend output
are stable. · **Status: ACCEPTED 2026-09-24**

## D-003 Prometheus from MVP  
*Full ADR: [decisions/ADR-003-metrics-source.md](decisions/ADR-003-metrics-source.md)*
**Problem:** Rightsizing needs history. metrics-server keeps only the latest sample and stores nothing.

| Option | For | Against |
|---|---|---|
| metrics-server only | Tiny footprint | No history, so no percentiles, no throttling, no OOM history |
| **kube-prometheus-stack (trimmed)** | History, PromQL, KSM, cAdvisor, Grafana, recording rules; industry standard | Heavier (footprint measured in Phase 1); needs a PVC |
| Plain Prometheus + KSM | Lighter; every manifest understood | More YAML to own; no ready-made rules or dashboards |

**Decision:** Trimmed kube-prometheus-stack: Alertmanager off; etcd/scheduler/controller-manager/
kube-proxy/API-server monitors off; Prometheus on an 8Gi PVC with `retention: 7d`,
`retentionSize: 5GB`, memory limit 1Gi; KSM `collectors` without `secrets` and `configmaps`; Grafana
admin from an existing Secret. Footprint is measured after install. · **Status: ACCEPTED 2026-09-24**

## D-004 PostgreSQL in MVP
**Question:** What data cannot live in Prometheus, Kubernetes or a config file?

| Data | Where it can live |
|---|---|
| Time series (usage, state) | Prometheus |
| Pricing, thresholds, rule profiles | Config file / env vars (versioned in git) |
| Current findings and latest analysis run | Recomputed deterministically each run; in-memory snapshot |
| Finding history, acknowledgements, suppressions, users | **Needs a DB**, but none of these are in the MVP |

Options: none · PostgreSQL · SQLite file.
**Recommendation:** No application database in MVP. Add PostgreSQL when a concrete feature (history,
acknowledgements, auth) needs it. · **Status: PROPOSED**

## D-005 Local cluster tool  
*Full ADR: [decisions/ADR-005-local-cluster.md](decisions/ADR-005-local-cluster.md)*
**Problem:** Choose a local cluster for a laptop with ~7 GiB available RAM (measured) and a ~4.5 GiB cluster budget.

| Criterion | kind | k3d | minikube |
|---|---|---|---|
| Memory | Moderate (kubeadm control plane) | Lowest | Moderate–high |
| K8s compatibility | Upstream kubeadm; used by Kubernetes' own CI | Conformant; bundles Traefik/ServiceLB | Upstream |
| Local images | `kind load docker-image` | `k3d image import` | `minikube image load` |
| Multi-node | Yes, but hits inotify limits at 128 instances | Yes | Less mature |
| CI reuse | Standard in GitHub Actions | Used | Less common |

**Decision:** kind, single node. Multi-node is not planned: it costs RAM and hits the measured inotify
limit; scheduling is demonstrated on one node with taints, nodeSelector and a Pending workload.
Switch to k3d if available RAM drops below 1.5 GiB after the monitoring stack is installed.
· **Status: ACCEPTED 2026-09-24**

## D-006 Project positioning  
*Full ADR: [decisions/ADR-006-positioning.md](decisions/ADR-006-positioning.md)*
**Problem:** OpenCost, KRR, VPA/Goldilocks and Grafana already cover pieces of this. The README must say why this project exists.

| Option | Pitch | Risk |
|---|---|---|
| Learning / reimplementation | "I rebuilt a slice of OpenCost+KRR" | Reads as a copy without depth |
| **Unified findings + explanations (portfolio/demo)** | One ranked list across health, rightsizing and estimated cost; each finding shows evidence, confidence and caveats | Must genuinely explain better than the tools it borrows ideas from |
| Narrower gap (e.g. HPA-aware rightsizing only) | Deep on one gap | Smaller demo surface |

**Decision:** Portfolio and company/college demonstration project ("Kubernetes Workload Analyzer")
built around unified, explainable findings. KRR and OpenCost are comparison baselines; the README
shows where results agree and differ. · **Status: ACCEPTED 2026-09-24**

## D-007 Backend Kubernetes API access  
*Full ADR: [decisions/ADR-007-k8s-api-scope.md](decisions/ADR-007-k8s-api-scope.md)*
**Problem:** Metrics come from Prometheus, but the backend also needs workload structure (what owns
what, HPA targets) and Kubernetes Events, which are not in Prometheus (Events are API objects that
expire after about an hour by default). The portfolio should also show real API and RBAC use. Pod
specs are sensitive: they contain literal env values, which often hold secrets.

| Option | For | Against |
|---|---|---|
| No API access (Prometheus only) | Smallest attack surface | No Events; no RBAC/API demonstration; structure only via KSM |
| **Structure + Events** | Real client-go and RBAC use; Events available; never sees pod specs | Needs a ClusterRole (cluster-scoped nodes, cross-namespace discovery) |
| Structure + Events + pods | Full detail | Exposes env vars, args and last-applied manifests |

**Decision:** Dedicated ServiceAccount bound by a ClusterRole + ClusterRoleBinding with
`get/list/watch` on `namespaces, nodes, deployments, replicasets, statefulsets, daemonsets,
horizontalpodautoscalers, events`. No `pods`, `secrets`, `configmaps`, `pods/log`, `pods/exec`, and
no write verbs. Pod-level facts (restarts, OOM, requests) come from KSM via Prometheus. Verified with
`kubectl auth can-i` checks. · **Status: ACCEPTED 2026-09-24**

## D-008 Multi-cluster (2+ clusters)
**Problem:** v1 monitors one cluster. A real need arose to show 2+ clusters monitored at once for
a demo. `docs/architecture.md` had left multi-cluster "not yet designed (on purpose)" -- every core
type already carries `ClusterID` (AGENTS.md), but no aggregation or UI existed.

| Option | For | Against |
|---|---|---|
| **One analyzer per cluster + client-side switcher (chosen)** | Zero change to the proven single-cluster analyzer's logic; each instance still only ever talks to its own Prometheus/Kubernetes API (D-007 untouched); switcher is a link, not a proxy, so no new trust boundary | No single combined ranked list across clusters yet -- switching is a full page navigation, not a merged view |
| Combined single list (one aggregator merges N analyzers' `/api/findings`) | One ranked list across all clusters -- closer to "true" multi-cluster monitoring | New aggregator service/logic, new failure mode (aggregator down != any cluster down), more code for a demo need |
| Single analyzer process holds N Kubernetes/Prometheus clients | One deployment to operate | Bigger blast radius on crash; N sets of credentials/config in one place; against "small modules" |

**Decision:** One analyzer instance per cluster (already reusable as-is, no code change to the
core logic); a new `GET /api/clusters` endpoint returns this instance's `ClusterID` plus a
`PEER_CLUSTERS`-configured list of other clusters' URLs; the dashboard's top bar shows a switcher
that navigates to a peer's own URL. This is link metadata only -- no analyzer ever calls another
analyzer, so D-007's read-only/least-privilege posture and each cluster's trust boundary are
unaffected. Combined single-list aggregation is deferred until there's a concrete need for it
(same "don't build future stages without a reason" principle as the rest of v1).
**Status: ACCEPTED 2026-09-26**
