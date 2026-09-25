import type { ReactNode } from 'react'
import './Card.css'

export function Card({ title, children, className }: { title?: string; children: ReactNode; className?: string }) {
  return (
    <section className={`card ${className ?? ''}`}>
      {title && <h2 className="card-title">{title}</h2>}
      {children}
    </section>
  )
}
