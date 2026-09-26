import { Link } from 'react-router-dom'
import type { Finding, Severity } from '../api/types'
import './StatusStrip.css'

const SEVERITY_RANK: Record<Severity, number> = { critical: 3, warning: 2, info: 1 }

interface WorkloadStatus {
  key: string
  label: string
  severity: Severity
  findingId: string
  count: number
}

/** A compact, at-a-glance strip of every workload that currently has a finding, one chip each,
 * colored by its worst severity -- the visual language of a status/uptime strip, but every chip
 * is a real evidence-backed finding, not a synthetic up/down check. Deliberately does not
 * include workloads with no finding: there is no real "is this fine" signal to render for them
 * without guessing (R005 -- never guess through missing data), so they are left out rather than
 * padded in as a fake "OK" chip. */
export function StatusStrip({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) return null

  const byWorkload = new Map<string, WorkloadStatus>()
  for (const f of findings) {
    const key = `${f.workload.namespace}/${f.workload.name}`
    const existing = byWorkload.get(key)
    if (!existing || SEVERITY_RANK[f.severity] > SEVERITY_RANK[existing.severity]) {
      byWorkload.set(key, { key, label: key, severity: f.severity, findingId: f.id, count: (existing?.count ?? 0) + 1 })
    } else {
      existing.count += 1
    }
  }

  const statuses = Array.from(byWorkload.values()).sort((a, b) => SEVERITY_RANK[b.severity] - SEVERITY_RANK[a.severity])

  return (
    <div className="status-strip" role="list" aria-label="Workloads with findings">
      {statuses.map((s) => (
        <Link key={s.key} to={`/findings/${s.findingId}`} className={`status-chip status-chip-${s.severity}`} role="listitem" title={`${s.label}: ${s.count} finding${s.count === 1 ? '' : 's'}, worst is ${s.severity}`}>
          {s.label}
        </Link>
      ))}
    </div>
  )
}
