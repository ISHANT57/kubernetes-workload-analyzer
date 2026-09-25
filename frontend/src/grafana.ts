// Phase 6: links a resource finding to its matching Grafana panel, per docs/requirements.md's
// MVP scope ("each resource finding links to its panel"). Built entirely client-side from data
// already on the Finding -- no backend change needed, since this is just a URL, not a query
// result the browser needs Prometheus access for.
//
// Points at kube-prometheus-stack's own bundled "Kubernetes / Compute Resources / Workload"
// dashboard (uid confirmed via `GET /api/search` against the real Grafana instance, kept as a
// constant since default dashboard UIDs are stable across chart upgrades in practice but not
// guaranteed -- if this ever breaks, `curl -u admin:$PW localhost:3000/api/search` finds the
// current uid). Grafana requires login (no anonymous access, matching the threat model's
// read-only/least-privilege posture) -- this link is not a bypass of that, the user's own
// browser session handles it same as visiting Grafana directly.
const GRAFANA_BASE_URL = import.meta.env.VITE_GRAFANA_URL ?? 'http://localhost:3000'
const WORKLOAD_DASHBOARD_PATH = '/d/a164a7f0339f99e89cea5cb47e9be617/kubernetes-compute-resources-workload'

export function grafanaWorkloadUrl(namespace: string, workload: string, kind: string): string {
  const params = new URLSearchParams({
    'var-datasource': 'default',
    'var-namespace': namespace,
    'var-type': kind.toLowerCase(), // dashboard's own variable values: deployment | statefulset | daemonset
    'var-workload': workload,
  })
  return `${GRAFANA_BASE_URL}${WORKLOAD_DASHBOARD_PATH}?${params}`
}
