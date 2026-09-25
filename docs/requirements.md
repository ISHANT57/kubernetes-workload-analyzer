# Requirements (Phase 0 draft)

Everything below is **PROPOSED** unless marked otherwise. Thresholds are starting points to be
validated against fixture data in Phase 2 — they are not tuned values.

## 1. MVP

### Candidate from the brief
> Run the platform against one local Kubernetes cluster, retain at least 24 hours of Prometheus
> data, and show a ranked list of unhealthy and potentially over-provisioned workloads with
> explanations and estimated resource/cost impact.

### Assessment
Realistic in scope, but three gaps:
1. **"24 hours" on a laptop is fragile.** Sleep, reboot or cluster recreation creates gaps or wipes data. Prometheus in kube-prometheus-stack uses `emptyDir` unless storage is configured, so a pod restart loses history. Needs a PVC and explicit gap handling.
2. **Not testable as written.** "Shows a list" has no pass/fail. The fixtures give a ground truth to check against.
3. **Waiting 24h blocks every test run.** Rules need configurable windows: a *standard* profile (real use) and a *demo* profile (short windows, always labelled LOW confidence).

### Proposed done-condition
The MVP is done when all of these are true:
1. One local cluster runs the monitoring stack with Prometheus on persistent storage and a retention of at least 7 days.
2. The demo namespace is deployed from the repository with a single command.
3. The backend shows one ranked findings list (health first, then resource, then cost) in which:
   - every fixture scenario produces its expected finding, and the negative-control fixture produces none;
   - each finding shows its metric evidence, threshold, window, data coverage and confidence;
   - resource findings show estimated allocation cost, estimated optimized allocation cost and the potential difference, with the pricing assumptions displayed.
4. After at least 24h of collected data, resource findings are reported at MEDIUM confidence or better. Demo-profile findings are always labelled LOW.
5. An automated check compares actual findings against `demo/expected-findings.yaml` and passes.
6. The backend exposes `/healthz`, `/readyz` and `/metrics`, and writes structured logs.
7. The backend degrades cleanly and visibly when Prometheus is unreachable (tested).

Out of MVP: auth beyond local-only access, multi-cluster, alerting, finding history, AI, CI deployment.

## 2. Measurement model (minimum metrics)

Sources: **KSM** = kube-state-metrics · **cAdvisor** = kubelet `/metrics/cadvisor` · **node** = node-exporter.
All must be verified to exist in Phase 1 before any rule is written. Labels on container
metrics must be filtered (`container!="", container!="POD"`).

| Area | Metric | Source |
|---|---|---|
| Workload identity | `kube_pod_owner`, `kube_replicaset_owner`, `kube_pod_info` | KSM |
| Requests / limits | `kube_pod_container_resource_requests{resource="cpu"\|"memory"}`, `kube_pod_container_resource_limits` | KSM |
| CPU usage | `rate(container_cpu_usage_seconds_total[5m])` (or the stack's recording rule) | cAdvisor |
| CPU throttling | `container_cpu_cfs_throttled_periods_total`, `container_cpu_cfs_periods_total` — emitted **only for containers with a CPU limit**; absent otherwise (VERIFIED) | cAdvisor |
| Memory usage | `container_memory_working_set_bytes` (what the OOM killer and eviction act on) | cAdvisor |
| Restarts | `kube_pod_container_status_restarts_total` | KSM |
| OOM | `kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}` (EXPERIMENTAL in KSM) combined with restart increase (VERIFIED: 5 OOMs → reason `OOMKilled`, restarts 5). **Not used:** `container_oom_events_total` stayed 0 through 5 real OOMs (VERIFIED); `node_vmstat_oom_kill` is node-wide, cannot be attributed to a container | KSM |
| CrashLoop / image errors | `kube_pod_container_status_waiting_reason{reason=~"CrashLoopBackOff\|ImagePullBackOff\|ErrImagePull\|CreateContainerConfigError"}` | KSM |
| Pending | `kube_pod_status_phase{phase="Pending"}`, `kube_pod_status_unschedulable` | KSM |
| Deployment availability | `kube_deployment_spec_replicas`, `kube_deployment_status_replicas_available` | KSM |
| Node health | `kube_node_status_condition{condition=~"Ready\|MemoryPressure\|DiskPressure\|PIDPressure"}`, `kube_node_status_allocatable` | KSM |
| HPA coupling | `kube_horizontalpodautoscaler_info` (scale target), `kube_horizontalpodautoscaler_spec_target_metric` | KSM |
| Data coverage | `count_over_time(<usage series>[window])` vs expected sample count | derived |

**Known limitation:** `last_terminated_reason` only shows the *most recent* termination. To count
OOMs, combine a restart increase with that reason.

**Absent series ≠ missing data (VERIFIED in Phase 1).** State metrics such as `waiting_reason`,
`kube_pod_status_unschedulable`, HPA metrics and CPU throttling exist only while the condition or
object exists. For these, "no series" means "condition not present". For usage metrics
(`container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`), no series means
missing data → R005 `insufficient`. Also: cAdvisor returns a pod-level series with empty
`container` label next to each container series; always filter `container!=""`.
Instant queries can miss short states (a CrashLoopBackOff between restarts); use
`max_over_time(...[window])`.

## 3. MVP rules — built and verified live, Phase 4 (2026-09-26)

R001/R002 (percentile-based) each carry a `data_quality` block from R005's coverage-tier gate
and only fire when its status is `ok`. R003/R004 (counter/state-based: restart counts and
termination reasons) use a simpler gate -- "series present" -- not the coverage-tier check: a
restart counter is meaningful with far less history than a percentile calculation needs, and
applying the same 30m/24h/7d coverage bar to it would under-serve health findings that should be
allowed to fire quickly.

| ID | Rule | Condition (as implemented) | Window | Min data |
|---|---|---|---|---|
| R001 | CPU over-requested | p95(usage) < 0.5 × request AND request − p95 ≥ 100m AND throttled ratio < 5% AND not HPA-CPU-coupled AND not bursty (p99/p50 ≤ 4) | R005 tier (30m/24h/7d) | R005 ok |
| R002 | Memory over-requested | max(working set) < 0.5 × request AND request − max ≥ 64Mi AND no OOM in the last 24h | R005 tier (30m/24h/7d) | R005 ok |
| R003 | Frequent restarts | restart increase ≥ 3 in 1h OR ≥ 10 in 24h; waiting reason (e.g. CrashLoopBackOff) attached as evidence when present | 1h / 24h | restart counter present |
| R004 | OOM | last_terminated_reason == OOMKilled AND restarts increased in the last 24h | 24h | termination reason + restart counter present |
| R005 | Data quality gate | `insufficient`: below 80% coverage even at the 30m minimum tier (or zero data) · `stale`: newest sample older than 5m · `ok`: at least the 30m tier clears 80% | 30m / 24h / 7d tiers | — |

**Not implemented in the MVP** (documented here so it isn't silently forgotten, not treated as
done): R004's original design also included "max(working set) ≥ 0.9 × limit" as a warning-level
near-limit signal separate from a confirmed OOM. No demo fixture exercises it and it was left out
to keep R004 to the one condition actually verified end-to-end. Candidate for a later phase.

Suggested requests: CPU = p95 × 1.2; memory = max × 1.15 (compared against KRR in a later phase).
Confidence: **HIGH** = ≥ 7d at ≥ 80% coverage · **MEDIUM** = ≥ 24h at ≥ 80% · **LOW** = only the 30m
minimum tier clears 80%. R003/R004 report no tiered confidence (empty) -- they are deterministic
counter facts, not statistical estimates; `confidence_reason` explains why in place of a tier.
CPU and memory use different logic on purpose: too little memory kills the pod, too little CPU only
slows it.

### Implementation notes (real bugs found via live verification, not just unit tests)
- **Pod resolution without Pods API access (D-007):** `container_cpu_usage_seconds_total` etc.
  only carry a `pod` label, not an owner reference. Pods are resolved via
  `kube_pod_owner` joined to `kube_replicaset_owner` on the ReplicaSet name (`label_replace` on
  both sides to align the join key) -- verified live to return exactly the right pod name before
  being relied on. See `internal/evidence/builder.go`'s `resolvePodNames`.
- **Coverage is self-calibrated, not assumed from a scrape interval.** An assumed 30s interval
  was checked against real fixture data and found off by >2x (823 samples over ~15h where the
  assumption predicted ~1816) -- likely scrape-job cadence differences and real gaps (see below).
  Coverage instead comes from the actual sample timestamps returned: span covered ÷ window,
  penalized for the single largest gap found.
- **A `rate()` lookback that is too long silently defeats the bursty guard.** The CPU usage query
  originally used `rate(...[5m])`, copied from krr's own pattern -- correct for krr's use case,
  but for the cpu-spike fixture (30s burst every 300s) a 5-minute lookback smoothed every
  evaluated point into a near-constant ~10% average, destroying the p50/p99 spread the burstiness
  guard depends on. Changed to `rate(...[2m])`, short enough to leave idle-only points near zero
  and burst-overlapping points visibly elevated.
- **A zero median is the strongest possible burstiness signal, not the absence of one.** The
  first version of the p99/p50 ratio returned 0 when p50 was exactly 0 (treating "no meaningful
  median" the same as "no signal at all") -- exactly backwards for a workload idle more than half
  the time with a real usage tail. Fixed to return +Inf in that case.
- **Real infrastructure noise, not a bug:** every long-running demo workload showed one restart
  with `reason=Unknown` from a `SandboxChanged` event (containerd sandbox recreation, most likely
  from the laptop sleeping overnight -- the exact documented laptop-sleep-gaps risk). Correctly
  did not trigger R003 (well under both thresholds) and did not affect R004 (checks the specific
  reason string, not "any non-empty reason").
- **Verified against real, non-fixture data too:** the live analyzer found a genuine R002 finding
  on `kube-system/metrics-server` (200Mi requested vs ~59Mi observed) -- confirms the rule engine
  generalizes beyond the demo/ fixtures it was designed against.

### Exceptions and caveats (these apply before any recommendation)
- **HPA targets CPU/memory utilization:** utilization is measured *relative to request*. Lowering the request raises measured utilization and makes the HPA scale out sooner, which may cost *more*. Emit an "HPA-coupled — review together" finding, not a resize.
- **Batch Jobs/CronJobs:** short-lived pods give sparse data. Excluded from resource rules in MVP; health rules still apply.
- **Bursty workloads:** if p99/p50 > 4, cap confidence at LOW and say why.
- **JVM and other self-managed-heap runtimes:** working set reflects configured heap, not need. They cannot be detected reliably, so support an opt-out annotation and cap memory confidence at MEDIUM.
- **Recently changed workloads:** if requests changed inside the window, only evaluate data since the change.

### Backlog (after R005)
CrashLoopBackOff-only, image pull failure, unschedulable/Pending, deployment unavailable, node
conditions, CPU throttled (under-provisioned), memory growth.

## 4. Cost model

Terms are kept separate on purpose:

| Term | Definition |
|---|---|
| Requested | Sum of container requests |
| Observed usage | p95 CPU / max memory over the window |
| Allocation | Per container: max(request, observed usage) (same idea as OpenCost) |
| Request slack | Request − observed usage (workload level) |
| Idle capacity | Node allocatable − sum of requests on the node (cluster level) |
| Estimated allocation cost | Allocation × unit price × hours |
| Estimated optimized allocation cost | Suggested request × unit price × hours |
| Potential difference | Allocation cost − optimized cost. **Not savings.** |

Unit prices (`CPU per core-hour`, `memory per GiB-hour`) come from configuration. They have no
built-in defaults that imply a real cloud price; any example value must cite its source.

**Required disclaimer (shown in the UI and docs):** Reducing a pod's request does not by itself
reduce the cloud bill. You pay for nodes. Money is saved only if the reduced requests let nodes be
removed, downsized, or not added (by a cluster autoscaler or manual change). Until then, freed
requests become idle capacity on nodes you still pay for.

The pricing source sits behind an interface, so a flat-rate model can later be replaced by per-node
pricing or OpenCost data.

## 5. Demo workloads (built and verified in Phase 2, 2026-09-25)

Namespace `demo`. Label `k8sa.dev/scenario=<name>`. Apply with `kubectl apply -k demo/`; ground
truth (captured against live Prometheus queries) is in `demo/expected-findings.yaml`, which
drives the Phase 4 integration test.

**Deviation from the original design:** built with `registry.k8s.io/e2e-test-images/agnhost:2.53`
and `resource-consumer:1.15.0` only — both already verified and cached on the node from Phase 1
(no stress-ng, no busybox, zero new image pulls). agnhost's shell (`sh`/`busybox`/`timeout`/`yes`,
confirmed present in Phase 1 Lab 02) gives precise, throttle-free fractional CPU via a
busy/idle duty cycle (`timeout Ns yes >/dev/null; sleep Ms`), which stress-ng's `--cpus`/`--cpu-load`
would only approximate. agnhost's built-in `stress` subcommand (already proven for the Phase 1
OOM lab) covers the memory scenarios directly.

| # | Workload | Mechanism | Verified result (2026-09-25) | Expected result |
|---|---|---|---|---|
| 1 | `right-sized` (negative control) | request 500m/128Mi; 40% CPU duty cycle + agnhost stress holding ~60Mi | 320–460m CPU (~80% of request), ~65Mi mem | no finding |
| 2 | `cpu-over-requested` | request 1000m; 5% CPU duty cycle, no limit | `rate(...[2m])` ≈ 55–83m (~6% of request) | R001 |
| 3 | `memory-over-requested` | request 1Gi; agnhost stress holds ~50Mi | ~57–60Mi steady | R002 |
| 4 | `cpu-spike` | request 500m; 1 core busy 30s / idle 270s | 0m idle → 999m burst (caught live) | R001 suppressed/LOW, "bursty" caveat |
| 5 | `oom` | request=limit=128Mi; agnhost stress allocates 200Mi | `OOMKilled`, exit 137, 5 restarts | R004 critical (+R003) |
| 6 | `crashloop` | `sh -c "exit 1"` | `Error`, exit 1, 5 restarts, CrashLoopBackOff | R003 |
| 7 | `hpa-coupled` | resource-consumer, real HPA (target 70% CPU util), no load driver | idle (~0m/5Mi), `kube_horizontalpodautoscaler_info` present | R001 replaced by "HPA-coupled" caveat + HPA-neutral request |
| 8 | `new-workload` | agnhost pause, deployed with the others | idle, age < 10 min at check time | R005 `insufficient`, no R001/R002 |
| 9 | `pending` | CPU request `64` | `FailedScheduling: Insufficient cpu`, pod Pending | no MVP rule; backlog scheduling-health rule |

Notes:
- Fixture #7 does not drive load: the HPA-coupled *caveat logic* is what's under test here, not
  actual scaling (that mechanism was already proven end-to-end in Phase 1 Lab 05 — lowering a
  request under this exact HPA shape increased total allocation, CPU +50%/memory 3×).
- Fixture #8's "newness" needs no special timing: any rule with a minimum-data window longer than
  the fixture's actual age must gate to `insufficient` regardless of when it was created.
- Total demo CPU stays under ~1.5 cores at peak (only #4's burst and #2's duty cycle overlap
  briefly); steady-state is well under 1 core. OOM/crashloop restarts are cheap: kubelet backoff
  caps them at one every 5 minutes.
- All values queried live via `kubectl get --raw` against the Prometheus service proxy, not
  `kubectl top` alone (see docs/kubernetes-fundamentals.md entry 07).

## 6. Findings contract (draft; evolves from real backend output)

Finding: `id` (hash of cluster_id + rule_id + namespace + workload + container), `cluster_id`,
`rule_id`, `analysis_run_id`, `generated_at`, `severity` (critical|warning|info), `category`
(health|resource|cost), `workload {kind, namespace, name}`, `container`, `problem`,
`evidence[] {metric, value, unit, query}`, `threshold`, `window`,
`data_quality {status, coverage, last_sample_at}`, `confidence` (LOW|MEDIUM|HIGH),
`confidence_reason`, `caveats[]`, `cost` (null unless resource finding:
`{allocation, optimized, difference, currency, assumptions[]}`).
Units live in field names or `unit` (`cpu_millicores`, `memory_bytes`). Missing values are
`null`, never 0.

Analysis run: `id`, `started_at`, `duration_ms`, `status` (complete|partial|failed),
`workloads_seen`, `query_errors[]`. Served at `/api/runs/latest`.
Idempotency: the same inputs produce the same finding IDs, so a repeated run updates findings instead of duplicating them.
