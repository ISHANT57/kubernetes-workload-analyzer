import type { ReactNode } from 'react'
import './Card.css'

interface CardProps {
  /** Panel heading. Omit for an unheaded panel (e.g. the finding-detail summary block). */
  title?: string
  /** One line of context under the title, as the reference dashboards' panel headers use. */
  subtitle?: string
  /** Right-aligned controls in the header row (filters, search, links). */
  actions?: ReactNode
  children: ReactNode
  className?: string
}

export function Card({ title, subtitle, actions, children, className }: CardProps) {
  const hasHeader = Boolean(title || actions)
  return (
    <section className={`card ${className ?? ''}`}>
      {hasHeader && (
        <header className="card-header">
          <div className="card-heading">
            {title && <h2 className="card-title">{title}</h2>}
            {subtitle && <p className="card-subtitle">{subtitle}</p>}
          </div>
          {actions && <div className="card-actions">{actions}</div>}
        </header>
      )}
      <div className="card-body">{children}</div>
    </section>
  )
}
