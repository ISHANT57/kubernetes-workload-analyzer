import type { ReactNode } from 'react'
import './Kpi.css'

export type KpiTone = 'critical' | 'warning' | 'info' | 'good' | 'neutral' | 'feature'

interface KpiTileProps {
  value: ReactNode
  label: string
  /** One line under the value: what this number means right now, never a decorative caption. */
  hint?: string
  tone?: KpiTone
  icon?: ReactNode
}

/** A KPI card: label, value, an optional one-line status hint and a tone-coloured icon, in the
 * shape production observability dashboards use. The tone always comes from real severity
 * counts already on the page -- never a decorative colour choice. */
export function KpiTile({ value, label, hint, tone = 'neutral', icon }: KpiTileProps) {
  return (
    <div className={`kpi-tile kpi-tile-${tone}`}>
      <div className="kpi-head">
        <span className="kpi-label">{label}</span>
        {icon && <span className="kpi-icon">{icon}</span>}
      </div>
      <div className="kpi-value">{value}</div>
      {hint && <div className="kpi-hint">{hint}</div>}
    </div>
  )
}

export function KpiRow({ children }: { children: ReactNode }) {
  return <div className="kpi-row">{children}</div>
}
