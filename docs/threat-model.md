# Threat model (Phase 0 draft)

Scope: a local, single-user prototype. Many risks below are low *today* because everything runs on
one laptop. They are written down now so that the design does not quietly assume that.

## Assets
1. Kubernetes API credentials: the local kubeconfig (admin on the dev cluster), and later the backend's ServiceAccount token.
2. Workload metadata: names, images, labels, annotations, node placement, resource settings.
3. Secrets embedded in pod specs (see below).
4. Prometheus data: complete operational history of the cluster.
5. Dashboard output: findings and cost estimates.

## Trust boundaries
```
 browser ──(1)── backend ──(2)── Prometheus ──(3)── kubelet / KSM ──(4)── Kubernetes API
                    └───────────(5, read-only)──────────────────────────────┘
```
(1) is unauthenticated in the MVP (localhost only). (2) and (3) are in-cluster HTTP without auth by
default. (4) is what KSM and Prometheus use via their ServiceAccounts. (5) is the backend's read-only
ClusterRole (D-007).

## What read-only pod access exposes
`get/list/watch` on `pods` returns the **full pod spec and status**, including:
- **Literal `env` values.** Teams often put passwords, API keys or connection strings there.
- `command` / `args`, which sometimes contain tokens.
- The `kubectl.kubernetes.io/last-applied-configuration` annotation, a full copy of the applied manifest.
- Names of referenced Secrets/ConfigMaps (`secretKeyRef`, `envFrom`), but not their values.
- Image names and registries, ServiceAccount names, volumes, host paths, node names, pod IPs.

`pods/log`, `pods/exec` and `secrets` are separate permissions and must never be granted.
Conclusion: "read-only" is **not** "harmless". Pod read access is sensitive.

## Threats and mitigations

| Threat | Mitigation (proposed) |
|---|---|
| Backend ServiceAccount token leaks | Token can only get/list/watch namespaces, nodes, deployments, replicasets, statefulsets, daemonsets, HPAs, events (D-007). No pods (env vars), secrets, configmaps, pods/log, pods/exec, or write verbs. Verify: `kubectl auth can-i --list --as=system:serviceaccount:<ns>:<sa>` |
| Event messages reveal details | Events are readable under D-007. Messages can name images, volumes and referenced Secrets (e.g. failed mounts), never Secret values. Show event messages escaped; don't log them in full |
| Secrets baked into the React bundle | Vite embeds every `VITE_*` env var into the public JS bundle. Never put tokens or URLs with credentials there; the frontend only calls the same-origin backend API |
| Secrets pushed to the public GitHub repo | `.gitignore` covers kubeconfig, `.env`, keys; review `git diff --cached` before each commit. Optional later: a secret scanner in CI (ask first) |
| kube-state-metrics has broad read access: default collectors include `secrets` and `configmaps`, and its ClusterRole grants `list`/`watch` on them (which returns Secret **values** to anyone holding its token) | Set `kube-state-metrics.collectors` to a list without `secrets` and `configmaps`; the chart's RBAC template drops those rules automatically. Verify with `kubectl auth can-i list secrets --as=system:serviceaccount:<ns>:<ksm-sa>` → `no` |
| Labels/annotations leak into metrics | KSM exports no labels/annotations unless allowlisted; allowlist only what rules need |
| Unauthorized dashboard access | MVP binds to `127.0.0.1` or runs behind `kubectl port-forward`. Auth is required before any non-local exposure (Phase 7) |
| Grafana admin credentials | Current chart (grafana-community chart 13.x via kube-prometheus-stack 91.x) generates a random password when `adminPassword` is unset; older chart versions shipped a fixed default. Use `grafana.admin.existingSecret` with a Secret created from `openssl rand` by a script, so the password is stable across `helm upgrade` and never in git |
| Prometheus / Grafana / Alertmanager UIs reachable from the network | ClusterIP services only; access via port-forward; no NodePort/Ingress in MVP |
| kubeconfig committed or logged | `.gitignore` covers kubeconfig, `.env`, keys; the backend never logs config values that could hold credentials |
| Malicious or oversized PromQL results (label injection into HTML) | Escape all label values in HTML output; cap result sizes; per-query timeouts |
| Fixtures consume host resources (DoS on the laptop) | Fixtures have limits; total CPU under ~1 core; OOM/crash pods back off |
| Unrelated containers on the same Docker host | The local cluster must not share networks or volumes with them; do not touch them |

## Open questions
- Authentication approach for non-local use: deferred to Phase 7.
- Whether to add a secret scanner (e.g. gitleaks, free/open source) to CI: decide in Phase 7.
