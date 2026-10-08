import { describe, expect, it } from 'vitest'
import { browserZone, coverage, describeDuration, describeWindow, describeZone, formatAxisTick, formatClock, formatDateTime, toColumns } from './timeRange'

// 10:35:00 UTC on 2026-10-08, as the API sends it.
const T_1035Z = 1791455700

describe('one instant, two displays (presentation only)', () => {
  it('10:35 UTC is 16:05 IST', () => {
    expect(formatClock(T_1035Z, 'UTC')).toBe('10:35')
    expect(formatClock(T_1035Z, 'Asia/Kolkata')).toBe('16:05')
  })

  it('IST is UTC+05:30 and the instant itself is never altered', () => {
    expect(describeZone('Asia/Kolkata', T_1035Z)).toBe('Asia/Kolkata (UTC+05:30)')
    expect(describeZone('UTC', T_1035Z)).toBe('UTC (UTC+00:00)')
    const shown = (zone: string) => {
      const [h, m] = formatClock(T_1035Z, zone).split(':').map(Number)
      return h * 60 + m
    }
    expect(shown('Asia/Kolkata') - shown('UTC')).toBe(5 * 60 + 30)
    expect(T_1035Z).toBe(1791455700) // formatting reads the value; it never changes it
  })

  it('other zones round-trip the same way, including a negative offset', () => {
    expect(formatClock(T_1035Z, 'America/New_York')).toBe('06:35') // EDT, UTC-04:00 in October
    expect(describeZone('America/New_York', T_1035Z)).toBe('America/New_York (UTC-04:00)')
  })

  it('reports a usable browser zone', () => {
    expect(browserZone().length).toBeGreaterThan(0)
  })
})

describe('axis and cursor labels (24-hour, like Grafana)', () => {
  it('labels ticks HH:mm in the chosen zone', () => {
    expect(formatAxisTick(T_1035Z, 'UTC')).toBe('10:35')
    expect(formatAxisTick(T_1035Z, 'Asia/Kolkata')).toBe('16:05')
  })

  it('labels local midnight with the date instead of 00:00', () => {
    const utcMidnight = 1791417600 // 2026-10-08T00:00:00Z
    expect(formatAxisTick(utcMidnight, 'UTC')).toBe('10/08')
    expect(formatAxisTick(utcMidnight, 'Asia/Kolkata')).toBe('05:30') // not midnight in IST
    expect(formatAxisTick(utcMidnight - 19800, 'Asia/Kolkata')).toBe('10/08') // IST midnight
  })

  it('formats the cursor time with date and seconds, 24-hour', () => {
    expect(formatDateTime(T_1035Z, 'UTC')).toBe('2026-10-08 10:35:00')
    expect(formatDateTime(T_1035Z, 'Asia/Kolkata')).toBe('2026-10-08 16:05:00')
  })
})

describe('uPlot columns', () => {
  it('keeps timestamps as Unix seconds (not milliseconds, not shifted)', () => {
    const { xs, ys } = toColumns([
      { t: T_1035Z, v: 0.25 },
      { t: T_1035Z + 300, v: 0.5 },
    ])
    expect(xs).toEqual([T_1035Z, T_1035Z + 300])
    expect(ys).toEqual([0.25, 0.5])
    expect(xs[0]).toBeLessThan(1e11) // seconds; milliseconds would be ~1.8e12
  })
})

describe('coverage of the requested range', () => {
  const step = 300
  const dayPoints = (n: number) => Array.from({ length: n }, (_, i) => ({ t: T_1035Z + i * step, v: 1 }))

  it('flags 90 minutes of data against a 24h request as partial', () => {
    const c = coverage(dayPoints(19), 86400, step)
    expect(c?.coveredSeconds).toBe(90 * 60)
    expect(c?.partial).toBe(true)
  })

  it('does not flag a window that spans the request, within two steps of slack', () => {
    const c = coverage(dayPoints(287), 86400, step) // 286 steps = 24h minus two steps
    expect(c?.partial).toBe(false)
  })

  it('returns null with no points', () => {
    expect(coverage([], 86400, step)).toBeNull()
  })
})

describe('wording', () => {
  it('describes windows and durations', () => {
    expect(describeWindow(86400)).toBe('24 hours')
    expect(describeWindow(3600)).toBe('1 hour')
    expect(describeWindow(5400)).toBe('90 minutes')
    expect(describeDuration(5400)).toBe('1h 30m')
    expect(describeDuration(45 * 60)).toBe('45m')
    expect(describeDuration(90000)).toBe('1d 1h')
  })
})
