import type { ReactNode } from 'react'
import './Kpi.css'

export type KpiTone = 'critical' | 'warning' | 'info' | 'good' | 'neutral'

/** The page's one hero moment (frontend-design guidance: spend boldness in one place). A flat,
 * severity-tinted tile, not another bordered white card -- so the KPI row reads as the headline,
 * and every card below it reads as supporting detail. The tone always comes from real severity
 * counts already on the page, never a decorative color choice. */
export function KpiTile({ value, label, tone = 'neutral' }: { value: ReactNode; label: string; tone?: KpiTone }) {
  return (
    <div className={`kpi-tile kpi-tile-${tone}`}>
      <div className="kpi-value">{value}</div>
      <div className="kpi-label">{label}</div>
    </div>
  )
}

export function KpiRow({ children }: { children: ReactNode }) {
  return <div className="kpi-row">{children}</div>
}
