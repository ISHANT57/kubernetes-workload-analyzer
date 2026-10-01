import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { getClusters } from '../api/client'
import './Breadcrumb.css'

const PAGE_LABEL: Record<string, string> = {
  '/': 'Overview',
  '/findings': 'Findings',
  '/workloads': 'Workloads',
  '/analytics': 'Analytics',
  '/status': 'Platform Status',
}

/** Wayfinding only -- which cluster, which page. Not a navigation control (the sidebar already
 * is one); this just answers "where am I" at a glance, the way a breadcrumb does in any
 * infrastructure tool. Cluster name reuses the same /api/clusters call the switcher already
 * makes (cheap, same pattern, no shared state introduced for a two-field value). */
export function Breadcrumb() {
  const location = useLocation()
  const [clusterId, setClusterId] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    getClusters()
      .then((data) => {
        if (!cancelled) setClusterId(data.self)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  const page = PAGE_LABEL[location.pathname] ?? (location.pathname.startsWith('/findings/') ? 'Finding Detail' : '')

  return (
    <div className="breadcrumb">
      {clusterId ?? '…'}
      {page && (
        <>
          <span className="breadcrumb-sep">/</span>
          {page}
        </>
      )}
    </div>
  )
}
