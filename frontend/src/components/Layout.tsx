import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { Breadcrumb } from './Breadcrumb'
import { ClusterSwitcher } from './ClusterSwitcher'
import { IconAnalytics, IconFindings, IconOverview, IconPlatform, IconWorkloads } from './Icons'
import { ThemeToggle } from './ThemeToggle'
import './Layout.css'

interface NavItem {
  to: string
  label: string
  end?: boolean
  icon: ReactNode
}

const NAV_GROUPS: { heading: string; items: NavItem[] }[] = [
  {
    heading: 'Overview',
    items: [
      { to: '/', label: 'Dashboard', end: true, icon: <IconOverview /> },
      { to: '/findings', label: 'Findings', icon: <IconFindings /> },
      { to: '/workloads', label: 'Workloads', icon: <IconWorkloads /> },
    ],
  },
  {
    heading: 'Insights',
    items: [
      { to: '/analytics', label: 'Analytics', icon: <IconAnalytics /> },
      { to: '/status', label: 'Platform status', icon: <IconPlatform /> },
    ],
  },
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
          {NAV_GROUPS.map((group) => (
            <div className="nav-group" key={group.heading}>
              <div className="nav-heading">{group.heading}</div>
              {group.items.map((item) => (
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
            </div>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className="sidebar-cluster">
            <span className="sidebar-foot-label">Cluster</span>
            <ClusterSwitcher />
          </div>
          <ThemeToggle />
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <Breadcrumb />
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
