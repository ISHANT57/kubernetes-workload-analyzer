import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { useEffect, useRef } from 'react'
import type { TimeSeriesPoint } from '../api/types'
import './UsageChart.css'

interface Props {
  points: TimeSeriesPoint[]
  request: number
  limit: number
  formatValue: (v: number) => string
  color: string
}

/** Wraps uPlot (imperative canvas library, not a React component) in a ref-managed container.
 * This is the "usage over time vs request line" chart -- the evidence behind every R001/R002
 * finding, per the original project brief's "important chart". */
export function UsageChart({ points, request, limit, formatValue, color }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const plotRef = useRef<uPlot | null>(null)

  useEffect(() => {
    const el = containerRef.current
    if (!el || points.length === 0) return

    // uPlot strokes a canvas, where a CSS custom property is not a valid colour -- resolve any
    // var(--x) the caller passes against the live computed style so the chart follows the theme
    // instead of silently drawing nothing.
    const resolve = (c: string): string => {
      const match = /^var\((--[^)]+)\)$/.exec(c.trim())
      if (!match) return c
      const value = getComputedStyle(el).getPropertyValue(match[1]).trim()
      return value || '#6cafff'
    }
    const styles = getComputedStyle(el)
    const mutedStroke = styles.getPropertyValue('--color-text-muted').trim() || '#94a3b8'
    const criticalStroke = styles.getPropertyValue('--color-critical-fg').trim() || '#f87171'

    const xs = points.map((p) => p.t)
    const ys = points.map((p) => p.v)
    const requestLine = xs.map(() => request)

    const series: uPlot.Series[] = [
      {},
      { label: 'usage', stroke: resolve(color), width: 2, points: { show: false } },
      { label: 'request', stroke: mutedStroke, width: 1.5, dash: [5, 4], points: { show: false } },
    ]
    const data: uPlot.AlignedData = [xs, ys, requestLine]

    if (limit > 0) {
      series.push({ label: 'limit', stroke: criticalStroke, width: 1.5, dash: [2, 3], points: { show: false } })
      data.push(xs.map(() => limit))
    }

    // Axis labels and gridlines are canvas-drawn too: uPlot's defaults are tuned for a light
    // background and all but disappear on the dark theme, so both come from our tokens.
    const gridStroke = styles.getPropertyValue('--color-border').trim() || '#e3e7ee'
    const axis = { stroke: mutedStroke, grid: { stroke: gridStroke, width: 1 }, ticks: { stroke: gridStroke } }

    const opts: uPlot.Options = {
      width: el.clientWidth,
      height: 220,
      series,
      scales: { x: { time: true } },
      axes: [
        { ...axis, space: 60 },
        { ...axis, values: (_u: uPlot, vals: number[]) => vals.map(formatValue), size: 70 },
      ],
      cursor: { drag: { x: false, y: false } },
      legend: { show: true },
    }

    const plot = new uPlot(opts, data, el)
    plotRef.current = plot

    const onResize = () => plot.setSize({ width: el.clientWidth, height: 220 })
    window.addEventListener('resize', onResize)

    return () => {
      window.removeEventListener('resize', onResize)
      plot.destroy()
      plotRef.current = null
    }
  }, [points, request, limit, formatValue, color])

  if (points.length === 0) {
    return <div className="chart-empty">No usage data in this window.</div>
  }

  return <div ref={containerRef} className="usage-chart" />
}
