# ADR-007: Backend Kubernetes API access scope

## Context
Metrics come from Prometheus (ADR-003), but the backend also needs workload structure (ownership,
HPA targets) and Kubernetes Events, which are not exported to Prometheus (Events are API objects
that expire after ~1 hour by default and are not scraped by kube-state-metrics). Pod specs are
sensitive: Phase 1 Lab 04 demonstrated that reading a Pod exposes literal environment-variable
values in plain text (`LITERAL_PASSWORD=visible-in-pod-spec`), which is how secrets commonly leak
even without direct Secret access.

## Options
- **No API access (Prometheus only)**: smallest attack surface, but no Events access, and no
  RBAC/client-go demonstration for the portfolio.
- **Structure + Events**: `get/list/watch` on namespaces, nodes, deployments, replicasets,
  statefulsets, daemonsets, HPAs, events. Never touches `pods`.
- **Structure + Events + pods**: full detail, but exposes env vars/args/last-applied-config.

## Decision
Dedicated ServiceAccount bound by a ClusterRole + ClusterRoleBinding with `get/list/watch` on
`namespaces, nodes, deployments, replicasets, statefulsets, daemonsets, horizontalpodautoscalers,
events`. No `pods`, `secrets`, `configmaps`, `pods/log`, `pods/exec`, and no write verbs anywhere.
Pod-level facts (restarts, OOM reason, requests/limits) come from kube-state-metrics via
Prometheus, never directly from the Kubernetes API.

## Reasoning — verified in Phase 1 Lab 04 (2026-09-24)
The exact ClusterRole was deployed as `analyzer-readonly` and tested two ways:
- `kubectl auth can-i --as=<sa>`: `list deployments -A` yes, `list events -A` yes,
  `watch horizontalpodautoscalers -A` yes, `list nodes` yes; `list pods -A` no, `get secrets` no,
  `list configmaps` no, `delete deployments` no, `create pods` no, `get pods/log` no.
- Real bearer token, isolated kubeconfig (`KUBECONFIG=/dev/null`), identity confirmed with
  `kubectl auth whoami` first (a first attempt without isolation gave a false "access granted"
  because the loaded kubeconfig's admin client cert silently took precedence over `--token`):
  deployments and events readable; `get pods` and `get secrets` returned `Forbidden`.
- Restart *causes* (liveness-probe failure vs. real crash) and scheduling failures
  (`FailedScheduling`, `Insufficient cpu`, node-selector mismatch) were visible only in Events
  (Labs 02–03) — concrete justification for including `events` in the ClusterRole.

## Trade-offs
- Requires a ClusterRole (cluster-scoped, needed for `nodes`) rather than a per-namespace Role —
  broader blast radius if the ServiceAccount token leaks, mitigated by the token granting only
  read verbs on non-sensitive resources.
- No pod-level API access means the backend cannot cross-check kube-state-metrics data against
  live pod state directly; accepted, since Prometheus/KSM data was verified sufficient in Lab 06.

## Consequences
- Backend never sees environment variables, secret references' values, or last-applied-config
  annotations — the exact exposure demonstrated in Lab 04 is structurally impossible.
- RBAC tests (Phase 7) must use an isolated kubeconfig and assert `auth whoami` before every
  Forbidden-check, per the Lab 04 testing pitfall.
- kube-state-metrics and metrics-server (system components, not the analyzer) still have broader
  read access to pods — documented in docs/threat-model.md as an accepted, upstream-scoped risk.

## Status
Accepted — 2026-09-24
