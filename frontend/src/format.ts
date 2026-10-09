// Small, dependency-free formatting helpers shared across pages. Kept centralized so a unit
// (e.g. "cores shown as millicores below 1") is formatted the same way everywhere.

export function formatCores(cores: number): string {
  if (cores === 0) return '0m'
  // Below 10m a whole number hides real differences (4.2m vs 4.4m), so keep one decimal.
  if (cores < 0.01) return `${(cores * 1000).toFixed(1)}m`
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

/** Renders one EvidenceItem's value using the unit the backend attached to it. Shared by the
 * finding detail page and Overview's attention rail so the same number never formats two ways. */
export function formatEvidenceValue(value: number, unit: string): string {
  if (unit === 'cores') return formatCores(value)
  if (unit === 'bytes') return formatBytes(value)
  if (unit === 'percent') return `${value.toFixed(0)}%`
  if (unit === 'count') return value.toFixed(0)
  if (unit === 'bool') return value === 1 ? 'yes' : 'no'
  // For flag-style evidence (e.g. reason=OOMKilled encoded as value=1, unit="OOMKilled"),
  // the unit string itself is the meaningful label -- show it plainly instead of "1 OOMKilled".
  if (value === 1 && unit && Number.isNaN(Number(unit))) return unit
  return `${value} ${unit}`.trim()
}

// --- Chart units, in the style Grafana draws them -------------------------------------------
// The usage charts sit next to Grafana's panels, so they use the same unit text: memory as IEC
// bytes ("36.6 MiB"), CPU as plain cores ("0.00328"). Tables elsewhere keep Kubernetes notation
// ("200Mi", "57m") because that is how requests and limits are written in manifests.

function trimNumber(n: number, decimals: number): string {
  const fixed = n.toFixed(decimals)
  // Trim trailing zeros only after a decimal point: "1.50" -> "1.5", but "200" stays "200".
  return fixed.includes('.') ? fixed.replace(/\.?0+$/, '') : fixed
}

/** "36.6 MiB", "200 MiB", "1.46 GiB": IEC units with the decimals Grafana shows. */
export function formatBytesIEC(bytes: number): string {
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = Math.abs(bytes)
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const decimals = v < 10 ? 2 : v < 100 ? 1 : 0
  const sign = bytes < 0 ? '-' : ''
  return `${sign}${trimNumber(v, i === 0 ? 0 : decimals)} ${units[i]}`
}

/** Cores as a plain number with three significant digits: "0.00328", "0.0045", "0.1", "1.5". */
export function formatCoresPlain(cores: number): string {
  if (cores === 0 || Math.abs(cores) < 1e-6) return '0'
  return trimNumber(Number(cores.toPrecision(3)), 6)
}
