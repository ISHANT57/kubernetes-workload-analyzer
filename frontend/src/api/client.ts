// Thin, typed wrapper around the backend's JSON API. Every call goes through this file -- no
// component calls fetch() directly -- so the base path and error handling are defined once.
import type { AnalysisRun, Finding, TimeSeriesResponse } from './types'

// Empty string: same-origin, matching how the backend serves the built frontend in production
// (internal/httpserver would need a static-file handler added for that -- see backend/README.md
// "Not yet wired" note). In dev, Vite's proxy (vite.config.ts) forwards /api to the Go backend.
const BASE = ''

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path)
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new ApiError(res.status, `${path}: ${res.status} ${body}`.trim())
  }
  return res.json() as Promise<T>
}

export function getFindings(): Promise<Finding[]> {
  return getJSON<Finding[]>('/api/findings')
}

export function getLatestRun(): Promise<AnalysisRun> {
  return getJSON<AnalysisRun>('/api/runs/latest')
}

export function getTimeseries(
  namespace: string,
  workload: string,
  container: string,
  metric: 'cpu' | 'memory',
  kind = 'Deployment',
  window = '24h',
): Promise<TimeSeriesResponse> {
  const params = new URLSearchParams({ namespace, workload, container, metric, kind, window })
  return getJSON<TimeSeriesResponse>(`/api/timeseries?${params}`)
}
