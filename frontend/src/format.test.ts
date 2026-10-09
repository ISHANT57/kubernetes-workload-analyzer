import { describe, expect, it } from 'vitest'
import { formatBytesIEC, formatCoresPlain } from './format'

const MiB = 1024 * 1024

describe('formatBytesIEC (Grafana-style memory units)', () => {
  it('matches the values Grafana showed for the same series', () => {
    expect(formatBytesIEC(36.6 * MiB)).toBe('36.6 MiB')
    expect(formatBytesIEC(30887936)).toBe('29.5 MiB') // 29.46 MiB
    expect(formatBytesIEC(200 * MiB)).toBe('200 MiB')
    expect(formatBytesIEC(26 * MiB)).toBe('26 MiB')
  })

  it('scales through the IEC units', () => {
    expect(formatBytesIEC(0)).toBe('0 B')
    expect(formatBytesIEC(512)).toBe('512 B')
    expect(formatBytesIEC(1536)).toBe('1.5 KiB')
    expect(formatBytesIEC(1.46 * 1024 * MiB)).toBe('1.46 GiB')
    expect(formatBytesIEC(1024 * 1024 * MiB)).toBe('1 TiB')
  })
})

describe('formatCoresPlain (Grafana-style CPU units)', () => {
  it('shows cores with three significant digits', () => {
    expect(formatCoresPlain(0.00328)).toBe('0.00328')
    expect(formatCoresPlain(0.0045)).toBe('0.0045')
    expect(formatCoresPlain(0.1)).toBe('0.1')
    expect(formatCoresPlain(1)).toBe('1')
    expect(formatCoresPlain(1.5)).toBe('1.5')
    expect(formatCoresPlain(0.004185671137354694)).toBe('0.00419')
  })

  it('handles zero and vanishing values without exponent notation', () => {
    expect(formatCoresPlain(0)).toBe('0')
    expect(formatCoresPlain(1e-9)).toBe('0')
  })
})
