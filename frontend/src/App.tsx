import { Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Overview } from './pages/Overview'
import { Findings } from './pages/Findings'
import { FindingDetail } from './pages/FindingDetail'
import { Workloads } from './pages/Workloads'
import { Cluster } from './pages/Cluster'
import { Analytics } from './pages/Analytics'
import { PlatformStatus } from './pages/PlatformStatus'
import { NotFound } from './pages/NotFound'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Overview />} />
        <Route path="cluster" element={<Cluster />} />
        <Route path="findings" element={<Findings />} />
        <Route path="findings/:id" element={<FindingDetail />} />
        <Route path="workloads" element={<Workloads />} />
        <Route path="analytics" element={<Analytics />} />
        <Route path="status" element={<PlatformStatus />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}
