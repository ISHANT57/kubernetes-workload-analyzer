# Commands

Every command needed to build, run, verify and tear down this project. All of these have been
run against the real setup — none are illustrative. Deeper context lives in
[backend/README.md](../backend/README.md), [docs/runbook.md](runbook.md) (the scripted demo) and
[docs/architecture.md](architecture.md).

**Names used throughout:** kind clusters `workload-analyzer` / `workload-analyzer-2`, their
kubectl contexts `kind-workload-analyzer` / `kind-workload-analyzer-2`, namespaces `analyzer`
(the app), `monitoring` (Prometheus + Grafana), `demo` (the test fixtures), image tag
`k8s-workload-analyzer:local`.

---

## 1. Daily: start an existing setup

The clusters are docker containers. Stopping them keeps all state; starting them back needs no
rebuild or redeploy.

```bash
# bring both clusters back up
docker start workload-analyzer-control-plane workload-analyzer-2-control-plane

# pods need ~1 minute to resync after the node container restarts
kubectl --context kind-workload-analyzer   -n monitoring get pods
kubectl --context kind-workload-analyzer-2 -n monitoring get pods

# port-forwards (each blocks; use & or separate terminals)
kubectl --context kind-workload-analyzer   -n analyzer   port-forward svc/analyzer 8080:8080 &
kubectl --context kind-workload-analyzer-2 -n analyzer   port-forward svc/analyzer 8081:8080 &
kubectl --context kind-workload-analyzer   -n monitoring port-forward svc/kps-grafana 3000:80 &

# verify
curl -s localhost:8080/readyz; echo    # cluster 1  -> {"status":"ready"}
curl -s localhost:8081/readyz; echo    # cluster 2
curl -s -o /dev/null -w '%{http_code}\n' localhost:3000/   # Grafana -> 302 (login redirect)
```

Then open **http://localhost:8080/** (cluster 1) or **http://localhost:8081/** (cluster 2). The
sidebar's cluster dropdown switches between them.

## 2. Stop everything

```bash
pkill -f "kubectl.*port-forward"
docker stop workload-analyzer-control-plane workload-analyzer-2-control-plane
```

Stopping is not deleting — see §8 to actually remove a cluster.

---

## 3. First-time setup (cluster 1)

Requires Docker, [kind](https://kind.sigs.k8s.io/) v0.33+, kubectl, Helm v4.

```bash
# host prerequisite: two kind clusters exceed the default inotify limit and crash-loop
# kube-proxy with "too many open files". Needed once per machine, persisted across reboots.
sudo sysctl -w fs.inotify.max_user_instances=512
sudo sysctl -w fs.inotify.max_user_watches=1048576
printf 'fs.inotify.max_user_instances=512\nfs.inotify.max_user_watches=1048576\n' \
  | sudo tee /etc/sysctl.d/99-kind-multi-cluster.conf

kind create cluster --name workload-analyzer --wait 180s

kubectl --context kind-workload-analyzer apply -k deploy/metrics-server
kubectl --context kind-workload-analyzer create namespace monitoring
kubectl --context kind-workload-analyzer -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="$(openssl rand -base64 24)"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm --kube-context kind-workload-analyzer install kps \
  prometheus-community/kube-prometheus-stack --version 91.5.0 \
  -n monitoring -f deploy/prometheus/values.yaml --wait --timeout 10m

kubectl --context kind-workload-analyzer apply -k demo/                     # test fixtures
kubectl --context kind-workload-analyzer apply -f deploy/rbac/analyzer.yaml # namespace + D-007 RBAC
```

Then build and deploy the app (§5).

> Helm reports "install complete" before the Prometheus StatefulSet the Operator creates is
> actually ready. Wait for `prometheus-kps-kube-prometheus-stack-prometheus-0` to reach `2/2`
> before expecting the analyzer to go `ready` — until then it correctly reports `partial`.

## 4. First-time setup (cluster 2, optional — multi-cluster demo)

Identical, with the second context, plus the cluster-2 Deployment manifest:

```bash
kind create cluster --name workload-analyzer-2 --wait 180s
kubectl --context kind-workload-analyzer-2 apply -k deploy/metrics-server
kubectl --context kind-workload-analyzer-2 create namespace monitoring
kubectl --context kind-workload-analyzer-2 -n monitoring create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="$(openssl rand -base64 24)"
helm --kube-context kind-workload-analyzer-2 install kps \
  prometheus-community/kube-prometheus-stack --version 91.5.0 \
  -n monitoring -f deploy/prometheus/values.yaml --wait --timeout 10m
kubectl --context kind-workload-analyzer-2 apply -k demo/
kubectl --context kind-workload-analyzer-2 apply -f deploy/rbac/analyzer.yaml
```

---

## 5. Build and deploy the app

```bash
# multi-stage build: frontend (node) -> backend (go) -> distroless runtime
docker build -f backend/Dockerfile -t k8s-workload-analyzer:local .

# kind nodes don't see the host's docker images; load it into each
kind load docker-image k8s-workload-analyzer:local --name workload-analyzer
kind load docker-image k8s-workload-analyzer:local --name workload-analyzer-2

# first deploy
kubectl --context kind-workload-analyzer   apply -f deploy/analyzer/deployment.yaml
kubectl --context kind-workload-analyzer-2 apply -f deploy/analyzer/deployment-cluster-2.yaml

# redeploy after a rebuild (same tag, so a restart is what picks it up)
kubectl --context kind-workload-analyzer   -n analyzer rollout restart deploy/analyzer
kubectl --context kind-workload-analyzer-2 -n analyzer rollout restart deploy/analyzer

kubectl --context kind-workload-analyzer   -n analyzer rollout status deploy/analyzer --timeout=90s
kubectl --context kind-workload-analyzer-2 -n analyzer rollout status deploy/analyzer --timeout=90s
```

## 6. Local development (no container)

```bash
# backend needs Prometheus reachable on localhost:9090
kubectl --context kind-workload-analyzer -n monitoring \
  port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090 &

cd backend
CLUSTER_ID=workload-analyzer KUBE_CONTEXT=kind-workload-analyzer ANALYSIS_INTERVAL=30s \
CPU_CORE_HOUR_USD=0.031611 MEMORY_GIB_HOUR_USD=0.004237 PRICE_SOURCE="example only" \
go run ./cmd/analyzer            # API on :8080

# serve the built dashboard from the same binary
(cd ../frontend && npm install && npm run build)
STATIC_DIR=$(pwd)/../frontend/dist go run ./cmd/analyzer

# frontend with hot reload. vite proxies /api to localhost:8081, but the backend defaults to
# 8080 -- so the backend must be started on 8081 for this path:
LISTEN_ADDR=127.0.0.1:8081 CLUSTER_ID=workload-analyzer KUBE_CONTEXT=kind-workload-analyzer \
go run ./cmd/analyzer
# then, in another terminal, from the repo root:
(cd frontend && npm run dev -- --port 5173)
```

## 7. Checks, tests and debugging

```bash
# backend
cd backend
gofmt -l .          # must print nothing
go vet ./...
go test ./...
go build ./...

# frontend
cd frontend
npx tsc -b
npm run build

# what is the analyzer actually seeing? (raw evidence per demo fixture, needs :9090 forwarded)
cd backend && go run ./cmd/verify

# load test / latency percentiles (see docs/performance.md)
go run ./cmd/loadtest -base http://localhost:8080 -concurrency 20 -duration 15s

# the API directly
curl -s localhost:8080/healthz          # liveness, always ok while the process runs
curl -s localhost:8080/readyz           # 200 only when the last run was fully clean
curl -s localhost:8080/api/findings     # ranked findings
curl -s localhost:8080/api/runs/latest  # latest analysis run
curl -s localhost:8080/api/clusters     # this cluster + configured peers
curl -s localhost:8080/metrics | grep ^analyzer_

# logs and pod state
kubectl --context kind-workload-analyzer -n analyzer logs deploy/analyzer --tail=50
kubectl --context kind-workload-analyzer -n analyzer get pods
kubectl --context kind-workload-analyzer -n monitoring get pods
kubectl --context kind-workload-analyzer -n demo get pods

# Grafana opens without a login (anonymous read-only, local demo only, D-010). To edit
# dashboards, log in as `admin` (never an email); this prints the password to your terminal:
kubectl --context kind-workload-analyzer -n monitoring get secret grafana-admin \
  -o jsonpath='{.data.admin-password}' | base64 -d; echo

# D-007 RBAC check with the real ServiceAccount token (expect: no / no / yes / no)
SERVER=$(kubectl --context kind-workload-analyzer config view --minify -o jsonpath='{.clusters[0].cluster.server}')
TOKEN=$(kubectl --context kind-workload-analyzer -n analyzer create token analyzer)
for q in "list pods" "list secrets" "list deployments" "create deployments"; do
  KUBECONFIG=/dev/null kubectl --server="$SERVER" --token="$TOKEN" \
    --insecure-skip-tls-verify=true auth can-i $q -A
done
```

## 8. Teardown

```bash
# remove just the app, keep the clusters
kubectl --context kind-workload-analyzer delete -f deploy/analyzer/deployment.yaml -f deploy/rbac/analyzer.yaml

# remove the demo fixtures
kubectl --context kind-workload-analyzer delete -k demo/

# delete whole clusters (destructive: all monitoring history goes with them)
kind delete cluster --name workload-analyzer
kind delete cluster --name workload-analyzer-2
```

## 9. Failure-path demo (proven in Phase 7)

Prometheus is managed by its Operator: scaling the StatefulSet directly gets reverted within
~52s. Patch the `Prometheus` custom resource instead.

```bash
kubectl --context kind-workload-analyzer -n monitoring patch prometheus \
  kps-kube-prometheus-stack-prometheus --type merge -p '{"spec":{"replicas":0}}'

sleep 40
curl -s localhost:8080/api/runs/latest   # status: partial, findings stay sticky
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/readyz   # 503

kubectl --context kind-workload-analyzer -n monitoring patch prometheus \
  kps-kube-prometheus-stack-prometheus --type merge -p '{"spec":{"replicas":1}}'
# the next analysis cycle recovers on its own — no restart needed
```
