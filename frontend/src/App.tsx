import { Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Overview } from './pages/Overview'
import { Findings } from './pages/Findings'
import { FindingDetail } from './pages/FindingDetail'
import { Workloads } from './pages/Workloads'
import { PlatformStatus } from './pages/PlatformStatus'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Overview />} />
        <Route path="findings" element={<Findings />} />
        <Route path="findings/:id" element={<FindingDetail />} />
        <Route path="workloads" element={<Workloads />} />
        <Route path="status" element={<PlatformStatus />} />
      </Route>
    </Routes>
  )
}
