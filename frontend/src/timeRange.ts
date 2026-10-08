// Time model for the usage charts.
//
// Every timestamp that reaches the browser is a Unix instant in SECONDS (UTC by definition:
// seconds since 1970-01-01T00:00:00Z). Nothing here adds or subtracts an offset. A zone is applied
// only when a time is turned into text for a person, so the same instant can be shown in UTC,
// IST or any other zone without the underlying value changing.

import type { TimeSeriesPoint } from './api/types'

/** The range every usage chart requests. The query and the label both read this one constant,
 * so the label cannot say 24 hours while the request asks for something else. */
export const CHART_WINDOW = '24h'

/** The browser's IANA zone, e.g. "Asia/Kolkata". Falls back to UTC if the runtime hides it. */
export function browserZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

/** "HH:mm" for a Unix-seconds instant in the given IANA zone (24-hour clock). */
export function formatClock(unixSeconds: number, timeZone: string): string {
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone }).format(new Date(unixSeconds * 1000))
}

/** "Asia/Kolkata (UTC+05:30)": the zone and its offset at that instant, so a reader can map the
 * chart's clock to UTC or to Grafana's axis. */
export function describeZone(timeZone: string, unixSeconds: number): string {
  const parts = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' }).formatToParts(new Date(unixSeconds * 1000))
  const raw = parts.find((p) => p.type === 'timeZoneName')?.value ?? 'GMT'
  const offset = raw === 'GMT' ? 'UTC+00:00' : raw.replace('GMT', 'UTC')
  return `${timeZone} (${offset})`
}

/** "24 hours", "1 hour", "90 minutes": a window length in plain words. */
export function describeWindow(seconds: number): string {
  if (seconds % 3600 === 0) {
    const h = seconds / 3600
    return `${h} ${h === 1 ? 'hour' : 'hours'}`
  }
  const m = Math.round(seconds / 60)
  return `${m} ${m === 1 ? 'minute' : 'minutes'}`
}

/** "1h 30m", "45m", "2d 1h": a duration compactly. */
export function describeDuration(seconds: number): string {
  const s = Math.max(0, Math.round(seconds))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return h > 0 ? `${d}d ${h}h` : `${d}d`
  if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`
  return `${m}m`
}

export interface Coverage {
  firstT: number
  lastT: number
  coveredSeconds: number
  requestedSeconds: number
  /** True when the samples span noticeably less than the range that was requested. */
  partial: boolean
}

// Samples sit on a step grid, so a fully covered window spans slightly under its length. Allow
// two steps of slack before calling it partial.
const SLACK_STEPS = 2

/** How much of the requested range actually has samples. null when there are no points. */
export function coverage(points: TimeSeriesPoint[], requestedSeconds: number, stepSeconds: number): Coverage | null {
  if (points.length === 0) return null
  const firstT = points[0].t
  const lastT = points[points.length - 1].t
  const coveredSeconds = lastT - firstT
  return { firstT, lastT, coveredSeconds, requestedSeconds, partial: coveredSeconds < requestedSeconds - SLACK_STEPS * stepSeconds }
}

/** uPlot's columns, in the units it expects for a time axis: x in Unix SECONDS, unchanged. */
export function toColumns(points: TimeSeriesPoint[]): { xs: number[]; ys: number[] } {
  return { xs: points.map((p) => p.t), ys: points.map((p) => p.v) }
}
