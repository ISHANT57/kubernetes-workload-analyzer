import { useEffect, useState } from 'react'
import { getClusters } from '../api/client'
import type { ClustersInfo } from '../api/types'
import './ClusterSwitcher.css'

// Shows which cluster this dashboard is looking at, and lets a person jump to another one if
// PEER_CLUSTERS was configured (docs/architecture.md's multi-cluster note). Switching is a full
// navigation to the peer's own URL, not a fetch -- each analyzer instance only ever talks to its
// own Prometheus/Kubernetes API (D-007), so there is no cross-cluster call here, just a link.
export function ClusterSwitcher() {
  const [info, setInfo] = useState<ClustersInfo | null>(null)

  useEffect(() => {
    let cancelled = false
    getClusters()
      .then((data) => {
        if (!cancelled) setInfo(data)
      })
      .catch(() => {
        // No /api/clusters (older backend) or a transient error: fail silently into "no
        // switcher shown" rather than breaking the rest of the nav bar over a non-essential
        // feature.
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (!info) return null

  if (info.peers.length === 0) {
    return <span className="cluster-badge">{info.self}</span>
  }

  return (
    <select
      className="cluster-switcher"
      value={info.self}
      onChange={(e) => {
        const peer = info.peers.find((p) => p.id === e.target.value)
        if (peer) window.location.href = peer.url
      }}
      aria-label="Switch cluster"
    >
      <option value={info.self}>{info.self} (current)</option>
      {info.peers.map((p) => (
        <option key={p.id} value={p.id}>
          {p.id}
        </option>
      ))}
    </select>
  )
}
