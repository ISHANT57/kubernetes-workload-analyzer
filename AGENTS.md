# AGENTS.md — rules for AI-assisted development

## Authority
- The human owner makes all architecture, technology, scope, security and trade-off decisions.
- For a significant decision outside the delegation below: state the problem, options,
  trade-offs and a recommendation, then **wait**.
- **Delegation (2026-09-26, explicit, in the owner's own words, twice):** for finishing Phases
  4–8 (rule engine, cost, API, dashboard, tests, hardening), full implementation autonomy is
  granted — architecture, libraries, API, UI, testing, refactors, routine commits/PRs/merges,
  adopting patterns from reference repos — without per-step confirmation. Ask first only before:
  destructive/hard-to-recover changes to existing work, paid services, irreversible external
  actions (force-push, deleting a repo), or anything needing credentials not already available.
  This does not relax the Correctness/Security/Code rules below — those are a harder floor, not
  a decision to renegotiate.
- Record only genuinely significant decisions in `docs/DECISIONS.md`. The per-decision full-ADR
  ceremony (`docs/decisions/ADR-NNN-*.md`) used in Phases 0–3 is paused during the delegation
  above, to avoid documentation-as-busywork; resume it if asked.
- Label statements as **Required**, **Recommended**, **Optional** or **Assumption**. Never
  present an assumption as fact.

## Scope
- Prefer a modular monolith. No Kafka, operators, service mesh, microservices or extra datastores
  without a written justification.
- No new dependency without a stated reason and a cost classification: FREE LOCAL / FREE
  SELF-HOSTED / FREE-TIER / PAID. PAID is not allowed.
- Reference repos (`../k8s-reference-repos/*`): inspect actual source before reusing anything;
  adapt only what materially improves this project, never copy wholesale; preserve licenses and
  note in the commit/code where a pattern was adapted from.

## Correctness and honesty
- Never invent metrics, test results, benchmark numbers, cost savings, security claims, files or
  APIs. Never say something works without actually testing it.
- Findings must be deterministic and reproducible from Prometheus/Kubernetes data. AI is never
  the source of truth.
- Every rule has explicit thresholds, window, minimum data and confidence. No vague words
  ("high", "frequent") without a number.
- Cost output is always labelled **Estimated**; a request reduction is a "potential difference",
  never "savings".
- Missing/insufficient data produces an explicit caveat or safe no-finding, never a guess.

## Security
- Never commit or log kubeconfig, tokens, secrets or `.env` files.
- Read-only Kubernetes access. No `secrets`, `pods/log`, `pods/exec`, or any write verb unless the
  owner approves.
- Treat pod specs as sensitive: they may contain secrets in env vars and args.

## Code
- Small modules, clear interfaces, explicit error handling, config via environment variables.
- Rule and cost logic are pure functions with unit tests. External calls (Prometheus, Kubernetes)
  sit behind interfaces.
- Handle failure explicitly: unavailable source, timeout, missing series, insufficient
  permissions → degraded result, logged, counted in a metric.
- Model `ClusterID` in core types even though v1 has one cluster.

## Before finishing a change
- Run tests, lint and formatting for the touched code; fix failures; report anything that stays
  broken verbatim, never silently.
- Update the relevant doc (`README.md`, `PHASES.md`, `docs/DECISIONS.md`) in the same change.
- Small, logical commits with clear messages.
- Report: what changed, how it was verified, what remains risky.
