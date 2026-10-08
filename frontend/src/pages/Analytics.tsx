import { Link } from 'react-router-dom'
import { getFindings } from '../api/client'
import { Card } from '../components/Card'
import { KpiRow, KpiTile } from '../components/Kpi'
import { BarBreakdown } from '../components/BarBreakdown'
import { SeverityTicks } from '../components/SeverityTicks'
import { StemChart } from '../components/StemChart'
import { IconCoin, IconCube, IconFindings, IconPulse } from '../components/Icons'
import { ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatUSD } from '../format'
import type { Finding } from '../api/types'

const RULE_LABEL: Record<string, string> = {
  R001: 'R001 · CPU over-requested',
  R002: 'R002 · Memory over-requested',
  R003: 'R003 · Frequent restarts',
  R004: 'R004 · OOM',
  R006: 'R006 · Unschedulable',
}

/** Every number here is computed client-side from the findings already returned by
 * /api/findings -- no new backend endpoint, no persisted history (D-004 stays not-accepted;
 * this is a snapshot view of the current run, not a trend over time). Still evidence-first per
 * D-006: every count traces to a real finding, and cost totals carry the same "Estimated, never
 * savings" framing as a single finding's cost card. */
export function Analytics() {
  const findings = usePolling<Finding[]>(getFindings, 15000)

  if (findings.status === 'loading') return <LoadingState label="Loading analytics…" />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const fs = findings.data

  if (fs.length === 0) {
    return (
      <div>
        <div className="page-head">
          <h1 className="page-title">Analytics</h1>
          <p className="page-subtitle">Aggregate view of the current findings.</p>
        </div>
        {findings.stale && <StaleBanner />}
        <Card>
          <p className="note">No findings right now, so there is nothing to summarize. Either everything is right-sized and healthy, or there isn't enough data yet.</p>
        </Card>
      </div>
    )
  }

  const byCategory = [
    { label: 'Health', count: fs.filter((f) => f.category === 'health').length, tone: 'neutral' as const },
    { label: 'Resource', count: fs.filter((f) => f.category === 'resource').length, tone: 'neutral' as const },
  ].filter((i) => i.count > 0)

  const ruleIds = Array.from(new Set(fs.map((f) => f.rule_id))).sort()
  const priced = fs.filter((f) => f.cost !== null)
  const totalAllocation = priced.reduce((sum, f) => sum + (f.cost?.allocation_cost_usd ?? 0), 0)
  const totalOptimized = priced.reduce((sum, f) => sum + (f.cost?.optimized_cost_usd ?? 0), 0)
  const totalPotentialDifference = priced.reduce((sum, f) => sum + (f.cost?.potential_difference_usd ?? 0), 0)

  const byWorkload = new Map<string, number>()
  for (const f of fs) {
    const key = `${f.workload.namespace}/${f.workload.name}`
    byWorkload.set(key, (byWorkload.get(key) ?? 0) + 1)
  }
  const topWorkloads = Array.from(byWorkload.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
  const max1 = Math.max(1, ...ruleIds.map((id) => fs.filter((f) => f.rule_id === id).length))
  const workloadBar = topWorkloads.map(([label, count]) => ({ label, count, tone: 'neutral' as const }))

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Analytics</h1>
        <p className="page-subtitle">Aggregates computed from the current findings — a snapshot, not a trend over time.</p>
      </div>
      {findings.stale && <StaleBanner />}

      <div className="dash-grid">
        <KpiRow>
          <KpiTile value={fs.length} label="Total findings" hint="Across all rules" tone="neutral" icon={<IconFindings />} />
          <KpiTile value={byWorkload.size} label="Workloads affected" hint="With at least one finding" tone="neutral" icon={<IconCube />} />
          <KpiTile value={priced.length} label="Findings priced" hint={`${fs.length - priced.length} unpriced`} tone="neutral" icon={<IconPulse />} />
          <KpiTile
            value={formatUSD(totalPotentialDifference)}
            label="Est. potential difference"
            hint="Estimated per month, never savings"
            tone="feature"
            icon={<IconCoin />}
          />
        </KpiRow>

        <Card title="Findings by rule" subtitle="How many findings each rule produced in the current run" className="span-7">
          <StemChart
            unit={max1 === 1 ? 'finding' : 'findings'}
            stems={ruleIds.map((id) => ({ label: id, title: RULE_LABEL[id] ?? id, count: fs.filter((f) => f.rule_id === id).length }))}
          />
          <div className="chip-row">
            {ruleIds.map((id) => (
              <span className="chip" key={id}>
                {RULE_LABEL[id] ?? id} <strong>{fs.filter((f) => f.rule_id === id).length}</strong>
              </span>
            ))}
          </div>
        </Card>

        <Card title="Severity and category" subtitle="Same findings, two ways to split them" className="span-5">
          <SeverityTicks
            parts={[
              { severity: 'critical', label: 'Critical', count: fs.filter((f) => f.severity === 'critical').length },
              { severity: 'warning', label: 'Warning', count: fs.filter((f) => f.severity === 'warning').length },
              { severity: 'info', label: 'Info', count: fs.filter((f) => f.severity === 'info').length },
            ]}
          />
          <div className="chip-row">
            {byCategory.map((c) => (
              <span className="chip" key={c.label}>
                {c.label} <strong>{c.count}</strong>
              </span>
            ))}
          </div>
        </Card>

        {workloadBar.length > 0 && (
          <Card
            title="Most findings per workload"
            subtitle={byWorkload.size > topWorkloads.length ? `Top ${topWorkloads.length} of ${byWorkload.size}` : undefined}
            className="span-7"
          >
            <BarBreakdown items={workloadBar} />
          </Card>
        )}

        {priced.length > 0 && (
          <Card title="Estimated cost impact" subtitle="Summed across every priced finding" className="span-5">
            <dl className="cost-list">
              <div>
                <dt>Current estimated allocation</dt>
                <dd>{formatUSD(totalAllocation)}</dd>
              </div>
              <div>
                <dt>Optimized estimated allocation</dt>
                <dd>{formatUSD(totalOptimized)}</dd>
              </div>
              <div>
                <dt>Potential difference</dt>
                <dd>{formatUSD(totalPotentialDifference)}</dd>
              </div>
            </dl>
            <p className="note">
              Sum across {priced.length} priced finding{priced.length === 1 ? '' : 's'} ({fs.length - priced.length} unpriced). Each finding's own assumptions are on its{' '}
              <Link to="/findings">detail page</Link>. Reducing a request does not by itself reduce a cloud bill; it only does if the freed capacity lets a node be removed,
              downsized, or not added.
            </p>
          </Card>
        )}
      </div>
    </div>
  )
}
