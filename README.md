# Kubernetes Workload Analyzer

> **Status:** Phase 0 (foundation) done. Phase 1 (kind cluster + fundamentals + monitoring) next.
> No application code yet. See [PHASES.md](PHASES.md).

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

## Planned layout

```
backend/    Go analyzer
frontend/   React dashboard
deploy/     prometheus values, rbac
demo/       deterministic test workloads + expected findings
docs/       requirements, architecture, decisions, threat model
tests/      integration and failure tests
```

## Local setup

Coming in Phase 1.

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
- [docs/DECISIONS.md](docs/DECISIONS.md) — decisions and their status
- [docs/threat-model.md](docs/threat-model.md) — threats and permissions
