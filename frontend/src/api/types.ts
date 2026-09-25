// Mirrors backend/internal/model/model.go's JSON shapes exactly. Kept as one file, hand-written
// rather than generated, since the backend's JSON surface is small and stable at this phase --
// if it grows a lot, generating this from the Go structs (e.g. via tygo) becomes worth doing.

export type Severity = 'critical' | 'warning' | 'info'
export type Category = 'health' | 'resource'
export type Confidence = 'HIGH' | 'MEDIUM' | 'LOW' | ''
export type DataQualityStatus = 'ok' | 'insufficient' | 'stale' | 'query_error'
export type RunStatus = 'complete' | 'partial' | 'failed'

export interface WorkloadRef {
  cluster_id: string
  namespace: string
  kind: string
  name: string
}

export interface DataQuality {
  status: DataQualityStatus
  coverage: number // 0..1
  window_seconds: number
  as_of: string // RFC3339
}

export interface EvidenceItem {
  metric: string
  value: number
  unit: string
  query: string
}

export interface CostImpact {
  allocation_cost_usd: number
  optimized_cost_usd: number
  potential_difference_usd: number
  assumptions: string[]
}

export interface Finding {
  id: string
  cluster_id: string
  rule_id: string
  analysis_run_id: string
  generated_at: string
  severity: Severity
  category: Category
  workload: WorkloadRef
  container: string
  problem: string
  evidence: EvidenceItem[]
  threshold: string
  window: string
  data_quality: DataQuality
  confidence: Confidence
  confidence_reason: string
  caveats: string[] | null
  cost: CostImpact | null
}

export interface QueryError {
  source: string
  message: string
}

export interface AnalysisRun {
  id: string
  cluster_id: string
  started_at: string
  duration_ms: number
  status: RunStatus
  workloads_seen: number
  findings_count: number
  query_errors: QueryError[]
}

export interface TimeSeriesPoint {
  t: number // unix seconds
  v: number
}

export interface TimeSeriesResponse {
  metric: 'cpu' | 'memory'
  request: number
  limit: number
  points: TimeSeriesPoint[]
}
