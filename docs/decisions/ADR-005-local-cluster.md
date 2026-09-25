# ADR-005: Local cluster — kind, single node

## Context
Development and demo happen on one laptop with ~7 GiB available RAM (measured 2026-09-23) and a
~4.5 GiB budget set aside for the whole cluster. `fs.inotify.max_user_instances=128` is a known
failure point for multi-node kind (kind's own documentation names this case).

## Options
- **kind**: upstream kubeadm control plane; same tool used in Kubernetes' own CI; used by
  GitHub Actions for K8s testing.
- **k3d**: lighter (k3s-based), fastest startup, but bundles Traefik/ServiceLB not needed here.
- **minikube**: mature, but heavier and slower to start; less common in CI.

## Decision
kind, single node, cluster name `workload-analyzer`. Fall back to k3d if available RAM drops
below 1.5 GiB after the monitoring stack is installed.

## Reasoning — measured in Phase 1 (2026-09-24)
- Idle footprint: ~550 MiB (below the 0.6–1.0 GiB Phase 0 estimate).
- Create time: 2 min 49 s total, Ready after 16 s (the rest was the one-time node-image pull).
- With the trimmed monitoring stack installed (ADR-003), total node container memory reached
  ~2.2 GiB against 14.79 GiB node-allocatable and ~6–7 GiB host-available — within budget, no
  fallback to k3d needed.
- Multi-node was never attempted: one node was sufficient to demonstrate scheduling concepts
  (Pending pods, taints, node selectors — Lab 03), and the measured inotify headroom
  (74/128 instances in use by the owner's account before the cluster existed) left no comfortable
  margin for a second node's watchers.

## Trade-offs
- k3d would likely save 200–400 MiB (not measured) — not needed given the measured headroom.
- Single node means no cross-node scheduling or node-affinity-across-nodes demonstration; taints,
  node selectors, and a CPU-request-too-large Pending pod cover the scheduling concepts the
  project needs without a second node.

## Consequences
- `learn/03-resources.yaml`'s `pending-cpu` pod (request `64` CPU) will always be Pending on this
  cluster — used deliberately as a scheduling-failure demonstration, not a bug.
- Cluster-level recovery (Docker container restart, host reboot) was not tested in Phase 1 and is
  deferred to Phase 7.

## Status
Accepted — 2026-09-24
