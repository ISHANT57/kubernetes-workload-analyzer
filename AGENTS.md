# AGENTS.md — rules for AI-assisted development

## Authority
- The human owner makes all architecture, technology, scope, security and trade-off decisions.
- For a significant decision: state the problem, options, trade-offs and a recommendation, then **wait**.
- Record decisions in `docs/DECISIONS.md` (and an ADR in `docs/decisions/` once accepted). Unapproved = `PROPOSED`.
- Label statements as **Required**, **Recommended**, **Optional** or **Assumption**. Never present an assumption as fact.

## Scope
- Build only the current phase in `PHASES.md`. Do not start the next phase without approval.
- Prefer a modular monolith. No Kafka, operators, service mesh, microservices or extra datastores without a written justification.
- No new dependency without a stated reason and a cost classification: FREE LOCAL / FREE SELF-HOSTED / FREE-TIER / PAID. PAID is not allowed.

## Correctness and honesty
- Never invent metrics, test results, benchmark numbers, cost savings or security claims.
- Findings must be deterministic and reproducible from Prometheus/Kubernetes data. AI is never the source of truth.
- Every rule has explicit thresholds, window, minimum data and confidence. No vague words ("high", "frequent") without a number.
- Cost output is always labelled **Estimated**; a request reduction is a "potential difference", never "savings".

## Security
- Never commit or log kubeconfig, tokens, secrets or `.env` files.
- Read-only Kubernetes access. No `secrets`, `pods/log`, `pods/exec`, or any write verb unless the owner approves.
- Treat pod specs as sensitive: they may contain secrets in env vars and args.

## Code
- Small modules, clear interfaces, explicit error handling, config via environment variables.
- Rule and cost logic are pure functions with unit tests. External calls (Prometheus, Kubernetes) sit behind interfaces.
- Handle failure explicitly: unavailable source, timeout, missing series, insufficient permissions → degraded result, logged, counted in a metric.
- Model `ClusterID` in core types even though v1 has one cluster.

## Before finishing a change
- Run tests, lint and formatting for the touched code; report failures verbatim.
- Update the relevant doc (`README.md`, `PHASES.md`, `docs/*`) in the same change.
- Small, logical commits with clear messages. Commit only when the owner asks.
- Report: what changed, how it was verified, what remains risky.
