# Kubernetes fundamentals — lab notes

Cluster: kind `workload-analyzer`, Kubernetes v1.37.0, single node. Namespace `learn`.

## 01 — Deployment, ReplicaSet, Pod, labels, self-healing, scaling (2026-09-24)
Manifest: `learn/01-deployment.yaml` (agnhost 2.53, 2 replicas, request 50m/32Mi, limit 64Mi)

| Experiment | Observation | Command |
|---|---|---|
| Apply Deployment | Deployment → ReplicaSet `web-5b8dbf6fc9` → 2 Pods; ownerReferences confirm the chain | `kubectl -n learn get deploy,rs,pods --show-labels` |
| Labels | All objects carry `app=web`; RS/Pods also `pod-template-hash` (changes when the pod template changes) | same |
| Delete a pod | Replacement Running within ~3 s with a new name and IP | `kubectl -n learn delete pod <name>` |
| Ordering | `SuccessfulCreate` for the replacement logged before `Killing` of the old pod: a terminating pod no longer counts toward replicas | `kubectl -n learn get events --sort-by=.lastTimestamp` |
| Scale 2 → 3 | Event `ScalingReplicaSet … from 2 to 3` | `kubectl -n learn scale deploy web --replicas=3` |
| Image pull | First pull 22 s (54 MB); later pods reuse cached image | events |

Relevance to the analyzer: pod counts can briefly exceed replicas during replacement; findings
should be keyed by workload (Deployment), not by pod name.

## 02 — Service, EndpointSlice, rolling update, readiness vs liveness (2026-09-24)
Manifest: `learn/02-service-probes.yaml` (exec probes on marker files so one pod can be broken on purpose)

| Experiment | Observation | Command |
|---|---|---|
| Change pod template | Rolling update: new RS `web-77765dc8b5` 3/3, old RS kept at 0 (rollback possible); revisions 1, 2 | `kubectl -n learn get rs`, `kubectl -n learn rollout history deploy/web` |
| Service | ClusterIP 10.96.113.85:80 → EndpointSlice with 3 pod IPs on 8080; selection by label | `kubectl -n learn get endpointslices -l kubernetes.io/service-name=web -o wide` |
| Load balancing | 9 calls spread 4/3/2 across pods; one call during the rollout returned empty (likely a terminating pod; UNVERIFIED) | `curl -s --max-time 3 web/hostname` from a pod |
| Readiness fails | Pod 0/1, `ready=false` in EndpointSlice, receives no traffic, **restart count stays 0** | `rm /tmp/ready` |
| Liveness fails | Restart after 13 s (3 × 5 s); events `Unhealthy` + `Killing … failed liveness probe` | `rm /tmp/alive` |
| Termination reason | Liveness kill recorded as `reason: Completed, exitCode: 0` (app exited cleanly on SIGTERM) | `kubectl get pod <p> -o jsonpath='{.status.containerStatuses[0].lastState}'` |

Relevance to the analyzer: R003 counts restart increases, not termination reasons; the *cause* of a
liveness restart is only visible in Events (D-007). Readiness failures cause no restarts at all, so
they need a different signal (unavailable replicas).

## 03 — Requests/limits, QoS, OOM, throttling, scheduling (2026-09-24)
Manifest: `learn/03-resources.yaml` (agnhost `stress` / `pause`; no extra image)

| Experiment | Observation | Command |
|---|---|---|
| QoS classes | BestEffort (none), Burstable (request < limit, or request only), Guaranteed (request = limit) | `kubectl get pods -o custom-columns=NAME:.metadata.name,QOS:.status.qosClass` |
| OOM | 200Mi alloc vs 64Mi limit → `OOMKilled`, exit 137, ~1 s after start; 3 restarts in 49 s, then BackOff | `kubectl get pod oom -o jsonpath='{.status.containerStatuses[0].lastState}'` |
| CPU throttling | limit 200m (`cpu.max 20000 100000`); `nr_throttled == nr_periods` (100% of periods); usage ≈ 217m | `kubectl exec throttled -- cat /sys/fs/cgroup/cpu.stat` |
| Pending (resources) | `FailedScheduling: 1 Insufficient cpu` for request 64 | `kubectl get events --field-selector reason=FailedScheduling` |
| Pending (selector) | `didn't match Pod's node affinity/selector`; schedules once node labelled `disktype=ssd` | `kubectl label node <n> disktype=ssd` |
| Selector after unlabel | Pod keeps running: selectors are checked only at scheduling time | `kubectl label node <n> disktype-` |
| Taint NoSchedule | New pod Pending (`untolerated taint`); existing pods unaffected; schedules after untaint | `kubectl taint node <n> lab=only:NoSchedule[-]` |
| Allocated resources | Node requests 1350m CPU / 546Mi; Pending pods are not counted | `kubectl describe node <n>` |

Relevance to the analyzer: OOM is reliably `OOMKilled` (exit 137), unlike liveness kills (`Completed`,
exit 0); throttling ratio from cgroup counters matches the Prometheus metrics R001 will use;
scheduling reasons exist only in Events; requested totals exclude Pending pods.

## 04 — ConfigMap, Secret, RBAC (2026-09-24)
Manifest: `learn/04-config-secret-rbac.yaml`; Secret created by command (never committed).

| Experiment | Observation | Command |
|---|---|---|
| Secret storage | `Opaque`, data base64-encoded; `base64 -d` recovers it — encoding, not encryption | `kubectl -n learn get secret web-secret -o jsonpath='{.data}'` |
| Pod spec exposure | Literal env value visible in plain text; ConfigMap/Secret values only as references | `kubectl -n learn get pod config-demo -o yaml` |
| ConfigMap update | Mounted file updated after 89 s; env var unchanged until restart | `kubectl -n learn patch configmap web-config ...` |
| RBAC (can-i) | analyzer-readonly: deployments/events/HPAs/nodes yes; pods/secrets/configmaps/delete/create/pods/log no | `kubectl auth can-i <verb> <res> --as=system:serviceaccount:learn:analyzer-readonly` |
| RBAC (real token) | With `KUBECONFIG=/dev/null`: deployments/events allowed; pods, secrets, delete → Forbidden | `kubectl create token analyzer-readonly --duration=10m` |
| Test pitfall | With kubeconfig loaded, the admin client cert was used despite `--token` → false "access granted" | `kubectl auth whoami` first |

Relevance to the analyzer: D-007 ClusterRole verified; RBAC tests must run with an isolated
kubeconfig and assert identity with `kubectl auth whoami`.

## 05 — HPA and the request/utilization coupling (2026-09-24)
Manifest: `learn/05-hpa.yaml` (resource-consumer 1.15.0; load pod sends ~300m total via the Service;
HPA target 70% CPU utilization, min 1, max 5). Needs metrics-server (`deploy/metrics-server`).

| Experiment | Observation | Command |
|---|---|---|
| New pod | First ~45 s: `FailedGetResourceMetric: no metrics returned` / `pods might be unready` | `kubectl -n learn describe hpa hpa-demo` |
| Phase A: request 400m | 300m used → 75% → stays at 1 replica (75/70 = 1.07, within 10% tolerance) | `kubectl -n learn get hpa hpa-demo` |
| Phase B: request → 200m | 150% → `SuccessfulRescale: New size: 3` (ceil(1 × 150/70) = 3; my prediction of 2 was an arithmetic error) | `kubectl -n learn patch deploy hpa-demo ...` |
| After scale-out | 40–62% utilization; stays at 3 (scale-down needs < ~47% and a 5-min stabilization window) | same |
| Totals | CPU requested 400m → **600m**; memory requested 64Mi → **192Mi**; pods 1 → 3 | `kubectl -n learn get deploy hpa-demo` |
| Load spread | Snapshot 190m / 0m / 1m across pods: few requests + random Service routing = uneven pods | `kubectl -n learn top pods -l app=hpa-demo` |

Relevance to the analyzer: lowering a request on an HPA-managed workload *increased* total
allocation here — R001 must emit the HPA-coupled caveat and the HPA-neutral request
(usage / target = 300m / 0.7 ≈ 430m), never a plain resize. Use workload-level aggregates, not
single pods, for HPA-managed workloads. New pods have no metrics at first (R005 case).
