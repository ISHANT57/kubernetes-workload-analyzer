# ADR-001: Backend language — Go

## Context
The analyzer backend needs a Kubernetes client (structure + Events, D-007), a Prometheus client,
a deterministic rule engine, a cost calculator and a REST API. It must run comfortably inside a
~4.5 GiB laptop cluster budget alongside the monitoring stack.

## Options
- **Go**: native `client-go` and official Prometheus client; single static binary; low memory.
- **Python**: fastest to write analysis/percentile logic; 3.14 already installed.
- **TypeScript (Node)**: same language as the frontend (D-002).

## Decision
Go, toolchain 1.27.

## Reasoning
- `client-go` is the reference Kubernetes client; the RBAC and Events work verified in Phase 1
  Lab 04 (`kubectl auth can-i`, isolated-kubeconfig testing) map directly onto it.
- Static binary means no runtime dependency inside the cluster budget measured in Phase 1
  (kind idle ~550 MiB, monitoring stack ~580 MiB, leaving limited headroom).
- Owner fluency in Go confirmed at decision time (2026-09-24).
- Go 1.24 (initially installed) is out of support; `client-go`'s `go.mod` requires 1.27. Installed
  1.27 directly rather than relying on `GOTOOLCHAIN=auto`.

## Trade-offs
- More verbose than Python for ad-hoc data wrangling (percentiles, aggregation).
- Smaller ecosystem for statistics than Python/pandas — rule logic (R001–R005) is simple enough
  (percentile, max, ratio) that this did not matter in practice.

## Consequences
- Backend ships as one static binary; packaging for in-cluster deployment (Phase 7) is simple.
- Rule and cost logic can be pure Go functions with table-driven tests (AGENTS.md requirement).
- Frontend (D-002, React/TypeScript) and backend are different languages; the JSON findings
  contract (docs/requirements.md §6) is the only coupling.

## Status
Accepted — 2026-09-24
