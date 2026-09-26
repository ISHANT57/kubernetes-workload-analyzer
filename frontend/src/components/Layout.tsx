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
    <div className="layout">
      <header className="topbar">
        <div className="brand">Kubernetes Workload Analyzer</div>
        <nav className="nav">
          {NAV.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => (isActive ? 'nav-link active' : 'nav-link')}>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <ClusterSwitcher />
      </header>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
