# ADR-003: Metrics source — trimmed kube-prometheus-stack from Phase 1

## Context
Rightsizing rules (R001/R002) need historical percentiles (p95 CPU, max memory) and CPU throttling
ratios, not a single point-in-time reading. metrics-server only exposes the latest sample and
stores no history.

## Options
- **metrics-server only**: tiny footprint, but no history — rightsizing would rest on one sample,
  exactly the flaw the project set out to avoid.
- **Trimmed kube-prometheus-stack**: Prometheus + kube-state-metrics + node-exporter + Grafana,
  with Alertmanager and control-plane scrape targets disabled.
- **Plain Prometheus + kube-state-metrics (hand-rolled manifests)**: lighter, but loses the
  operator, default rules, and Grafana dashboards; more YAML to own.

## Decision
kube-prometheus-stack 91.5.0, installed via Helm with `deploy/prometheus/values.yaml`:
Alertmanager off; `kubeEtcd`/`kubeScheduler`/`kubeControllerManager`/`kubeProxy`/`kubeApiServer`
monitors off; Prometheus on an 8Gi PVC, `retention: 7d`, `retentionSize: 5GB`, memory limit 1Gi;
`kube-state-metrics.collectors` excludes `secrets`/`configmaps`; Grafana admin from an existing
Secret (`grafana-admin`), not `adminPassword` in values.

## Reasoning — measured in Phase 1 (2026-09-24)
- Install: 4 min 32 s; Prometheus Ready ~4.5 min after start.
- Footprint: ~580 MiB across 5 pods (Grafana ~400 MiB — largest single consumer; Prometheus
  85→144 MiB over 50 min; operator/KSM/node-exporter ~28/20/11 MiB).
- Ingestion: ~15.5k active series, ~600 samples/s — well under the Phase 0 estimate of ~100k
  series; disk projected at ~0.6 GB for 7 days (ESTIMATED from measured rate), comfortably under
  the 5GB cap.
- Every metric in the measurement model (docs/requirements.md §2) was verified present under real
  conditions (Lab 06), except `container_cpu_cfs_throttled_seconds_total` (intentionally dropped
  by the chart's relabeling) and `container_oom_events_total` (present but stuck at 0 through 5
  real OOMs — removed from the model; R004 uses `kube_pod_container_status_last_terminated_reason`
  instead).
- `kube-state-metrics` default collectors include `secrets`/`configmaps` with `list`/`watch` RBAC
  on them; excluding them from `collectors` removes the RBAC rules too (verified: `auth can-i` →
  no; zero `kube_secret_*`/`kube_configmap_*` series).

## Trade-offs
- Heavier than metrics-server alone; mitigated by disabling Alertmanager and control-plane
  monitors, which the analyzer does not query.
- Depends on the Prometheus Operator's CRDs, adding one moving part vs. hand-rolled manifests —
  accepted for the ready-made scrape configuration, recording rules, and Grafana dashboards.

## Consequences
- Backend queries only Prometheus for metrics (never scrapes kubelets/cAdvisor itself).
- R004 (OOM) is single-sourced from kube-state-metrics; no redundant OOM signal exists.
- Absent-series semantics differ by metric type (state metrics: "condition not present"; usage
  metrics: "missing data" → R005 `insufficient`) — documented in docs/requirements.md.

## Status
Accepted — 2026-09-24
