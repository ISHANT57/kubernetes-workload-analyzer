import { getLatestRun } from '../api/client'
import { Card } from '../components/Card'
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
      <h1 className="page-title">Platform Status</h1>
      {run.stale && <StaleBanner />}

      <Card title="Latest analysis run">
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
            <div className="kv-label">Status</div>
            <div className="kv-value">{r.status}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Started</div>
            <div className="kv-value">{formatRelativeTime(r.started_at)}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Duration</div>
            <div className="kv-value">{formatDurationMs(r.duration_ms)}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Workloads seen</div>
            <div className="kv-value">{r.workloads_seen}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Findings produced</div>
            <div className="kv-value">{r.findings_count}</div>
          </div>
        </div>
      </Card>

      <Card title="Data source errors this run">
        {r.query_errors.length === 0 ? (
          <p style={{ color: 'var(--color-good-fg)' }}>Prometheus and Kubernetes both answered cleanly.</p>
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
        <p style={{ color: 'var(--color-text-muted)', fontSize: '0.85rem' }}>
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
          (Prometheus self-observability, per the project brief's own "observability of our own platform" requirement)
        </p>
      </Card>
    </div>
  )
}
