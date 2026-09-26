import { NavLink, Outlet } from 'react-router-dom'
import { ClusterSwitcher } from './ClusterSwitcher'
import './Layout.css'

const NAV = [
  { to: '/', label: 'Overview', end: true },
  { to: '/findings', label: 'Findings' },
  { to: '/workloads', label: 'Workloads' },
  { to: '/status', label: 'Platform Status' },
]

export function Layout() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="sidebar-scroll">
          <div className="brand">
            <span className="brand-mark" aria-hidden="true" />
            Workload Analyzer
          </div>
          <nav className="nav">
            {NAV.map((item) => (
              <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}>
                {item.label}
              </NavLink>
            ))}
          </nav>
        </div>
        <div className="sidebar-footer">
          <ClusterSwitcher />
        </div>
      </aside>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
