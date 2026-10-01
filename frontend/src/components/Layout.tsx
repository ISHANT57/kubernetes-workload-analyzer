import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { Breadcrumb } from './Breadcrumb'
import { ClusterSwitcher } from './ClusterSwitcher'
import { IconAnalytics, IconFindings, IconOverview, IconPlatform, IconWorkloads } from './Icons'
import './Layout.css'

interface NavItem {
  to: string
  label: string
  end?: boolean
  icon: ReactNode
}

const NAV: NavItem[] = [
  { to: '/', label: 'Overview', end: true, icon: <IconOverview /> },
  { to: '/findings', label: 'Findings', icon: <IconFindings /> },
  { to: '/workloads', label: 'Workloads', icon: <IconWorkloads /> },
  { to: '/analytics', label: 'Analytics', icon: <IconAnalytics /> },
  { to: '/status', label: 'Platform Status', icon: <IconPlatform /> },
]

export function Layout() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true" />
          <span className="brand-name">Workload Analyzer</span>
        </div>
        {/* aria-label/title carry each item's name on narrow viewports, where the label text is
            hidden and the rail becomes icon-only (display:none drops it from the a11y tree). */}
        <nav className="nav">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}
              aria-label={item.label}
              title={item.label}
            >
              <span className="nav-icon">{item.icon}</span>
              <span className="nav-label">{item.label}</span>
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="main">
        <header className="topbar">
          <Breadcrumb />
          <div className="topbar-actions">
            <ClusterSwitcher />
          </div>
        </header>
        <main className="content">
          <div className="content-inner">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
