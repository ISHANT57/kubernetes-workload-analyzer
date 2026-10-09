import './StemChart.css'

export interface Stem {
  label: string
  title: string
  count: number
}

/** A lollipop chart of category counts: one stem per category, the largest (all ties) in the accent
 * colour with a dashed reference line at its value. Heights are count / max, so every stem traces
 * to a real finding count -- no smoothing, no invented axis. */
export function StemChart({ stems, unit }: { stems: Stem[]; unit: string }) {
  const max = Math.max(1, ...stems.map((s) => s.count))
  return (
    <div className="stems" role="img" aria-label={stems.map((s) => `${s.title}: ${s.count}`).join(', ')}>
      <div className="stems-plot" style={{ gridTemplateColumns: `repeat(${stems.length}, 1fr)` }}>
        <div className="stems-ref">
          <span className="stems-ref-tag">
            {max} {unit}
          </span>
        </div>
        {stems.map((s) => (
          <div className={s.count === max ? 'stem stem-top' : 'stem'} key={s.label} title={`${s.title}: ${s.count}`}>
            <span className="stem-line" style={{ height: `${(s.count / max) * 100}%` }} />
            <span className="stem-dot" style={{ bottom: `calc(${(s.count / max) * 100}% - 6px)` }} />
          </div>
        ))}
      </div>
      <div className="stems-labels" style={{ gridTemplateColumns: `repeat(${stems.length}, 1fr)` }}>
        {stems.map((s) => (
          <span key={s.label} title={s.title}>
            {s.label}
          </span>
        ))}
      </div>
    </div>
  )
}
