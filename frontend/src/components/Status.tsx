import { IconCritical, IconInfo, IconPulse } from './Icons'
import './Status.css'

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="state state-loading" role="status">
      <span className="state-icon">
        <IconPulse size={20} />
      </span>
      <span>{label}</span>
    </div>
  )
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="state state-error" role="alert">
      <span className="state-icon">
        <IconCritical size={20} />
      </span>
      <div className="state-text">
        <strong>Could not load data.</strong>
        <div className="state-detail">{message}</div>
      </div>
    </div>
  )
}

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="state state-empty">
      <span className="state-icon">
        <IconInfo size={20} />
      </span>
      <span>{message}</span>
    </div>
  )
}

/** Shown when data is being displayed but the last refresh attempt failed -- the data itself may
 * be minutes old. Never hides the data behind this (docs/architecture.md: "keep last snapshot
 * marked stale" is a UI requirement too, not just a backend one). */
export function StaleBanner() {
  return (
    <div className="stale-banner" role="status">
      Showing the last known data — the most recent refresh failed.
    </div>
  )
}
