import { getLatestRun } from '../api/client'
import { Card } from '../components/Card'
import { KpiRow, KpiTile } from '../components/Kpi'
import { IconClock, IconCube, IconFindings, IconPulse } from '../components/Icons'
import { ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatDurationMs, formatRelativeTime } from '../format'
import type { AnalysisRun } from '../api/types'

/** The project analyzing itself, not just the cluster (original brief §15,
 * "Observability of our own platform"): run cadence, duration, source failures. */
export function PlatformStatus() {
  const run = usePolling<AnalysisRun>(getLatestRun, 10000)

  if (run.status === 'loading') return <LoadingState label="Loading platform status…" />
  if (run.status === 'error') return <ErrorState message={run.error} />

  const r = run.data

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Platform Status</h1>
        <p className="page-subtitle">The analyzer's own health — run cadence, duration and source failures.</p>
      </div>
      {run.stale && <StaleBanner />}

      <KpiRow>
        <KpiTile
          value={r.status}
          label="Run status"
          hint={r.query_errors.length === 0 ? 'All sources answered' : `${r.query_errors.length} query error${r.query_errors.length === 1 ? '' : 's'}`}
          tone={r.status === 'complete' ? 'good' : r.status === 'partial' ? 'warning' : 'critical'}
          icon={<IconPulse />}
        />
        <KpiTile value={formatDurationMs(r.duration_ms)} label="Duration" hint="Last analysis pass" tone="neutral" icon={<IconClock />} />
        <KpiTile value={r.workloads_seen} label="Workloads seen" hint="Deployments, StatefulSets, DaemonSets" tone="neutral" icon={<IconCube />} />
        <KpiTile value={r.findings_count} label="Findings produced" hint="In this run" tone="neutral" icon={<IconFindings />} />
      </KpiRow>

      <Card title="Run details" className="card--quiet">
        <div className="kv-grid">
          <div className="kv-item">
            <div className="kv-label">Run ID</div>
            <div className="kv-value mono" style={{ fontSize: '0.8rem' }}>
              {r.id}
            </div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Cluster</div>
            <div className="kv-value">{r.cluster_id}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Started</div>
            <div className="kv-value">{formatRelativeTime(r.started_at)}</div>
          </div>
        </div>
      </Card>

      <Card title="Data source errors this run" subtitle="Prometheus, Kubernetes and per-workload evidence failures" className={r.query_errors.length > 0 ? 'card--flush' : ''}>
        {r.query_errors.length === 0 ? (
          <p className="note" style={{ color: 'var(--color-good-fg)', margin: 0 }}>
            Prometheus and Kubernetes both answered cleanly.
          </p>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Source</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {r.query_errors.map((e, i) => (
                <tr key={i}>
                  <td>{e.source}</td>
                  <td>
                    <code>{e.message}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Card title="Raw endpoints">
        <p className="note" style={{ margin: 0 }}>
          <a href="/healthz" target="_blank" rel="noreferrer">
            /healthz
          </a>{' '}
          ·{' '}
          <a href="/readyz" target="_blank" rel="noreferrer">
            /readyz
          </a>{' '}
          ·{' '}
          <a href="/metrics" target="_blank" rel="noreferrer">
            /metrics
          </a>{' '}
          · Prometheus self-observability, per the project brief's own "observability of our own platform" requirement.
        </p>
      </Card>
    </div>
  )
}
