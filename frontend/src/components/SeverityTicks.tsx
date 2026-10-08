import type { Severity } from '../api/types'
import './SeverityTicks.css'

interface Part {
  severity: Severity
  label: string
  count: number
}

/** Findings split by severity: a count per column and one tick bar whose segments are
 * proportional to those counts. Same numbers as the KPI tiles; no second source. */
export function SeverityTicks({ parts }: { parts: Part[] }) {
  const total = parts.reduce((n, p) => n + p.count, 0)
  return (
    <div>
      <div className="sev-cols">
        {parts.map((p) => (
          <div className={`sev-col sev-col-${p.severity}`} key={p.severity}>
            <span className="sev-col-label">{p.label}</span>
            <span className="sev-col-value">{p.count}</span>
            <span className="sev-col-share">{total > 0 ? `${Math.round((p.count / total) * 100)}%` : '0%'}</span>
          </div>
        ))}
      </div>
      <div className="sev-bar" role="img" aria-label={parts.map((p) => `${p.count} ${p.label}`).join(', ')}>
        {total === 0 ? (
          <span className="sev-seg sev-seg-empty" style={{ flexGrow: 1 }} />
        ) : (
          parts.filter((p) => p.count > 0).map((p) => <span key={p.severity} className={`sev-seg sev-seg-${p.severity}`} style={{ flexGrow: p.count }} />)
        )}
      </div>
    </div>
  )
}
