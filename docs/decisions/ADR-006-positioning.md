# ADR-006: Project positioning — unified, explainable findings

## Context
OpenCost, Robusta KRR, VPA/Goldilocks and Grafana already cover pieces of this project's scope
(cost allocation, rightsizing recommendations, dashboards). The project is explicitly a portfolio
and company/college demonstration piece, so its reason for existing must be stated honestly.

## Options
- **Learning/reimplementation**: "rebuilt a slice of OpenCost+KRR to understand it" — risks
  reading as a copy without added depth.
- **Unified findings + explanations**: one ranked list across health, rightsizing and estimated
  cost, where every finding shows evidence, confidence, and caveats (HPA-coupling, JVM, batch
  jobs) — a narrower but more defensible gap than either existing tool covers alone.
- **Narrower single-gap tool** (e.g. HPA-aware rightsizing only): deep on one problem, smaller
  demo surface.

## Decision
Portfolio/demonstration project: unified, explainable findings across health, resource
rightsizing and estimated cost. KRR and OpenCost are used as comparison baselines (Phase 4/7),
not treated as if they don't exist.

## Reasoning
Phase 1 produced concrete evidence that "unified and explainable" is a real gap, not just a
pitch: KRR's simple strategy uses p95 CPU / max+15% memory alone and explicitly skips HPA-managed
workloads by default; Phase 1 Lab 05 showed that skip is warranted — lowering a request under an
HPA increased total allocation (CPU +50%, memory 3×) rather than reducing it. A tool that
explains *why* a recommendation is or isn't being made (HPA-coupled caveat, insufficient-data
gate, throttling gate) adds real value beyond a bare percentile calculation.

## Trade-offs
- Must actually out-explain the tools it borrows ideas from, or the positioning is just marketing.
- Smaller demo surface than "full OpenCost clone"; deliberately scoped to R001–R005 only.

## Consequences
- README carries a "why build this if X exists" comparison table (already present).
- Phase 4/7 must run KRR against the same Prometheus data and document agreements/differences,
  not just claim superiority.
- AI (if added later, §22 of the original brief) may explain findings but is never the source of
  truth for them — findings stay deterministic and reproducible from Prometheus/K8s data.

## Status
Accepted — 2026-09-24
