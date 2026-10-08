import { useCallback } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import type { BackState } from '../components/ClickableRow'
import { getFindings, getTimeseries } from '../api/client'
import { Card } from '../components/Card'
import { CategoryTag, CaveatBadge, ConfidenceBadge, DataQualityBadge, SeverityBadge } from '../components/Badges'
import { EmptyState, ErrorState, LoadingState } from '../components/Status'
import { UsageChart } from '../components/UsageChart'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatCores, formatEvidenceValue, formatPercent, formatRelativeTime, formatUSD } from '../format'
import { grafanaWorkloadUrl } from '../grafana'
import { CHART_WINDOW, browserZone, coverage, describeDuration, describeWindow, describeZone, formatClock } from '../timeRange'
import type { Finding, Summary, TimeSeriesResponse } from '../api/types'

const CHART_RULES: Record<string, 'cpu' | 'memory'> = { R001: 'cpu', R002: 'memory' }

export function FindingDetail() {
  const { id } = useParams<{ id: string }>()
  const findings = usePolling<Finding[]>(getFindings, 20000)

  if (findings.status === 'loading') return <LoadingState label="Loading finding…" />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const finding = findings.data.find((f) => f.id === id)
  if (!finding) {
    return (
      <div>
        <BackLink />
        <EmptyState message="This finding is no longer in the current results. It may have been resolved, or a newer analysis run replaced it." />
      </div>
    )
  }

  return <FindingDetailBody finding={finding} />
}

/** Returns to the page the person came from (with its filters), else the findings list. */
function BackLink() {
  const state = useLocation().state as Partial<BackState> | null
  const back = state?.back ?? { to: '/findings', label: 'Back to findings' }
  return (
    <Link className="back-link" to={back.to}>
      ← {back.label}
    </Link>
  )
}

function FindingDetailBody({ finding: f }: { finding: Finding }) {
  return (
    <div>
      <BackLink />
      <div className="page-head">
        <h1 className="page-title">
          {f.workload.namespace}/{f.workload.name}
          {f.container ? `/${f.container}` : ''}
        </h1>
        <p className="page-subtitle">
          {f.rule_id} · {f.workload.kind}
        </p>
      </div>

      {f.summary && <SummaryCard summary={f.summary} />}

      <Card>
        <div className="badge-row" style={{ marginBottom: 'var(--space-4)' }}>
          <SeverityBadge severity={f.severity} />
          <CategoryTag category={f.category} />
          <ConfidenceBadge confidence={f.confidence} reason={f.confidence_reason} />
          <DataQualityBadge status={f.data_quality.status} />
          {(f.caveats ?? []).map((c) => (
            <CaveatBadge key={c} caveat={c} />
          ))}
        </div>
        <p style={{ margin: '0 0 var(--space-4)', fontSize: '0.95rem' }}>{f.problem}</p>
        <div className="kv-grid">
          <div className="kv-item">
            <div className="kv-label">Rule</div>
            <div className="kv-value">{f.rule_id}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Threshold</div>
            <div className="kv-value">{f.threshold}</div>
          </div>
          {f.window && (
            <div className="kv-item">
              <div className="kv-label">Evaluation window</div>
              <div className="kv-value">{f.window}</div>
            </div>
          )}
          <div className="kv-item">
            <div className="kv-label">Data coverage</div>
            <div className="kv-value">{formatPercent(f.data_quality.coverage)}</div>
          </div>
          <div className="kv-item">
            <div className="kv-label">Generated</div>
            <div className="kv-value">{formatRelativeTime(f.generated_at)}</div>
          </div>
        </div>
        {f.confidence_reason && (
          <p className="note">
            <strong>Why this confidence:</strong> {f.confidence_reason}
          </p>
        )}
      </Card>

      <Card title="Evidence" subtitle="Every value below is the output of the query shown beside it" className="card--flush">
        <table className="data-table evidence-table">
          <thead>
            <tr>
              <th>Metric</th>
              <th>Value</th>
              <th>Query</th>
            </tr>
          </thead>
          <tbody>
            {f.evidence.map((e, i) => (
              <tr key={i}>
                <td>{e.metric}</td>
                <td className="num">{formatEvidenceValue(e.value, e.unit)}</td>
                <td>
                  <code>{e.query}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {f.cost && (
        <Card title="Estimated cost impact" subtitle="Estimated under the assumptions listed below — never a quoted saving">
          <div className="cost-box">
            <div className="kv-grid">
              <div className="kv-item">
                <div className="kv-label">Current estimated allocation</div>
                <div className="kv-value">{formatUSD(f.cost.allocation_cost_usd)}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Optimized estimated allocation</div>
                <div className="kv-value">{formatUSD(f.cost.optimized_cost_usd)}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Potential difference</div>
                <div className="kv-value">{formatUSD(f.cost.potential_difference_usd)}</div>
              </div>
            </div>
            <ul className="cost-assumptions">
              {f.cost.assumptions.map((a, i) => (
                <li key={i}>{a}</li>
              ))}
            </ul>
          </div>
        </Card>
      )}

      {CHART_RULES[f.rule_id] && <UsageChartCard finding={f} metric={CHART_RULES[f.rule_id]} />}
    </div>
  )
}

function SummaryCard({ summary: s }: { summary: Summary }) {
  const steps = s.next_steps ?? []
  const notes = s.notes ?? []
  return (
    <Card title="Summary" subtitle="Written from this finding's evidence by fixed rules, the same every time. Not AI." className="summary-card">
      <div className="summary-grid">
        <section>
          <h3 className="summary-heading">What happened</h3>
          <p className="summary-text">{s.what_happened}</p>
        </section>
        <section>
          <h3 className="summary-heading">Why it matters</h3>
          <p className="summary-text">{s.why_it_matters}</p>
        </section>
      </div>
      {steps.length > 0 && (
        <section>
          <h3 className="summary-heading">What to check next</h3>
          <ol className="summary-steps">
            {steps.map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
        </section>
      )}
      {notes.length > 0 && (
        <ul className="summary-notes">
          {notes.map((n, i) => (
            <li key={i}>{n}</li>
          ))}
        </ul>
      )}
    </Card>
  )
}

function UsageChartCard({ finding, metric }: { finding: Finding; metric: 'cpu' | 'memory' }) {
  const fetcher = useCallback(
    () => getTimeseries(finding.workload.namespace, finding.workload.name, finding.container, metric, finding.workload.kind, CHART_WINDOW),
    [finding.workload.namespace, finding.workload.name, finding.container, finding.workload.kind, metric],
  )
  const series = usePolling<TimeSeriesResponse>(fetcher, 30000)

  const grafanaLink = (
    <a href={grafanaWorkloadUrl(finding.workload.namespace, finding.workload.name, finding.workload.kind)} target="_blank" rel="noreferrer">
      Investigate in Grafana ↗
    </a>
  )

  const data = series.status === 'ok' ? series.data : null
  return (
    <Card
      title={`${metric === 'cpu' ? 'CPU' : 'Memory'} usage vs request`}
      subtitle={data ? `Requested: last ${describeWindow(data.window_seconds)}` : undefined}
      actions={grafanaLink}
    >
      {series.status === 'loading' && <LoadingState label="Loading chart…" />}
      {series.status === 'error' && <ErrorState message={series.error} />}
      {data && (
        <>
          <UsageChart
            points={data.points}
            request={data.request}
            limit={data.limit}
            formatValue={metric === 'cpu' ? formatCores : formatBytes}
            color={metric === 'cpu' ? 'var(--color-accent)' : '#a78bfa'}
            tickUnit={metric === 'memory' ? 1024 * 1024 : undefined}
          />
          <TimeCaption data={data} />
        </>
      )}
    </Card>
  )
}

/** States what the chart's clock means and how much of the requested range has data, so the axis
 * can be matched against Grafana (which may be set to UTC) without guessing. */
function TimeCaption({ data }: { data: TimeSeriesResponse }) {
  const cov = coverage(data.points, data.window_seconds, data.step_seconds)
  if (!cov) return null
  const zone = browserZone()
  return (
    <div className="chart-caption">
      {cov.partial && (
        <p>
          Only {describeDuration(cov.coveredSeconds)} of the last {describeWindow(cov.requestedSeconds)} has data: the earliest sample is at {formatClock(cov.firstT, zone)}. The series, or
          Prometheus itself, is newer than the range requested.
        </p>
      )}
      <p>
        Times are shown in {describeZone(zone, cov.lastT)}. Latest sample {formatClock(cov.lastT, zone)} ({formatClock(cov.lastT, 'UTC')} UTC).
      </p>
    </div>
  )
}
