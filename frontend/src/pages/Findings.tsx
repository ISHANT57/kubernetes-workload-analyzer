import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { getFindings } from '../api/client'
import { CategoryTag, ConfidenceBadge, SeverityBadge } from '../components/Badges'
import { EmptyState, ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import type { Finding } from '../api/types'

export function Findings() {
  const findings = usePolling<Finding[]>(getFindings, 15000)
  const [namespace, setNamespace] = useState('')
  const [severity, setSeverity] = useState('')
  const [category, setCategory] = useState('')

  const namespaces = useMemo(() => {
    if (findings.status !== 'ok') return []
    return Array.from(new Set(findings.data.map((f) => f.workload.namespace))).sort()
  }, [findings])

  if (findings.status === 'loading') return <LoadingState label="Loading findings…" />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const filtered = findings.data.filter(
    (f) => (!namespace || f.workload.namespace === namespace) && (!severity || f.severity === severity) && (!category || f.category === category),
  )

  return (
    <div>
      <h1 className="page-title">Findings</h1>
      {findings.stale && <StaleBanner />}

      <div className="filters">
        <select value={namespace} onChange={(e) => setNamespace(e.target.value)}>
          <option value="">All namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
        <select value={severity} onChange={(e) => setSeverity(e.target.value)}>
          <option value="">All severities</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
        <select value={category} onChange={(e) => setCategory(e.target.value)}>
          <option value="">All categories</option>
          <option value="health">Health</option>
          <option value="resource">Resource</option>
        </select>
      </div>

      {filtered.length === 0 ? (
        <EmptyState message={findings.data.length === 0 ? 'No findings right now.' : 'No findings match these filters.'} />
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th>Severity</th>
              <th>Category</th>
              <th>Workload</th>
              <th>Problem</th>
              <th>Confidence</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((f) => (
              <tr key={f.id}>
                <td>
                  <SeverityBadge severity={f.severity} />
                </td>
                <td>
                  <CategoryTag category={f.category} />
                </td>
                <td>
                  <Link to={`/findings/${f.id}`}>
                    {f.workload.namespace}/{f.workload.name}
                    {f.container ? `/${f.container}` : ''}
                  </Link>
                </td>
                <td>{f.problem}</td>
                <td>
                  <ConfidenceBadge confidence={f.confidence} reason={f.confidence_reason} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
