import './Status.css'

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return <div className="state state-loading">{label}</div>
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="state state-error">
      <strong>Could not load data.</strong>
      <div className="state-detail">{message}</div>
    </div>
  )
}

export function EmptyState({ message }: { message: string }) {
  return <div className="state state-empty">{message}</div>
}

/** Shown when data is being displayed but the last refresh attempt failed -- the data itself may
 * be minutes old. Never hides the data behind this (docs/architecture.md: "keep last snapshot
 * marked stale" is a UI requirement too, not just a backend one). */
export function StaleBanner() {
  return <div className="stale-banner">Showing the last known data — the most recent refresh failed.</div>
}
