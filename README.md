# Kubernetes Workload Analyzer

> **Status:** Phases 0–8 done: foundation through a hardened, in-cluster-deployed backend with a
> working React dashboard, Grafana links, verified read-only RBAC, and a scripted demo runbook —
> end-to-end, live against a real cluster. See [PHASES.md](PHASES.md) and
> [docs/runbook.md](docs/runbook.md).

A portfolio project that observes a Kubernetes cluster, analyzes real workload, resource and
reliability metrics, and turns them into **evidence-based findings** shown in a custom dashboard.

```
Kubernetes → Prometheus → Go analyzer → findings (JSON API) → React dashboard
```

## What it answers

- Is anything broken? (restarts, OOM kills)
- Which workloads request far more CPU/memory than they use over time?
- What is the **estimated** cost of current vs. rightsized allocation, under stated assumptions?
- Is there enough data to say so? (Missing data is reported as missing, never as zero.)

Each finding shows its evidence, threshold, time window, data coverage, confidence and caveats.

## Why build this if OpenCost / KRR / Grafana already exist?

They exist and are good. This project does not claim to replace them.

| Tool | What it already does | Relevance here |
|---|---|---|
| **kube-prometheus-stack** | Helm chart bundling Prometheus Operator, Prometheus, Alertmanager, Grafana, node-exporter, kube-state-metrics, default dashboards and rules | The data layer this project builds on |
| **Grafana** | Dashboards over Prometheus time series | Kept for raw time-series investigation; weak at "ranked list of what is wrong, and why" |
| **OpenCost** (CNCF) | Cost allocation per namespace/workload from Prometheus data plus cloud or custom pricing | Reference for the cost model; comparison baseline |
| **Robusta KRR** | CLI that recommends CPU/memory requests from Prometheus history | Reference for rightsizing; comparison baseline |
| **VPA recommender / Goldilocks** | Usage histograms and request recommendations via VPA objects | Needs VPA objects created in the cluster (write access); this project is read-only |

This project's angle: one ranked list across health, rightsizing and estimated cost, where every
finding explains itself. KRR and OpenCost are used as comparison baselines, and the results
document where they agree and differ.

## Stack (all free, runs locally)

| Layer | Choice |
|---|---|
| Cluster | kind, single node |
| Metrics | kube-prometheus-stack (Prometheus, kube-state-metrics, node-exporter, Grafana) |
| Backend | Go: client-go (read-only), Prometheus client, rule engine, REST API |
| Frontend | React + Vite + TypeScript + uPlot |
| Security | Read-only ClusterRole; no access to pods, secrets or configmaps |

Why each choice was made: [docs/DECISIONS.md](docs/DECISIONS.md).

## Layout

```
backend/    Go analyzer: evidence, rule engine, cost estimator, JSON API (backend/README.md)
frontend/   React + Vite + TS + uPlot dashboard (frontend/README.md)
deploy/     prometheus values, metrics-server, rbac
demo/       deterministic test workloads + expected findings
docs/       requirements, architecture, decisions, threat model
```

## Local setup

Requires Docker, [kind](https://kind.sigs.k8s.io/) v0.33+, kubectl, Helm v4. Tested on Ubuntu 26.04
with kind v0.33.0 / Kubernetes v1.37.0.

```bash
kind create cluster --name workload-analyzer --wait 180s
kubectl apply -k deploy/metrics-server            # HPA support (kind-only TLS flag)
kubectl create namespace monitoring
kubectl -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin --from-literal=admin-password="$(openssl rand -base64 24)"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kps prometheus-community/kube-prometheus-stack --version 91.5.0 \
  -n monitoring -f deploy/prometheus/values.yaml --wait --timeout 10m
kubectl -n monitoring port-forward svc/kps-grafana 3000:80   # Grafana on localhost only
```

Optionally apply the demo workloads (Phase 2) so there is something interesting to look at:
```bash
kubectl apply -k demo/
```

Build the dashboard and run the backend serving it, all as one binary (see
[backend/README.md](backend/README.md) and [frontend/README.md](frontend/README.md) for details):
```bash
kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090 &
(cd frontend && npm install && npm run build)
cd backend && CLUSTER_ID=workload-analyzer KUBE_CONTEXT=kind-workload-analyzer \
  CPU_CORE_HOUR_USD=0.0316 MEMORY_GIB_HOUR_USD=0.0042 PRICE_SOURCE="example only" \
  STATIC_DIR=$(pwd)/../frontend/dist go run ./cmd/analyzer
# open http://localhost:8080/
```

Measured footprint and verification results: [docs/architecture.md](docs/architecture.md).
Hands-on lab notes: [docs/kubernetes-fundamentals.md](docs/kubernetes-fundamentals.md).
Phase write-ups (PDF): [docs/reports/](docs/reports/).

## Limitations (known now)

- Cost figures are **estimates** from configurable unit prices, not a cloud invoice.
- Reducing a pod's request does **not** by itself reduce a cloud bill. Savings happen only if the
  freed capacity lets nodes be removed or avoided. See [docs/requirements.md](docs/requirements.md#4-cost-model).
- Single cluster in v1; the data model carries a `ClusterID` so multi-cluster stays possible.
- A laptop cluster that sleeps has gaps in history; findings report data coverage.

## Documents

- [PHASES.md](PHASES.md) — phases and progress
- [AGENTS.md](AGENTS.md) — rules for AI-assisted development
- [docs/requirements.md](docs/requirements.md) — MVP, metrics, rules, cost model, demo workloads, findings contract
- [docs/architecture.md](docs/architecture.md) — architecture, failure behaviour, dependencies
- [docs/DECISIONS.md](docs/DECISIONS.md) — decisions and their status, linking to [docs/decisions/](docs/decisions/) ADRs
- [docs/threat-model.md](docs/threat-model.md) — threats and permissions
- [docs/performance.md](docs/performance.md) — measured API latency, load test, resource usage under load
- [docs/kubernetes-fundamentals.md](docs/kubernetes-fundamentals.md) — hands-on lab notes (Phase 1)
- [docs/runbook.md](docs/runbook.md) — scripted demo, one section per success criterion (Phase 8)
- [backend/README.md](backend/README.md) — running and testing the Go backend (Phase 3)
- [docs/reports/](docs/reports/) — per-phase PDF write-ups
