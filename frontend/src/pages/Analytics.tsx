import { Link } from 'react-router-dom'
import { getFindings } from '../api/client'
import { Card } from '../components/Card'
import { KpiRow, KpiTile } from '../components/Kpi'
import { BarBreakdown } from '../components/BarBreakdown'
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

  const bySeverity = [
    { label: 'Critical', count: fs.filter((f) => f.severity === 'critical').length, tone: 'critical' as const },
    { label: 'Warning', count: fs.filter((f) => f.severity === 'warning').length, tone: 'warning' as const },
    { label: 'Info', count: fs.filter((f) => f.severity === 'info').length, tone: 'info' as const },
  ].filter((i) => i.count > 0)

  const byCategory = [
    { label: 'Health', count: fs.filter((f) => f.category === 'health').length, tone: 'neutral' as const },
    { label: 'Resource', count: fs.filter((f) => f.category === 'resource').length, tone: 'neutral' as const },
  ].filter((i) => i.count > 0)

  const ruleIds = Array.from(new Set(fs.map((f) => f.rule_id))).sort()
  const byRule = ruleIds.map((id) => ({ label: RULE_LABEL[id] ?? id, count: fs.filter((f) => f.rule_id === id).length, tone: 'neutral' as const }))

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
  const workloadBar = topWorkloads.map(([label, count]) => ({ label, count, tone: 'neutral' as const }))

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Analytics</h1>
        <p className="page-subtitle">Aggregates computed from the current findings — a snapshot, not a trend over time.</p>
      </div>
      {findings.stale && <StaleBanner />}

      <KpiRow>
        <KpiTile value={fs.length} label="Total findings" hint="Across all rules" tone="neutral" icon={<IconFindings />} />
        <KpiTile value={byWorkload.size} label="Workloads affected" hint="With at least one finding" tone="neutral" icon={<IconCube />} />
        <KpiTile value={priced.length} label="Findings priced" hint={`${fs.length - priced.length} unpriced`} tone="neutral" icon={<IconPulse />} />
        <KpiTile
          value={formatUSD(totalPotentialDifference)}
          label="Est. difference"
          hint="Estimated, never savings"
          tone={totalPotentialDifference > 0 ? 'info' : 'neutral'}
          icon={<IconCoin />}
        />
      </KpiRow>

      <div className="analytics-grid">
        {bySeverity.length > 0 && (
          <Card title="By severity">
            <BarBreakdown items={bySeverity} />
          </Card>
        )}
        {byCategory.length > 0 && (
          <Card title="By category">
            <BarBreakdown items={byCategory} />
          </Card>
        )}
        {byRule.length > 0 && (
          <Card title="By rule">
            <BarBreakdown items={byRule} />
          </Card>
        )}
      </div>

      {workloadBar.length > 0 && (
        <Card
          title="Most findings per workload"
          subtitle={byWorkload.size > topWorkloads.length ? `Top ${topWorkloads.length} of ${byWorkload.size}` : undefined}
        >
          <BarBreakdown items={workloadBar} />
        </Card>
      )}

      {priced.length > 0 && (
        <Card title="Estimated cost impact" subtitle="Summed across every priced finding">
          <div className="cost-box">
            <div className="kv-grid">
              <div className="kv-item">
                <div className="kv-label">Current estimated allocation</div>
                <div className="kv-value">{formatUSD(totalAllocation)}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Optimized estimated allocation</div>
                <div className="kv-value">{formatUSD(totalOptimized)}</div>
              </div>
              <div className="kv-item">
                <div className="kv-label">Potential difference</div>
                <div className="kv-value">{formatUSD(totalPotentialDifference)}</div>
              </div>
            </div>
          </div>
          <p className="note">
            Sum across {priced.length} priced finding{priced.length === 1 ? '' : 's'} ({fs.length - priced.length} unpriced). Each finding's own assumptions are on its{' '}
            <Link to="/findings">detail page</Link>. Reducing a request does not by itself reduce a cloud bill — it only does if the freed capacity lets a node be removed,
            downsized, or not added.
          </p>
        </Card>
      )}
    </div>
  )
}
