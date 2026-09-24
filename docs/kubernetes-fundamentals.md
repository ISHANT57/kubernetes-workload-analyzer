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
