import { Link } from 'react-router-dom'
import { getFindings, getLatestRun } from '../api/client'
import { Card } from '../components/Card'
import { CategoryTag, SeverityBadge } from '../components/Badges'
import { KpiRow, KpiTile } from '../components/Kpi'
import { StatusStrip } from '../components/StatusStrip'
import { IconCritical, IconCube, IconInfo, IconWarning } from '../components/Icons'
import { ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatRelativeTime } from '../format'
import { formatEvidenceValue } from '../format'
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
  const top = fs.slice(0, 6)
  const attention = fs.filter((f) => f.severity === 'critical' || f.severity === 'warning').slice(0, 4)

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Cluster Overview</h1>
        <p className="page-subtitle">Ranked, evidence-backed findings from the latest analysis run.</p>
      </div>
      {(run.stale || findings.stale) && <StaleBanner />}

      <KpiRow>
        <KpiTile
          value={critical}
          label="Critical"
          hint={critical > 0 ? 'Needs attention now' : 'Nothing critical'}
          tone={critical > 0 ? 'critical' : 'good'}
          icon={<IconCritical />}
        />
        <KpiTile
          value={warning}
          label="Warnings"
          hint={warning > 0 ? 'Review when you can' : 'No warnings'}
          tone={warning > 0 ? 'warning' : 'good'}
          icon={<IconWarning />}
        />
        <KpiTile value={info} label="Info / caveats" hint="Context, not problems" tone={info > 0 ? 'info' : 'neutral'} icon={<IconInfo />} />
        <KpiTile value={r.workloads_seen} label="Workloads" hint="Discovered this run" tone="neutral" icon={<IconCube />} />
      </KpiRow>

      <div className="split-grid">
        <div>
          <Card
            title={`Top findings${fs.length > top.length ? ` (${top.length} of ${fs.length})` : ''}`}
            subtitle="Health first, then resource, most severe first"
            className="card--flush"
          >
            {top.length === 0 ? (
              <p className="note">No findings right now. Either everything is right-sized and healthy, or there isn't enough data yet.</p>
            ) : (
              <>
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
                {fs.length > top.length && (
                  <Link className="card-footer-link" to="/findings">
                    See all {fs.length} findings
                  </Link>
                )}
              </>
            )}
          </Card>

          {fs.length > 0 && (
            <Card title="Workload health map" subtitle="One square per workload with an active finding, coloured by worst severity">
              <StatusStrip findings={fs} />
            </Card>
          )}
        </div>

        <div>
          <Card title="Needs attention" subtitle="Most severe findings, with their evidence">
            {attention.length === 0 ? (
              <p className="note">Nothing critical or warning-level right now.</p>
            ) : (
              <div className="attention-list">
                {attention.map((f) => (
                  <Link key={f.id} to={`/findings/${f.id}`} className="attention-card">
                    <div className="attention-head">
                      <span className="attention-name">
                        {f.workload.namespace}/{f.workload.name}
                      </span>
                      <SeverityBadge severity={f.severity} />
                    </div>
                    <p className="attention-problem">{f.problem}</p>
                    <dl className="attention-evidence">
                      {f.evidence.slice(0, 2).map((e, i) => (
                        <div className="attention-evidence-row" key={i}>
                          <dt>{e.metric.replace(/_/g, ' ')}</dt>
                          <dd>{formatEvidenceValue(e.value, e.unit)}</dd>
                        </div>
                      ))}
                    </dl>
                  </Link>
                ))}
              </div>
            )}
          </Card>

          <Card title="Analysis run" className="card--quiet">
            <div className="kv-grid">
              <div className="kv-item">
                <div className="kv-label">Status</div>
                <div className={`kv-value ${r.status === 'complete' ? 'stat-good' : r.status === 'partial' ? 'stat-warning' : 'stat-critical'}`}>{r.status}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Last run</div>
                <div className="kv-value">{formatRelativeTime(r.started_at)}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Duration</div>
                <div className="kv-value">{r.duration_ms}ms</div>
              </div>
            </div>
            {r.query_errors.length > 0 && (
              <>
                <div className="section-label">Query errors this run</div>
                <ul className="note">
                  {r.query_errors.map((e, i) => (
                    <li key={i}>
                      <code>{e.source}</code>: {e.message}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Card>
        </div>
      </div>
    </div>
  )
}
