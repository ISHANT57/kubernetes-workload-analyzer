import type { Category, Confidence, DataQualityStatus, Severity } from '../api/types'
import './Badges.css'

export function SeverityBadge({ severity }: { severity: Severity }) {
  return <span className={`badge badge-severity-${severity}`}>{severity}</span>
}

export function CategoryTag({ category }: { category: Category }) {
  return <span className={`badge badge-category-${category}`}>{category}</span>
}

export function ConfidenceBadge({ confidence, reason }: { confidence: Confidence; reason?: string }) {
  if (!confidence) {
    return (
      <span className="badge badge-confidence-none" title={reason}>
        n/a
      </span>
    )
  }
  return (
    <span className={`badge badge-confidence-${confidence.toLowerCase()}`} title={reason}>
      {confidence}
    </span>
  )
}

export function DataQualityBadge({ status }: { status: DataQualityStatus }) {
  const label = status === 'ok' ? 'data ok' : status.replace('_', ' ')
  return <span className={`badge badge-dq-${status}`}>{label}</span>
}

export function CaveatBadge({ caveat }: { caveat: string }) {
  return <span className="badge badge-caveat">{caveat}</span>
}
