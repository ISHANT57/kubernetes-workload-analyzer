# Demo runbook

Scripted walkthrough for demonstrating the platform, one section per success criterion from the
original brief (§24, items 1–8 — the demonstrable core; items 9–14 are documentation/testing
artifacts covered by `backend/README.md`, `docs/threat-model.md`, `docs/architecture.md` and the
`go test ./...` suite, not a live demo step).

Every command below is real and was run against the actual cluster while writing this runbook.
Example output is one real captured sample (labelled where shown) — duty-cycled demo workloads
vary run to run, so treat exact numbers as illustrative, not a guaranteed replay. Nothing here is
invented; if a number can't be reproduced live, it isn't included.

## Prerequisites

```bash
kind get clusters                                    # workload-analyzer should be listed
kubectl --context kind-workload-analyzer get nodes    # 1 node, Ready
kubectl --context kind-workload-analyzer -n monitoring get pods    # kube-prometheus-stack running
kubectl --context kind-workload-analyzer -n analyzer get pods      # analyzer Running 1/1 (Phase 7)
kubectl apply -k demo/               # if not already applied (Phase 2 fixtures)
kubectl -n analyzer port-forward svc/analyzer 8080:8080 &
```

---

## 1. A Kubernetes cluster being monitored

```bash
curl -s localhost:8080/healthz    # process liveness -- always ok while the process is up
curl -s localhost:8080/readyz     # 200 only when the last analysis run was fully clean
curl -s localhost:8080/api/runs/latest | jq .
```
Real example (captured live from the in-cluster analyzer, `svc/analyzer`, while writing this runbook):
```json
{
  "id": "run-20260925T204740-d01887b2",
  "cluster_id": "workload-analyzer",
  "status": "complete",
  "workloads_seen": 17,
  "findings_count": 3,
  "query_errors": []
}
```
`cluster_id` carries `ClusterID` end to end (model, findings, API) even though v1 targets one
cluster — the type exists so multi-cluster is additive later, not a rewrite (§14 of the brief).

## 2. Workloads being discovered automatically

`workloads_seen` above (17) is not hardcoded — it comes from a live Kubernetes API list
(`internal/k8sclient`, D-007-scoped: `deployments`/`statefulsets`/`daemonsets`, read-only) run
fresh every analysis cycle:
```bash
kubectl --context kind-workload-analyzer get deployments,statefulsets,daemonsets -A --no-headers | wc -l
curl -s localhost:8080/api/findings | jq '[.[].workload.name] | unique | length'   # distinct workloads with a finding
```
The Workloads page (`/workloads` in the dashboard) lists every discovered workload, not just ones
with findings, confirming discovery is independent of the rule engine.

## 3. CPU and memory metrics being collected

```bash
curl -s 'localhost:8080/api/timeseries?namespace=demo&workload=cpu-over-requested&container=c&metric=cpu&window=24h' | jq '.points | length, .points[0], .points[-1]'
```
Real example (79 points, `{"t": <unix seconds>, "v": <cores>}`):
```json
{"t": 1790308664, "v": 0.0468}
{"t": 1790369864, "v": 0.0404}
```
This hits the same Prometheus queries the evidence builder uses (`internal/evidence/builder.go`),
parameterized server-side — the browser never sends PromQL. The Finding Detail page renders this
as a real uPlot usage-vs-request chart for any R001/R002 finding.

## 4. Resource requests being compared against actual usage

```bash
curl -s localhost:8080/api/findings | jq '.[] | select(.rule_id=="R001" or .rule_id=="R002") | {workload: .workload.name, rule_id, evidence}'
```
Each `evidence` item carries the exact PromQL `query` that produced it, so the number is always
traceable back to Prometheus, not asserted (`docs/requirements.md` §2–3). Real evidence pulled live
from `cmd/verify` against `demo/cpu-over-requested` (request 1 core, `p95=0.0566` core over the
last 30m window via `rate(...[2m])`):
```
cpu: req=1.000(true) limit=0.000(false) p50=0.0495 p95=0.0566 p99=0.0589 burstiness=1.19
mem: req=67108864(true) limit=134217728(true) max=1400832
```
Whether this surfaces as an `R001` finding right now also depends on §7's data-quality gate — see
the note there for what this run showed live.

## 5. Pod failures/restarts being detected

```bash
curl -s localhost:8080/api/findings | jq '.[] | select(.rule_id=="R003" or .rule_id=="R004") | {workload: .workload.name, rule_id, severity, problem}'
```
Two demo fixtures exercise this directly: `demo/oom` (OOMKilled, exit 137 → R003 + R004,
`severity: critical`) and `demo/crashloop` (exit 1, `CrashLoopBackOff` → R003 only, R004 correctly
does **not** fire because the reason is `Error`, not `OOMKilled` — this distinction is a named
negative-control case in `demo/expected-findings.yaml`).

## 6. Health issues appearing clearly on the dashboard

Open `http://localhost:8080/` (analyzer serves the built React dashboard directly, Phase 5).
- **Overview** — top findings ranked health-first, then resource, then cost.
- **Findings** — full filterable list (severity, category, rule).
- **Finding Detail** — evidence table, threshold, window, data-quality/confidence badge, caveats,
  and (for R001/R002) the usage-vs-request chart, plus a **Grafana** link pre-filtered to that
  exact namespace/workload (Phase 6).
- **Platform Status** — the analyzer's own health: last run status, query errors, data coverage.

Verified live via Playwright in Phase 5/6: desktop, mobile (390px), dark mode, zero console errors.

## 7. Resource optimization opportunities being identified

Same R001/R002 findings as §4, but the point here is what's *excluded* and why — this is where
the rule engine earns its evidence-based framing over a naive threshold:
```bash
curl -s localhost:8080/api/findings | jq '.[] | select(.workload.name=="hpa-coupled" or .workload.name=="cpu-spike" or .workload.name=="right-sized")'
```
- `demo/right-sized` — usage/request ratio doesn't cross the threshold → **no finding** (negative
  control).
- `demo/cpu-spike` — a 30s burst every 300s smooths to a low mean, but p99/p50 burstiness > 4 caps
  confidence at LOW and suppresses R001 (the burstiness-guard bug found and fixed in Phase 4).
- `demo/hpa-coupled` — a real `HorizontalPodAutoscaler` targets this workload's CPU utilization;
  the rule engine detects `kube_horizontalpodautoscaler_info`/`..._spec_target_metric` and emits an
  `hpa-coupled` caveat with an HPA-neutral request suggestion instead of a plain resize (a naive
  resize here was shown in Phase 1 Lab 05 to inflate replicas and total requested resources, the
  opposite of the intent). Confirmed live: `cmd/verify` shows `hpa: present=true cpu=true(70%)` for
  this workload right now.

**Live data-quality note:** running this section right after the Phase 7 outage test (Prometheus
was deliberately taken down for several minutes, §"Failure-path demo" below) showed every R001/R002
candidate at `dq=insufficient ("less than 80% coverage over the minimum 30m0s window")` — the
outage itself created the gap. This is R005 (the data-quality gate) doing exactly its job: no
resource finding is asserted on thin data, rather than guessing through the gap. It self-clears as
the outage window ages out of the 30-minute lookback; re-running the §4 command a bit later than
the outage reliably shows `R001`/`R002` again once coverage crosses 80%.

## 8. Estimated cost impact being calculated with documented assumptions

```bash
curl -s localhost:8080/api/findings | jq '.[] | select(.cost != null) | {workload: .workload.name, cost}'
```
Every cost field traces to `CPU_CORE_HOUR_USD`/`MEMORY_GIB_HOUR_USD`/`PRICE_SOURCE` set at
startup — no built-in default price (`internal/cost/cost.go`: `PricingUnset` fails closed if
either is missing, so a finding either has a fully-assumption-labelled cost or none at all, never
a silent guess). `internal/findings/findings.go` estimates over `CostHours = 730` (the conventional
average hours/month). The in-cluster deployment (`deploy/analyzer/deployment.yaml`) sets:
```
CPU_CORE_HOUR_USD=0.031611
MEMORY_GIB_HOUR_USD=0.004237
PRICE_SOURCE=example only, approx AWS m5.large on-demand blended rate, NOT a real quote
```
Applying that same formula to the real §4 evidence for `demo/cpu-over-requested` (request 1 core,
observed p95 0.0566 core, allocation = `max(request, observed)`):
`allocation_cost_usd = 1.000 * 0.031611 * 730 ≈ 23.08`,
`optimized_cost_usd = 0.0566 * 0.031611 * 730 ≈ 1.31`,
`potential_difference_usd ≈ 21.78` — labelled `potential_difference_usd` in the API, never
"savings": lowering a request only reduces spend if it lets a node be removed or avoided
(`README.md` → Limitations), which this project cannot observe from inside one cluster. This is
worked out here from the real formula and real evidence above, not copied from a live API response
— pull `/api/findings` directly (command above) once R001 is firing (see §7's data-quality note)
for the actual computed value.

---

## Failure-path demo (bonus, proven in Phase 7)

```bash
kubectl --context kind-workload-analyzer -n monitoring patch prometheus kps-kube-prometheus-stack-prometheus \
  --type merge -p '{"spec":{"replicas":0}}'
sleep 40 && curl -s localhost:8080/api/runs/latest | jq '{status, findings_count, query_errors}'
# status: "partial", findings_count stays at its last good value (sticky), query_errors shows the real dial error
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/readyz   # 503

kubectl --context kind-workload-analyzer -n monitoring patch prometheus kps-kube-prometheus-stack-prometheus \
  --type merge -p '{"spec":{"replicas":1}}'
# wait for the Prometheus pod Ready, then the analyzer's next cycle recovers on its own:
curl -s localhost:8080/api/runs/latest | jq '{status, findings_count}'   # back to "complete"
```

## Cleanup

```bash
kill %1   # the port-forward started above
```
