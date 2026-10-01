import { Link } from 'react-router-dom'
import { getFindings } from '../api/client'
import { Card } from '../components/Card'
import { EmptyState, ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatCores, formatUSD } from '../format'
import { grafanaWorkloadUrl } from '../grafana'
import type { Finding } from '../api/types'

function evidenceValue(f: Finding, metric: string): number | undefined {
  return f.evidence.find((e) => e.metric === metric)?.value
}

/** Built entirely from R001/R002 findings already on hand -- no separate backend endpoint. This
 * intentionally shows only workloads with an active resource finding, not a full inventory: a
 * "right-sized, nothing to say" row for every workload would just repeat what Findings already
 * shows as "no finding", not add information. */
export function Workloads() {
  const findings = usePolling<Finding[]>(getFindings, 15000)

  if (findings.status === 'loading') return <LoadingState label="Loading workloads…" />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const resourceFindings = findings.data.filter((f) => f.rule_id === 'R001' || f.rule_id === 'R002')

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Workloads</h1>
        <p className="page-subtitle">
          Only workloads with an active R001 (CPU) or R002 (memory) finding. See <Link to="/findings">Findings</Link> for everything, including health issues.
        </p>
      </div>
      {findings.stale && <StaleBanner />}

      <Card
        title={`${resourceFindings.length} resource finding${resourceFindings.length === 1 ? '' : 's'}`}
        subtitle="Requested vs observed, with an estimated cost difference"
        className="card--flush"
      >
        {resourceFindings.length === 0 ? (
          <div style={{ padding: 'var(--space-5)' }}>
            <EmptyState message="No CPU or memory over-provisioning findings right now." />
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Workload</th>
                <th>Resource</th>
                <th>Requested</th>
                <th>Observed</th>
                <th>Suggested</th>
                <th>Est. potential difference</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {resourceFindings.map((f) => {
                const isCPU = f.rule_id === 'R001'
                const fmt = isCPU ? formatCores : formatBytes
                const requested = evidenceValue(f, isCPU ? 'cpu_request' : 'memory_request')
                const observed = evidenceValue(f, isCPU ? 'cpu_usage_p95' : 'memory_usage_max')
                const suggested = evidenceValue(f, isCPU ? 'cpu_suggested_request' : 'memory_suggested_request')
                return (
                  <tr key={f.id}>
                    <td className="truncate" title={`${f.workload.namespace}/${f.workload.name}`}>
                      <Link to={`/findings/${f.id}`}>
                        {f.workload.namespace}/{f.workload.name}
                      </Link>
                    </td>
                    <td>{isCPU ? 'CPU' : 'Memory'}</td>
                    <td className="num">{requested !== undefined ? fmt(requested) : '—'}</td>
                    <td className="num">{observed !== undefined ? fmt(observed) : '—'}</td>
                    <td className="num">{suggested !== undefined ? fmt(suggested) : '—'}</td>
                    <td className="num">{f.cost ? formatUSD(f.cost.potential_difference_usd) : <span className="muted-cell">not priced</span>}</td>
                    <td>
                      <a href={grafanaWorkloadUrl(f.workload.namespace, f.workload.name, f.workload.kind)} target="_blank" rel="noreferrer">
                        Grafana ↗
                      </a>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  )
}
