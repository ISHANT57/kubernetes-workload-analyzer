import { useCallback } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getFindings, getTimeseries } from '../api/client'
import { Card } from '../components/Card'
import { CategoryTag, CaveatBadge, ConfidenceBadge, DataQualityBadge, SeverityBadge } from '../components/Badges'
import { ErrorState, LoadingState } from '../components/Status'
import { UsageChart } from '../components/UsageChart'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatCores, formatPercent, formatRelativeTime, formatUSD } from '../format'
import { grafanaWorkloadUrl } from '../grafana'
import type { Finding, TimeSeriesResponse } from '../api/types'

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
        <Link className="back-link" to="/findings">
          ← Back to findings
        </Link>
        <ErrorState message={`Finding ${id} was not found in the current findings list (it may have resolved, or a new analysis run replaced it).`} />
      </div>
    )
  }

  return <FindingDetailBody finding={finding} />
}

function FindingDetailBody({ finding: f }: { finding: Finding }) {
  return (
    <div>
      <Link className="back-link" to="/findings">
        ← Back to findings
      </Link>
      <h1 className="page-title">
        {f.workload.namespace}/{f.workload.name}
        {f.container ? `/${f.container}` : ''}
      </h1>

      <Card>
        <div className="stat-row" style={{ marginBottom: '0.75rem' }}>
          <SeverityBadge severity={f.severity} />
          <CategoryTag category={f.category} />
          <ConfidenceBadge confidence={f.confidence} reason={f.confidence_reason} />
          <DataQualityBadge status={f.data_quality.status} />
          {(f.caveats ?? []).map((c) => (
            <CaveatBadge key={c} caveat={c} />
          ))}
        </div>
        <p>{f.problem}</p>
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
          <p style={{ color: 'var(--color-text-muted)', fontSize: '0.82rem', marginTop: '0.5rem' }}>
            <strong>Why this confidence:</strong> {f.confidence_reason}
          </p>
        )}
      </Card>

      <Card title="Evidence">
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
                <td>{formatEvidenceValue(e.value, e.unit)}</td>
                <td>
                  <code>{e.query}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {f.cost && (
        <Card title="Estimated cost impact">
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

function UsageChartCard({ finding, metric }: { finding: Finding; metric: 'cpu' | 'memory' }) {
  const fetcher = useCallback(
    () => getTimeseries(finding.workload.namespace, finding.workload.name, finding.container, metric, finding.workload.kind, '24h'),
    [finding.workload.namespace, finding.workload.name, finding.container, finding.workload.kind, metric],
  )
  const series = usePolling<TimeSeriesResponse>(fetcher, 30000)

  return (
    <Card title={`${metric === 'cpu' ? 'CPU' : 'Memory'} usage vs request (last 24h)`}>
      {series.status === 'loading' && <LoadingState label="Loading chart…" />}
      {series.status === 'error' && <ErrorState message={series.error} />}
      {series.status === 'ok' && (
        <UsageChart
          points={series.data.points}
          request={series.data.request}
          limit={series.data.limit}
          formatValue={metric === 'cpu' ? formatCores : formatBytes}
          color={metric === 'cpu' ? '#3b82f6' : '#a855f7'}
        />
      )}
      <p style={{ marginTop: '0.6rem', fontSize: '0.82rem' }}>
        <a href={grafanaWorkloadUrl(finding.workload.namespace, finding.workload.name, finding.workload.kind)} target="_blank" rel="noreferrer">
          Investigate further in Grafana ↗
        </a>
      </p>
    </Card>
  )
}

function formatEvidenceValue(value: number, unit: string): string {
  if (unit === 'cores') return formatCores(value)
  if (unit === 'bytes') return formatBytes(value)
  if (unit === 'percent') return `${value.toFixed(0)}%`
  if (unit === 'count') return value.toFixed(0)
  // For flag-style evidence (e.g. reason=OOMKilled encoded as value=1, unit="OOMKilled"),
  // the unit string itself is the meaningful label -- show it plainly instead of "1 OOMKilled".
  if (value === 1 && unit && Number.isNaN(Number(unit))) return unit
  return `${value} ${unit}`.trim()
}
