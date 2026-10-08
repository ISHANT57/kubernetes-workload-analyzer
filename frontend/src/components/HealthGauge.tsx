import './HealthGauge.css'

const TICKS = 40
const CX = 120
const CY = 118
const R_OUT = 104
const R_IN = 80

export type GaugeTone = 'good' | 'warning' | 'critical'

/** A half-circle gauge drawn as radial ticks: the filled ticks are `fraction` of the arc. The
 * caller supplies the tone and label, so the thresholds that choose them live next to the
 * calculation (Overview), not hidden inside a drawing component. */
export function HealthGauge({ fraction, tone, label, caption }: { fraction: number; tone: GaugeTone; label: string; caption: string }) {
  const clamped = Math.min(1, Math.max(0, fraction))
  const filled = Math.round(clamped * TICKS)
  const ticks = Array.from({ length: TICKS }, (_, i) => {
    const a = Math.PI + (i / (TICKS - 1)) * Math.PI
    const x1 = CX + R_IN * Math.cos(a)
    const y1 = CY + R_IN * Math.sin(a)
    const x2 = CX + R_OUT * Math.cos(a)
    const y2 = CY + R_OUT * Math.sin(a)
    return <line key={i} x1={x1} y1={y1} x2={x2} y2={y2} className={i < filled ? `gauge-tick gauge-tick-${tone}` : 'gauge-tick'} />
  })

  return (
    <figure className="gauge" aria-label={`${label}: ${Math.round(clamped * 100)} percent`}>
      <svg viewBox="0 0 240 128" role="img" aria-hidden="true">
        {ticks}
      </svg>
      <figcaption className="gauge-read">
        <span className="gauge-value">
          {Math.round(clamped * 100)}
          <span className="gauge-unit">%</span>
        </span>
        <span className={`gauge-label gauge-label-${tone}`}>{label}</span>
        <span className="gauge-caption">{caption}</span>
      </figcaption>
    </figure>
  )
}
