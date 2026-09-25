import { Link } from 'react-router-dom'
import { getFindings, getLatestRun } from '../api/client'
import { Card } from '../components/Card'
import { CategoryTag, SeverityBadge } from '../components/Badges'
import { ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatRelativeTime } from '../format'
import type { AnalysisRun, Finding } from '../api/types'

/** Overview answers "is anything wrong?" first, per the project's own design principle
 * (README: "What is wrong?" before "What data exists?"). Everything else is one click away. */
export function Overview() {
  const run = usePolling<AnalysisRun>(getLatestRun, 15000)
  const findings = usePolling<Finding[]>(getFindings, 15000)

  if (run.status === 'loading' || findings.status === 'loading') return <LoadingState label="Loading cluster status…" />
  if (run.status === 'error') return <ErrorState message={run.error} />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const r = run.data
  const fs = findings.data
  const critical = fs.filter((f) => f.severity === 'critical').length
  const warning = fs.filter((f) => f.severity === 'warning').length
  const info = fs.filter((f) => f.severity === 'info').length
  const top = [...fs].slice(0, 5)

  return (
    <div>
      <h1 className="page-title">Cluster Overview</h1>
      {(run.stale || findings.stale) && <StaleBanner />}

      <Card title="Cluster health">
        <div className="stat-row">
          <div className="stat">
            <div className={`stat-value ${critical > 0 ? 'stat-critical' : 'stat-good'}`}>{critical}</div>
            <div className="stat-label">Critical findings</div>
          </div>
          <div className="stat">
            <div className={`stat-value ${warning > 0 ? 'stat-warning' : 'stat-good'}`}>{warning}</div>
            <div className="stat-label">Warnings</div>
          </div>
          <div className="stat">
            <div className="stat-value">{info}</div>
            <div className="stat-label">Info / caveats</div>
          </div>
          <div className="stat">
            <div className="stat-value">{r.workloads_seen}</div>
            <div className="stat-label">Workloads monitored</div>
          </div>
        </div>
      </Card>

      <Card title="Analysis run">
        <div className="stat-row">
          <div className="stat">
            <div className={`stat-value ${r.status === 'complete' ? 'stat-good' : r.status === 'partial' ? 'stat-warning' : 'stat-critical'}`}>{r.status}</div>
            <div className="stat-label">Run status</div>
          </div>
          <div className="stat">
            <div className="stat-value">{formatRelativeTime(r.started_at)}</div>
            <div className="stat-label">Last analysis</div>
          </div>
          <div className="stat">
            <div className="stat-value">{r.duration_ms}ms</div>
            <div className="stat-label">Duration</div>
          </div>
        </div>
        {r.query_errors.length > 0 && (
          <>
            <div className="section-label">Query errors this run</div>
            <ul>
              {r.query_errors.map((e, i) => (
                <li key={i}>
                  <code>{e.source}</code>: {e.message}
                </li>
              ))}
            </ul>
          </>
        )}
      </Card>

      <Card title={`Top findings${fs.length > 5 ? ` (5 of ${fs.length})` : ''}`}>
        {top.length === 0 ? (
          <p>No findings right now. Either everything is right-sized and healthy, or there isn't enough data yet.</p>
        ) : (
          <ul className="finding-list">
            {top.map((f) => (
              <li key={f.id}>
                <Link className="finding-row" to={`/findings/${f.id}`}>
                  <SeverityBadge severity={f.severity} />
                  <CategoryTag category={f.category} />
                  <span className="workload-name">
                    {f.workload.namespace}/{f.workload.name}
                  </span>
                  <span className="problem">{f.problem}</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
        {fs.length > 5 && (
          <p>
            <Link to="/findings">See all {fs.length} findings →</Link>
          </p>
        )}
      </Card>
    </div>
  )
}
