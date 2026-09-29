import './BarBreakdown.css'

export interface BarItem {
  label: string
  count: number
  tone?: 'critical' | 'warning' | 'info' | 'good' | 'neutral'
}

/** A proportional bar per category -- every bar's width and count trace straight back to the
 * findings already on the page, nothing computed elsewhere or guessed. No chart library: these
 * are simple category counts, not a time series, so a plain div bar is the honest, minimal
 * representation rather than reaching for a charting dependency this doesn't need. */
export function BarBreakdown({ items }: { items: BarItem[] }) {
  const max = Math.max(1, ...items.map((i) => i.count))
  return (
    <div className="bar-breakdown">
      {items.map((item) => (
        <div className="bar-row" key={item.label}>
          <div className="bar-label">{item.label}</div>
          <div className="bar-track">
            <div className={`bar-fill bar-fill-${item.tone ?? 'neutral'}`} style={{ width: `${(item.count / max) * 100}%` }} />
          </div>
          <div className="bar-count">{item.count}</div>
        </div>
      ))}
    </div>
  )
}
