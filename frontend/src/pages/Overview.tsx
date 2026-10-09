import { Link } from 'react-router-dom'
import { getFindings, getLatestRun } from '../api/client'
import { Card } from '../components/Card'
import { ClickableRow } from '../components/ClickableRow'
import { CategoryTag, ConfidenceBadge, SeverityBadge } from '../components/Badges'
import { KpiRow, KpiTile } from '../components/Kpi'
import { HealthGauge } from '../components/HealthGauge'
import type { GaugeTone } from '../components/HealthGauge'
import { SeverityTicks } from '../components/SeverityTicks'
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

  const back = { to: '/', label: 'Back to dashboard' }
  const r = run.data
  const fs = findings.data
  const critical = fs.filter((f) => f.severity === 'critical').length
  const warning = fs.filter((f) => f.severity === 'warning').length
  const info = fs.filter((f) => f.severity === 'info').length
  const top = fs.slice(0, 8)
  const attention = fs.filter((f) => f.severity === 'critical' || f.severity === 'warning').slice(0, 3)

  // Share of discovered workloads with no active finding. Deterministic: distinct workloads in
  // `fs` over workloads_seen. Thresholds are stated on the card (90% / 70%), not implied.
  const affected = new Set(fs.map((f) => `${f.workload.namespace}/${f.workload.kind}/${f.workload.name}`)).size
  const hasData = r.workloads_seen > 0
  const healthyFraction = hasData ? Math.max(0, r.workloads_seen - affected) / r.workloads_seen : 0
  const gaugeTone: GaugeTone = healthyFraction >= 0.9 ? 'good' : healthyFraction >= 0.7 ? 'warning' : 'critical'
  const gaugeLabel = !hasData ? 'No data' : healthyFraction >= 0.9 ? 'Healthy' : healthyFraction >= 0.7 ? 'Watch' : 'At risk'

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Cluster overview</h1>
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
        <KpiTile value={info} label="Info and caveats" hint="Context, not problems" tone={info > 0 ? 'info' : 'neutral'} icon={<IconInfo />} />
        <KpiTile value={r.workloads_seen} label="Workloads" hint="Discovered this run" tone="neutral" icon={<IconCube />} />
      </KpiRow>

      <div className="dash-grid">
        <Card title="Findings by severity" subtitle={`${fs.length} active ${fs.length === 1 ? 'finding' : 'findings'} across ${affected} ${affected === 1 ? 'workload' : 'workloads'}`} className="dash-severity">
          <SeverityTicks
            parts={[
              { severity: 'critical', label: 'Critical', count: critical },
              { severity: 'warning', label: 'Warning', count: warning },
              { severity: 'info', label: 'Info', count: info },
            ]}
          />
        </Card>

        <Card title="Workload health" subtitle="Workloads with no active finding" className="dash-gauge">
          <HealthGauge
            fraction={healthyFraction}
            tone={gaugeTone}
            label={gaugeLabel}
            caption={hasData ? `${r.workloads_seen - affected} of ${r.workloads_seen} clear` : 'Nothing discovered yet'}
          />
          <p className="note gauge-note">Healthy at 90% or more, watch at 70% or more, at risk below that.</p>
        </Card>

        <Card title="Needs attention" subtitle="Most severe first, with evidence" className="dash-attention">
          {attention.length === 0 ? (
            <p className="note">Nothing critical or warning-level right now.</p>
          ) : (
            <div className="attention-list">
              {attention.map((f) => (
                <Link key={f.id} to={`/findings/${f.id}`} state={{ back }} className={`attention-card attention-${f.severity}`}>
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

        <Card title="Latest analysis run" className={`dash-run run-${r.status}`}>
          <div className="run-status">{r.status}</div>
          <dl className="run-meta">
            <div>
              <dt>Last run</dt>
              <dd>{formatRelativeTime(r.started_at)}</dd>
            </div>
            <div>
              <dt>Duration</dt>
              <dd>{r.duration_ms}ms</dd>
            </div>
            <div>
              <dt>Workloads</dt>
              <dd>{r.workloads_seen}</dd>
            </div>
          </dl>
          {r.query_errors.length > 0 && (
            <ul className="run-errors">
              {r.query_errors.map((e, i) => (
                <li key={i}>
                  <code>{e.source}</code>: {e.message}
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card
          title="Findings"
          subtitle="Health first, then resource, most severe first"
          className="card--flush dash-table"
          actions={
            <Link className="pill-link" to="/findings">
              {fs.length > top.length ? `See all ${fs.length}` : 'Open findings'}
            </Link>
          }
        >
          {top.length === 0 ? (
            <p className="note">No findings right now. Either everything is right-sized and healthy, or there isn&apos;t enough data yet.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Severity</th>
                  <th>Workload</th>
                  <th>Problem</th>
                  <th>Category</th>
                  <th>Confidence</th>
                </tr>
              </thead>
              <tbody>
                {top.map((f) => (
                  <ClickableRow key={f.id} to={`/findings/${f.id}`} back={back}>
                    <td>
                      <SeverityBadge severity={f.severity} />
                    </td>
                    <td>
                      <Link to={`/findings/${f.id}`} state={{ back }}>
                        {f.workload.namespace}/{f.workload.name}
                      </Link>
                    </td>
                    <td className="truncate" title={f.problem}>
                      {f.problem}
                    </td>
                    <td>
                      <CategoryTag category={f.category} />
                    </td>
                    <td>
                      <ConfidenceBadge confidence={f.confidence} reason={f.confidence_reason} />
                    </td>
                  </ClickableRow>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>
    </div>
  )
}
