import './CapacityBar.css'

interface Props {
  allocatable: number | null
  requested: number | null
  usage: number | null
  format: (v: number) => string
}

const pct = (part: number, whole: number) => (whole > 0 ? (part / whole) * 100 : 0)

/** One resource's capacity picture on a single scale: the track is what the nodes can give pods
 * (allocatable), the pale bar is what pods asked for (requests), the solid bar is what they
 * actually use. Missing numbers are left out and named, never drawn as zero. */
export function CapacityBar({ allocatable, requested, usage, format }: Props) {
  if (allocatable === null || allocatable <= 0) {
    return <p className="note">Allocatable capacity is not available, so the bar cannot be drawn.</p>
  }
  const reqPct = requested === null ? null : pct(requested, allocatable)
  const usePct = usage === null ? null : pct(usage, allocatable)
  const rows: { cls: string; label: string; value: number | null; share: number | null }[] = [
    { cls: 'cap-dot-use', label: 'In use', value: usage, share: usePct },
    { cls: 'cap-dot-req', label: 'Requested', value: requested, share: reqPct },
    { cls: 'cap-dot-alloc', label: 'Allocatable', value: allocatable, share: 100 },
  ]
  return (
    <div>
      <div className="cap-track" role="img" aria-label={rows.map((r) => `${r.label} ${r.value === null ? 'unavailable' : format(r.value)}`).join(', ')}>
        {reqPct !== null && <span className="cap-req" style={{ width: `${Math.min(100, reqPct)}%` }} />}
        {usePct !== null && <span className="cap-use" style={{ width: `${Math.min(100, usePct)}%` }} />}
      </div>
      <dl className="cap-legend">
        {rows.map((r) => (
          <div key={r.label}>
            <dt>
              <span className={`cap-dot ${r.cls}`} />
              {r.label}
            </dt>
            <dd>
              {r.value === null ? 'unavailable' : format(r.value)}
              {r.share !== null && r.label !== 'Allocatable' && <span className="cap-share"> · {r.share.toFixed(0)}% of allocatable</span>}
            </dd>
          </div>
        ))}
      </dl>
      {reqPct !== null && reqPct > 100 && <p className="note">Requests exceed allocatable capacity, so the bar is capped at 100%.</p>}
    </div>
  )
}
