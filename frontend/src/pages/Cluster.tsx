import { getClusterSummary } from '../api/client'
import { Card } from '../components/Card'
import { CapacityBar } from '../components/CapacityBar'
import { KpiRow, KpiTile } from '../components/Kpi'
import { IconCube, IconPulse, IconWarning } from '../components/Icons'
import { ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatCores, formatRelativeTime } from '../format'
import { grafanaDashboardsUrl } from '../grafana'
import type { ClusterSummary } from '../api/types'

const n = (v: number | null) => (v === null ? '—' : String(Math.round(v)))

/** Cluster-wide capacity and health, read from the same Prometheus data Grafana's cluster
 * dashboards show. Every number comes from one fixed query in the backend; a metric that could
 * not be read shows "—" and is listed under "Missing data", never a zero. */
export function Cluster() {
  const summary = usePolling<ClusterSummary>(getClusterSummary, 30000)

  if (summary.status === 'loading') return <LoadingState label="Loading cluster summary…" />
  if (summary.status === 'error') return <ErrorState message={summary.error} />
  const s = summary.data

  const grafana = (
    <a className="pill-link" href={grafanaDashboardsUrl()} target="_blank" rel="noreferrer">
      Open Grafana ↗
    </a>
  )

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Cluster</h1>
        <p className="page-subtitle">Capacity and health across all nodes, read from Prometheus {formatRelativeTime(s.generated_at)}.</p>
      </div>
      {summary.stale && <StaleBanner />}

      <div className="dash-grid">
        <KpiRow>
          <KpiTile
            value={s.nodes_ready === null || s.nodes === null ? '—' : `${n(s.nodes_ready)}/${n(s.nodes)}`}
            label="Nodes ready"
            hint={s.nodes !== null && s.nodes_ready !== null && s.nodes_ready < s.nodes ? 'Some nodes are not Ready' : 'All reporting nodes are Ready'}
            tone={s.nodes !== null && s.nodes_ready !== null && s.nodes_ready < s.nodes ? 'critical' : 'neutral'}
            icon={<IconCube />}
          />
          <KpiTile
            value={n(s.pods_running)}
            label="Pods running"
            hint={`${n(s.pods_pending)} pending, ${n(s.pods_failed)} failed`}
            tone={(s.pods_pending ?? 0) > 0 || (s.pods_failed ?? 0) > 0 ? 'warning' : 'neutral'}
            icon={<IconPulse />}
          />
          <KpiTile value={n(s.namespaces)} label="Namespaces" hint="Active" tone="neutral" icon={<IconCube />} />
          <KpiTile
            value={n(s.restarts_24h)}
            label="Restarts, 24h"
            hint="Container restarts, all pods"
            tone={(s.restarts_24h ?? 0) >= 1 ? 'warning' : 'neutral'}
            icon={<IconWarning />}
          />
        </KpiRow>

        <Card title="CPU" subtitle="Cores: what nodes offer, what pods asked for, what they use" className="span-6">
          <CapacityBar allocatable={s.cpu_allocatable_cores} requested={s.cpu_requested_cores} usage={s.cpu_usage_cores} format={(v) => (v >= 1 ? `${formatCores(v)} cores` : formatCores(v))} />
        </Card>
        <Card title="Memory" subtitle="Working set versus requests and allocatable" className="span-6">
          <CapacityBar allocatable={s.mem_allocatable_bytes} requested={s.mem_requested_bytes} usage={s.mem_usage_bytes} format={formatBytes} />
        </Card>

        <Card title="What stands out" subtitle="Plain readings of the numbers above, with their thresholds" className="span-7" actions={grafana}>
          {s.highlights.length === 0 ? (
            <p className="note" style={{ marginTop: 0 }}>
              Nothing crossed a threshold. Nodes are Ready, no pods are Pending or Failed, and requests are neither near capacity nor far above use.
            </p>
          ) : (
            <ul className="highlight-list">
              {s.highlights.map((h, i) => (
                <li key={i}>{h}</li>
              ))}
            </ul>
          )}
        </Card>

        <Card title="Missing data" subtitle="Metrics that could not be read this time" className="span-5">
          {s.errors.length === 0 ? (
            <p className="note" style={{ marginTop: 0 }}>
              Every metric was read.
            </p>
          ) : (
            <ul className="highlight-list">
              {s.errors.map((e, i) => (
                <li key={i}>{e}</li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  )
}
