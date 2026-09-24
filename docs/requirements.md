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
| CPU throttling | `container_cpu_cfs_throttled_periods_total`, `container_cpu_cfs_periods_total` | cAdvisor |
| Memory usage | `container_memory_working_set_bytes` (what the OOM killer and eviction act on) | cAdvisor |
| Restarts | `kube_pod_container_status_restarts_total` | KSM |
| OOM | `kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}` (EXPERIMENTAL in KSM) combined with restart increase; `container_oom_events_total` exists in kubelet since 1.24 but is reported to stay at 0 (cAdvisor issue) — test with fixture #2 before relying on it | KSM / cAdvisor |
| CrashLoop / image errors | `kube_pod_container_status_waiting_reason{reason=~"CrashLoopBackOff\|ImagePullBackOff\|ErrImagePull\|CreateContainerConfigError"}` | KSM |
| Pending | `kube_pod_status_phase{phase="Pending"}`, `kube_pod_status_unschedulable` | KSM |
| Deployment availability | `kube_deployment_spec_replicas`, `kube_deployment_status_replicas_available` | KSM |
| Node health | `kube_node_status_condition{condition=~"Ready\|MemoryPressure\|DiskPressure\|PIDPressure"}`, `kube_node_status_allocatable` | KSM |
| HPA coupling | `kube_horizontalpodautoscaler_info` (scale target), `kube_horizontalpodautoscaler_spec_target_metric` | KSM |
| Data coverage | `count_over_time(<usage series>[window])` vs expected sample count | derived |

**Known limitation:** `last_terminated_reason` only shows the *most recent* termination. To count
OOMs, combine a restart increase with that reason, or use `container_oom_events_total`.

## 3. MVP rules (thresholds are proposals until validated on demo workloads)

Every finding carries a `data_quality` block produced by R005. R001–R004 run only when R005
status is `ok`.

| ID | Rule | Condition | Window | Min data |
|---|---|---|---|---|
| R001 | CPU over-requested | p95(usage) < 0.5 × request AND request − p95 ≥ 100m AND throttled ratio < 5% | standard 7d / demo 30m | R005 ok |
| R002 | Memory over-requested | max(working set) < 0.5 × request AND request − max ≥ 64Mi AND no OOM in window | standard 7d / demo 30m | R005 ok |
| R003 | Frequent restarts | restart increase ≥ 3 in 1h OR ≥ 10 in 24h; CrashLoopBackOff reason attached as evidence | 1h / 24h | series present |
| R004 | OOM / memory instability | ≥ 1 OOMKilled termination in 24h (critical) OR max(working set) ≥ 0.9 × limit (warning) | 24h | series present |
| R005 | Data quality gate | `insufficient`: history shorter than window or coverage < 80% · `stale`: newest sample older than 5m · `query_error`: query failed or timed out · else `ok` | per rule | — |

Suggested requests: CPU = p95 × 1.2; memory = max × 1.15 (compared against KRR).
Confidence: **HIGH** = ≥ 7d at ≥ 80% coverage · **MEDIUM** = ≥ 24h at ≥ 80% · **LOW** = demo profile.
CPU and memory use different logic on purpose: too little memory kills the pod, too little CPU only slows it.

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

## 5. Demo workloads (design only; created in Phase 2)

Namespace `demo`. Label `k8sa.dev/scenario=<name>`. Images pinned by digest. Apply with
`kubectl apply -k demo/`; expected results in `demo/expected-findings.yaml` drive integration tests.

| # | Workload | Mechanism | Expected result |
|---|---|---|---|
| 1 | Right-sized (negative control) | request 500m / 128Mi; stress-ng ~40% of one core, ~60Mi | no finding |
| 2 | CPU over-requested | request 1000m; stress-ng ~5% of one core | R001 |
| 3 | Memory over-requested | request 1Gi; holds ~50Mi | R002 |
| 4 | CPU spike | busybox: busy 30s, idle 270s, repeating | R001 LOW confidence or suppressed, with "bursty" caveat |
| 5 | OOM | request = limit = 128Mi; stress-ng keeps 200M | R004 critical (+ R003) |
| 6 | Restarting | busybox `exit 1` | R003 |
| 7 | HPA-controlled | copy of #2 with an HPA on CPU utilization 70% | R001 replaced by "HPA-coupled" caveat and HPA-neutral request |
| 8 | New workload | deployed ≥ 30m after the others | R005 `insufficient`, no R001/R002 |
| 9 | Pending (optional) | CPU request 64 | no MVP rule; used in Phase 1 to learn scheduling (backlog rule) |

Notes:
- For the HPA to actually scale (Phase 1 learning), metrics-server is needed (FREE SELF-HOSTED, small). Installed only after owner approval.
- Images verified to exist: `ghcr.io/colinianking/stress-ng` (upstream author, multi-arch incl. amd64) and `docker.io/library/busybox:1.37`. `--cpu-load` is approximate under cgroup limits, so `expected-findings.yaml` uses ranges, not exact values.
- Whether the OOM workload is recorded as `OOMKilled` at container level is UNVERIFIED (stress-ng respawns killed workers); proven in Phase 2.
- Total demo CPU stays under ~1 core. Crash/OOM pods are cheap: kubelet backoff caps restarts at one every 5 minutes.

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
