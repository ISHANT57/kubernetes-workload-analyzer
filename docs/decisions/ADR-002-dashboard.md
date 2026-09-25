# ADR-002: Dashboard — React + Vite + TypeScript, Grafana kept for trends

## Context
The brief requires the dashboard to answer "what is wrong?" before "what data exists?", and to be
a genuine portfolio artifact. Phase 1 confirmed the browser must never query Prometheus directly
(security, D-007); all data reaches the UI as backend-owned JSON.

## Options
- **Grafana only**: zero UI code, but weak at ranked/explained findings; not "a dashboard you built".
- **Server-rendered HTML (+htmx)**: least code, one toolchain, weaker portfolio signal.
- **React + Vite + TypeScript SPA**, served as static files by the Go backend, consuming the
  findings JSON API; Grafana kept for raw time-series investigation.

## Decision
React + Vite + TypeScript, with uPlot for the small number of charts that answer a specific
question (usage-vs-request with p95/max; restart/OOM timeline). Built only after the findings
contract and real backend output are stable (Phase 5, after Phase 3–4).

## Reasoning
- Portfolio value: a built dashboard demonstrates frontend engineering, not just kubectl usage.
- Still one deployable: Vite build output is static files served by the Go backend; no Node
  process at runtime, so it adds no memory to the cluster budget.
- Grafana already exists in the monitoring stack (D-003) and is good at trend charts — no reason
  to rebuild that; it is kept for the "what does this look like over time" question while the
  React app answers "what is wrong and why".

## Trade-offs
- Second toolchain (Node, build-time only) vs. a single-language stack.
- The findings JSON contract must stay in sync between backend and frontend — mitigated by
  designing the contract from real backend output (docs/requirements.md §6) before building UI.

## Consequences
- Backend must serve static files in addition to JSON — a `web` module (docs/architecture.md).
- No dashboard work starts until Phase 3–4 produce real findings; avoids designing against
  imagined data.
- Grafana panels are linked from finding detail pages (Phase 6), not duplicated.

## Status
Accepted — 2026-09-24
