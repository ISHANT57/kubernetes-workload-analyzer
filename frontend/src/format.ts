// Small, dependency-free formatting helpers shared across pages. Kept centralized so a unit
// (e.g. "cores shown as millicores below 1") is formatted the same way everywhere.

export function formatCores(cores: number): string {
  if (cores === 0) return '0m'
  if (cores < 1) return `${Math.round(cores * 1000)}m`
  return `${cores.toFixed(2)}`
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0'
  const mib = bytes / (1024 * 1024)
  if (mib < 1024) return `${mib.toFixed(0)}Mi`
  return `${(mib / 1024).toFixed(2)}Gi`
}

export function formatUSD(v: number): string {
  return v.toLocaleString(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 2 })
}

export function formatPercent(fraction: number): string {
  return `${Math.round(fraction * 100)}%`
}

export function formatRelativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return iso
  const seconds = Math.round((Date.now() - then) / 1000)
  if (seconds < 5) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.round(hours / 24)}d ago`
}

export function formatDurationMs(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}
