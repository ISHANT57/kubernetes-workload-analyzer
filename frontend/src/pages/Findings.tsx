import { useMemo } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import { getFindings } from '../api/client'
import { Card } from '../components/Card'
import { ClickableRow } from '../components/ClickableRow'
import { CategoryTag, ConfidenceBadge, SeverityBadge } from '../components/Badges'
import { EmptyState, ErrorState, LoadingState, StaleBanner } from '../components/Status'
import { usePolling } from '../hooks/usePolling'
import type { Finding } from '../api/types'

export function Findings() {
  const findings = usePolling<Finding[]>(getFindings, 15000)
  // Filters live in the URL, so going to a finding and back (or reloading, or sharing the link)
  // keeps them. An empty value is removed from the URL rather than stored as "".
  const [params, setParams] = useSearchParams()
  const location = useLocation()
  const namespace = params.get('namespace') ?? ''
  const severity = params.get('severity') ?? ''
  const category = params.get('category') ?? ''
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }
  const back = { to: `${location.pathname}${location.search}`, label: 'Back to findings' }

  const namespaces = useMemo(() => {
    if (findings.status !== 'ok') return []
    return Array.from(new Set(findings.data.map((f) => f.workload.namespace))).sort()
  }, [findings])

  if (findings.status === 'loading') return <LoadingState label="Loading findings…" />
  if (findings.status === 'error') return <ErrorState message={findings.error} />

  const filtered = findings.data.filter(
    (f) => (!namespace || f.workload.namespace === namespace) && (!severity || f.severity === severity) && (!category || f.category === category),
  )

  const filters = (
    <div className="filters">
      <select value={namespace} onChange={(e) => setFilter('namespace', e.target.value)} aria-label="Filter by namespace">
        <option value="">All namespaces</option>
        {namespaces.map((ns) => (
          <option key={ns} value={ns}>
            {ns}
          </option>
        ))}
      </select>
      <select value={severity} onChange={(e) => setFilter('severity', e.target.value)} aria-label="Filter by severity">
        <option value="">All severities</option>
        <option value="critical">Critical</option>
        <option value="warning">Warning</option>
        <option value="info">Info</option>
      </select>
      <select value={category} onChange={(e) => setFilter('category', e.target.value)} aria-label="Filter by category">
        <option value="">All categories</option>
        <option value="health">Health</option>
        <option value="resource">Resource</option>
      </select>
    </div>
  )

  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Findings</h1>
        <p className="page-subtitle">Every active finding, each with its evidence, threshold and confidence.</p>
      </div>
      {findings.stale && <StaleBanner />}

      <Card
        title={`${filtered.length} finding${filtered.length === 1 ? '' : 's'}`}
        subtitle={filtered.length === findings.data.length ? 'Showing all' : `Filtered from ${findings.data.length}`}
        actions={filters}
        className="card--flush"
      >
        {filtered.length === 0 ? (
          <div style={{ padding: 'var(--space-5)' }}>
            <EmptyState message={findings.data.length === 0 ? 'No findings right now.' : 'No findings match these filters.'} />
          </div>
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
                <ClickableRow key={f.id} to={`/findings/${f.id}`} back={back}>
                  <td>
                    <SeverityBadge severity={f.severity} />
                  </td>
                  <td>
                    <CategoryTag category={f.category} />
                  </td>
                  <td>
                    <Link to={`/findings/${f.id}`} state={{ back }}>
                      {f.workload.namespace}/{f.workload.name}
                      {f.container ? `/${f.container}` : ''}
                    </Link>
                  </td>
                  <td className="truncate" title={f.problem}>
                    {f.problem}
                  </td>
                  <td>
                    <ConfidenceBadge confidence={f.confidence} reason={f.confidence_reason} />
                  </td>
                </ClickableRow>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  )
}
