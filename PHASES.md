# Phases

Status values: `NOT STARTED` · `IN PROGRESS` · `WAITING FOR DECISION` · `DONE`

| # | Phase | Goal | Done when | Status |
|---|---|---|---|---|
| 0 | Foundation | Environment check, git, docs, decisions | D-001..D-007 recorded | DONE (D-004 still PROPOSED) |
| 1 | Kubernetes + monitoring | Install kubectl/kind/helm; create 1-node kind cluster; hands-on fundamentals (Pod, Deployment, ReplicaSet, Service, Namespace, ConfigMap, Secret, requests/limits, probes, labels/selectors, Events, RBAC, HPA, scheduling, failure/recovery); **install trimmed kube-prometheus-stack at the end so history starts accumulating** | Every concept has an entry in `docs/kubernetes-fundamentals.md` (experiment, observation, command); Prometheus footprint MEASURED; all metrics in the measurement model return data; KSM cannot list secrets. Time limit: ~2 weeks | DONE |
| 2 | Demo workloads | Deterministic workloads in `demo/`, each mapped to an expected result in `demo/expected-findings.yaml` | Each workload shows its intended behaviour in Prometheus | DONE |
| 3 | Backend skeleton | Go module: config, Prometheus client, Kubernetes client (D-007 scope), analysis-run object, `/healthz` `/readyz` `/metrics`, structured logs | Runs against the cluster; degrades cleanly with Prometheus down | DONE |
| 4 | Findings (R001–R005) | Evidence builder, rule engine, finding model, stable IDs, cost calculator, REST API | Output matches `expected-findings.yaml` | DONE (KRR comparison deferred) |
| 5 | Dashboard | React + Vite + TS + uPlot on recorded real output | Overview, Findings, Detail, Workloads, Platform Status pages | DONE |
| 6 | Grafana integration | Links from findings to Grafana panels | Each resource finding links to its panel | DONE |
| 7 | Production engineering | Retries, idempotency, partial-failure handling, structured errors, RBAC hardening, in-cluster deploy; tracing only if a need appears | Failure tests pass (Prometheus down, partial query failure, duplicate run, insufficient history) | DONE |
| 8 | Final demonstration | Scripted demo covering the 8 points of the brief | Demo runbook in `docs/runbook.md` | DONE |

## Log
- 2026-09-23: Environment inspected; git initialized; docs created; decisions proposed.
- 2026-09-23: Environment re-measured after owner cleanup (75 GB free, 0 containers running).
- 2026-09-24: Owner accepted D-001, D-002, D-003, D-005, D-006, D-007. Phases reordered: Prometheus installed at end of Phase 1.
- 2026-09-24: Phase 1 done. kind v1.37.0 cluster; labs 01–05; metrics-server v0.9.0; kube-prometheus-stack 91.5.0 (~580 MiB pods).
  All measurement-model metrics verified with real conditions. `container_oom_events_total` stayed 0 through real OOMs → not used.
  KSM secrets access removed and verified; Grafana auth verified. Cluster-level recovery (Docker restart / reboot) moved to Phase 7.
- 2026-09-25: 6 ADRs written (D-001,002,003,005,006,007) with Phase 1 evidence; branches cleaned up.
- 2026-09-25: Phase 2 done. 9 demo/ workloads (agnhost/resource-consumer, no new image pulls) verified live
  against Prometheus: OOMKilled/137, CrashLoopBackOff/exit 1, Pending/Insufficient cpu, HPA object present,
  duty-cycled CPU and held memory all confirmed in expected ranges. `demo/expected-findings.yaml` is the
  ground truth for the Phase 4 integration test.
- 2026-09-25: Phase 3 done. Go 1.27 module (`backend/`): config, structured logging, Prometheus client,
  Kubernetes client scoped to D-007, analysis loop with per-source timeouts and a sticky in-memory snapshot,
  `/healthz` `/readyz` `/metrics` `/api/runs/latest`. All unit tests pass with fakes (no live cluster needed);
  additionally run live against the real cluster: counted 16 deployments correctly, then Prometheus access
  was killed and restored live -- `/readyz` correctly went 503 and recovered, `workloads_seen` stayed at 16
  throughout (carried forward, never dropped to 0), `/healthz` stayed unaffected, graceful SIGTERM shutdown
  confirmed. `deploy/rbac/analyzer.yaml` written (not yet applied; in-cluster deployment is Phase 7).
- 2026-09-26: Phase 4 done. Rule engine (R001-R004; R005 is the data-quality gate embedded in
  evidence.Classify, not a separate rule): evidence builder (Prometheus-only, pod resolution via
  KSM owner-chain join -- D-007 grants no `pods` access), self-calibrated coverage (an assumed
  scrape interval was checked live and found >2x wrong, so coverage comes from actual sample
  timestamps instead), cost estimator (explicit `Estimated`, never "savings", no built-in default
  price), findings orchestrator (stable IDs, ranking), `/api/findings`. All unit tests pass with
  synthetic evidence matching all 9 demo/ scenarios; additionally run live end-to-end against the
  real cluster with real pricing configured -- output matched `demo/expected-findings.yaml`
  exactly for all 9 scenarios, plus found a genuine, non-fixture finding on
  `kube-system/metrics-server` (confirms the rules generalize beyond the demo fixtures). Verified
  idempotent (same finding IDs across independent runs) and graceful degradation with real
  pricing wired in (Prometheus killed and restored live: findings stayed sticky, `/readyz` 503'd
  and recovered, SIGTERM shutdown clean). 3 real bugs found via live verification and fixed, not
  just made to pass a test written after the fact -- see docs/requirements.md §3 implementation
  notes: a `rate()` window that silently defeated the bursty guard, a p50=0 case that inverted
  the burstiness signal, and a misleading confidence-reason message. KRR comparison deferred to a
  later phase (not blocking; no demo fixture needs it).
- 2026-09-26: Phase 5 done. React + Vite + TS dashboard (`frontend/`): Overview, Findings (filterable),
  Finding Detail (full evidence, cost, and a real uPlot usage-vs-request chart for R001/R002),
  Workloads, Platform Status. New small backend addition: GET /api/timeseries (reuses the evidence
  builder's existing Prometheus queries; fixed, parameterized -- the browser still never sends
  PromQL). Backend now also serves the built frontend directly (STATIC_DIR + SPA fallback) as one
  deployable binary. Verified live with a real backend/cluster via Playwright, not just built and
  assumed: desktop, mobile (390px), and dark mode all checked with zero console errors; one real
  mobile CSS bug (long PromQL text overflowing the evidence table) found and fixed the same way.
- 2026-09-26: Phase 6 done. Resource findings (R001/R002) and the Workloads page link to
  kube-prometheus-stack's bundled "Kubernetes / Compute Resources / Workload" Grafana dashboard,
  pre-filtered to the exact namespace/workload/type via its own template variables (confirmed via
  the real Grafana API, not guessed). Built entirely client-side (frontend/src/grafana.ts) -- no
  backend change needed, since it's a URL, not a query result. Grafana's existing login
  requirement is left as-is (no anonymous-access change), matching the threat model's
  read-only/least-privilege posture.
- 2026-09-26: Phase 7 done. `backend/Dockerfile` (new): multi-stage build, `node:24-alpine` for the
  frontend then `golang:1.27-alpine` (`CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`) onto
  `gcr.io/distroless/static-debian12:nonroot` -- 45.7 MB image, no shell, no package manager.
  `deploy/rbac/analyzer.yaml` gained a dedicated `analyzer` namespace (ServiceAccount moved out of
  `default`); D-007 ClusterRole/ClusterRoleBinding content unchanged. `deploy/analyzer/deployment.yaml`
  (new): Deployment + Service, `runAsNonRoot: true` with explicit `runAsUser`/`runAsGroup: 65532`
  (distroless's `nonroot` user is a name, not a UID -- kubelet cannot verify non-root without one,
  hit as a real `CreateContainerConfigError` first), `allowPrivilegeEscalation: false`,
  `readOnlyRootFilesystem: true`, all capabilities dropped, `seccompProfile: RuntimeDefault`.
  Resource requests/limits measured live with `kubectl top` (~1-18m CPU / 10Mi memory observed)
  then set with headroom (`25m`/`32Mi` request, `128Mi` memory limit), not guessed. Verified live,
  in-cluster, twice: (1) normal path -- pod `Running 1/1`, `/readyz` 200, `/api/findings` returns
  real findings through the ClusterIP Service, D-007 RBAC re-verified with the real in-cluster
  ServiceAccount token (`kubectl auth whoami`, isolated `KUBECONFIG=/dev/null`) confirming no
  `pods`/`secrets`/write access; (2) failure path -- Prometheus taken down by patching its own
  Operator-managed CR to `replicas: 0` (a direct StatefulSet scale-down was found to be silently
  reverted by the operator within ~52s, a real methodology bug caught by a false-negative result,
  fixed by using the CR instead) produced a genuine in-cluster DNS-path connection-refused error,
  `status: partial`, sticky findings (stayed at 3, never dropped), `/readyz` 503; restoring the CR
  to `replicas: 1` brought Prometheus back and the analyzer's next cycle returned to `status:
  complete` and `/readyz` 200 on its own, no restart needed. `docs/threat-model.md` and
  `docs/architecture.md` updated with the real in-cluster deployment shape and RBAC verification
  detail.
- 2026-09-26: Phase 8 done. `docs/runbook.md`: one section per demonstrable success criterion from
  the original brief (§24 items 1-8), each with real commands run against the actual in-cluster
  analyzer, not invented output. Writing it surfaced a real, live observation, not a fabricated
  one: right after the Phase 7 outage test, `demo/cpu-over-requested` and `demo/memory-over-requested`
  showed `data_quality: insufficient` ("less than 80% coverage over the minimum 30m window") instead
  of firing R001/R002 -- confirmed via `cmd/verify` -- because the deliberate Prometheus outage had
  just created a real gap in that window. This is R005 (the data-quality gate) working as designed,
  not a bug: it self-clears as the gap ages out of the 30-minute lookback. The runbook documents
  this live rather than papering over it, and the cost-impact section is worked out from the real
  evidence and the real deployed formula (`internal/findings/findings.go`, `CostHours = 730`)
  rather than showing a fabricated API response, per AGENTS.md's "never invent metrics" rule.
- 2026-09-26: Post-Phase-8 gap closure (not a new phase; three documentation/testing gaps found
  during an honest completion review against the original brief). `backend/cmd/loadtest/main.go`
  (new, stdlib only): concurrent GET load generator, the brief §17 "basic load test". Run live
  against the in-cluster analyzer: 5 endpoints, 20 concurrent workers, 15s each, ~212k total
  requests, zero errors. `docs/performance.md` (new): the real measured results --
  `/healthz`/`/readyz`/`/api/runs/latest`/`/api/findings` at 3-6ms p50 / 15-20ms p99;
  `/api/timeseries` at 47ms p50 / 113ms p99 (queries Prometheus live per request, by design, unlike
  the other endpoints which read the in-memory snapshot); a genuine finding that CPU usage spiked
  to 1614m under this load against a 25m request with no CPU limit set (no restart, no OOM,
  documented as a recommendation, not silently applied); and the real
  `analyzer_analysis_run_duration_seconds` histogram from `/metrics` (29 runs, mean 0.643s, all
  under 1s). `docs/reports/phase-3-report.{html,pdf}` through `phase-8-report.{html,pdf}` (new):
  the PDF write-up convention from Phases 1-2 had lapsed for Phases 3-8; backfilled from the
  already-verified facts in this file and `docs/*`, not reconstructed from memory, in the same
  template.
- 2026-09-26: D-008 multi-cluster switcher (owner-requested, for a manager demo showing 2+
  clusters monitored). New `GET /api/clusters` endpoint (`self` + `PEER_CLUSTERS`-configured
  peers, link metadata only -- no analyzer instance ever calls another one); dashboard top-bar
  switcher navigates to a peer's own URL. Verified with two real, separately-deployed kind
  clusters (`workload-analyzer`, `workload-analyzer-2`), each with its own full
  kube-prometheus-stack, demo workloads and analyzer Deployment -- not one cluster split into two
  views. Confirmed live via Playwright: switching works both directions, zero console errors
  either side, each `/api/clusters` correctly self-identifies and lists the other as a peer. A
  real host limit was hit and fixed along the way: two kind clusters exhausted
  `fs.inotify.max_user_instances` (128 default; cluster 1 alone used 78), crash-looping cluster
  2's `kube-proxy` with "too many open files" -- raised to 512 and persisted
  (`/etc/sysctl.d/99-kind-multi-cluster.conf`), a documented laptop constraint
  (`docs/architecture.md`), not an application bug. Combined single-list aggregation across
  clusters deliberately not built (see D-008) until there's a concrete need for it.
- 2026-09-26: Dashboard visual pass (owner-requested: adopt reference-dashboard visual
  language -- spacing, KPI cards, status strips, hierarchy, polish -- while preserving D-006's
  information architecture, not copying a raw-metrics/uptime-grid layout). New
  `components/Kpi.tsx` (flat, severity-tinted hero tiles, one bold moment per page instead of
  another bordered card) and `components/StatusStrip.tsx` (compact colored chips, one per
  workload with an active finding, each linking to its real evidence -- deliberately excludes
  workloads with no finding rather than padding in a fake "OK" chip, since there is no real
  per-workload health signal to show without one, per R005's never-guess rule). Overview's
  "Analysis run" card demoted to a quieter, dashed, transparent treatment so it doesn't compete
  with the KPI hero above it. Verified live via Playwright against real findings data (desktop
  light, dark, 390px mobile): zero console errors, all three renders confirmed by screenshot.
- 2026-09-26: Bug fix -- partial workload-evidence failures were reported as `status: complete`.
  `findings.Analyzer.Analyze()` logged a per-workload evidence-gathering failure (a broken
  pod-resolution query, a failed container listing) but never returned it; `internal/runner`
  only ever saw the two top-level Prometheus/Kubernetes connectivity checks, so a run where
  Prometheus and Kubernetes were both reachable but some individual workload's evidence query
  failed still reported `complete` -- indistinguishable from a fully clean run, and invisible to
  `/readyz`. This contradicted `docs/architecture.md`'s own already-documented failure table
  ("One query fails: only that workload gets query_error; the rest of the run continues") --
  the fix makes the real behavior match what was already documented, not a new architecture
  decision. `Analyze()` now returns `[]model.QueryError` alongside its findings; `runner.runOnce`
  folds them into the run's `QueryErrors` and a revised `statusFor` that tracks source failures
  (prometheus/kubernetes, `failed` threshold, unchanged) separately from workload failures
  (`partial` threshold, new) -- a workload-level failure can never by itself push a run to
  `failed`, since both top-level sources being reachable means some usable data still exists.
  4 new tests in `internal/findings` (all workloads succeed -> no errors; some workload fails ->
  error returned for that workload only, source=`evidence`; successful workload's finding still
  present; error messages identify the workload without leaking anything unsafe) and 4 in
  `internal/runner` (partial workload failure -> `status: partial` with an `evidence`-source
  `QueryError`; successful workload's finding preserved in `Findings()`; all-succeed ->
  `complete`; a full Prometheus outage still reports the pre-existing `partial` behavior
  unchanged, confirming no regression). The fix was proven against the real bug, not just
  written to pass a test written after the fact: reverting the runner-side wiring while keeping
  the new tests made `TestRunOnce_PartialWorkloadFailure_ReportsPartialNotComplete` fail exactly
  as expected, then passed again once restored. No change to rules, thresholds, cost semantics,
  `/readyz`'s own logic, the frontend, or any new infrastructure -- `/readyz` behaves correctly
  now only because the `Status` it reads is finally accurate. Rebuilt and redeployed to both live
  clusters; both confirmed healthy post-deploy.
- 2026-09-26: PR18 (open, not merged) -- workload discovery coverage. D-007's ClusterRole has
  granted `get/list/watch` on `statefulsets`/`daemonsets` since Phase 0, but the code only ever
  listed Deployments (`k8sclient.ListDeployments`) and only ever resolved pods through the
  Deployment-specific two-hop ReplicaSet join -- StatefulSets and DaemonSets were invisible to
  the analyzer even though RBAC already allowed seeing them. `k8sclient.ListWorkloads` (renamed
  from `ListDeployments`) now lists all three kinds, merged into one `WorkloadsSeen` count; a
  failure listing any one kind still fails the whole call, so this stays one "kubernetes"
  top-level source, not three (no change to the runner's failed/partial accounting).
  `evidence.resolvePodNames` now branches on `Kind`: Deployment keeps the existing
  `kube_pod_owner`→`kube_replicaset_owner` join; StatefulSet/DaemonSet use a direct
  `kube_pod_owner{owner_kind=...}` query instead, since they own pods directly and the two-hop
  join would structurally never match for them (a StatefulSet/DaemonSet never creates a
  ReplicaSet); an unrecognized Kind fails closed with an error rather than silently returning
  zero pods. 9 new tests (`internal/k8sclient`: all three kinds discovered and correctly tagged,
  empty cluster, one kind's listing failing fails the whole call; `internal/evidence`: the exact
  query shape issued per Kind, an unsupported Kind failing before querying, `ResolveContainers`
  working end-to-end for both new kinds) -- proven against the real bug the same way as the
  partial-workload-failure fix above: temporarily routing all three kinds through the old
  Deployment-only join reproduced exactly the failure the new StatefulSet/DaemonSet tests exist
  to catch, then passed again once reverted. Live-verified against real cluster objects, not just
  fixtures: `workloads_seen` went from 17 to 21 (the real 1 StatefulSet + 3 DaemonSets already
  running), zero new query errors, and `/api/timeseries` returned real non-empty CPU data for
  Prometheus itself (StatefulSet), node-exporter and kube-proxy (DaemonSets) -- proving pod
  resolution actually works end-to-end for both new kinds, not just that they're counted without
  erroring. D-007 RBAC re-confirmed unaffected with a real token (`statefulsets`/`daemonsets`:
  yes; `pods`/`secrets`: still no). `/readyz` unaffected (still reads `Status`, untouched here).
  No change to D-006, rule thresholds, cost semantics, or the frontend.
- 2026-09-26: Dashboard shell redesign (owner-requested: "a professional SaaS dashboard for
  the frontend UI"), extending the KPI/status-strip visual language from the earlier design pass
  to the whole app rather than just Overview -- still D-006's information architecture
  underneath, only the chrome changed. `Layout.tsx`/`.css`: top nav bar replaced with a left
  sidebar app shell (the structural pattern shared by every reference dashboard discussed --
  Grafana, Datadog, Stripe, Linear), collapsing to a top bar with a horizontally-scrolling nav
  row on mobile, with the cluster switcher deliberately kept on its own always-visible row below
  the nav rather than sharing its scroll area (a first pass put it in the same scrollable row and
  it silently scrolled out of reach on a narrow viewport -- caught live, not assumed). One new
  token, `--shadow-card` (light/dark variants), gives content cards a single consistent elevation
  device -- chrome (sidebar) stays flat with a hairline border, so depth always signals "this is
  content". `PlatformStatus` converts its "Latest analysis run" section to the same `KpiRow`/
  `KpiTile` hero pattern Overview already uses, unifying the visual vocabulary instead of leaving
  it in the older plain key-value layout. Table header/row styling refined once in `Card.css` so
  every page's tables (Findings, Workloads, evidence) inherit it. Verified live via Playwright
  against real findings data across every page (Overview, Findings, Finding Detail x2, Workloads,
  Platform Status), desktop light, dark, and 390px mobile: zero console errors throughout: this
  same pass also incidentally reconfirmed PR18 end-to-end in the real UI -- the `monitoring`
  namespace's real Prometheus StatefulSet appears with a live resource finding on both the
  Findings and Workloads pages. Rebuilt and redeployed to both live clusters.
- 2026-09-29: Bug fix -- Grafana OOMKilled on startup (exit 137) after both kind clusters were
  rebuilt from scratch (host reset). `deploy/prometheus/values.yaml`'s Grafana memory limit
  (384Mi) was measured and set against whatever Grafana version was current back in Phase 1;
  kube-prometheus-stack 91.5.0 now ships Grafana 13.2.2, which builds an in-memory search index
  for its "unified storage" feature at startup ("Building index using memory" in its own logs) --
  a real memory cost the old limit never accounted for. Confirmed via `kubectl describe pod`
  (`lastState.terminated.reason: OOMKilled`), not guessed. Limit raised to 512Mi with headroom;
  applied via `helm upgrade` (not reinstall) on both clusters, live-remeasured after the fix
  (~407Mi real usage, confirming 384Mi was genuinely too tight, not a fluke). Both clusters'
  Grafana instances stable afterward, 0 restarts.
- 2026-09-29: UI audit + Analytics page (owner-requested). Scope confirmed with the owner first:
  a snapshot analytics view computed from the findings already on hand, not a historical-trends
  feature -- that would need persisted finding history, which `docs/requirements.md` explicitly
  lists as Out of MVP (D-004 no database, still PROPOSED), a real architecture decision the owner
  would need to make, not a UI task to just build.
  **Audit findings, both real, both caught live via Playwright, not assumed:** (1) the Findings
  table at 390px had no explicit wrap control on its longer text columns (Problem, and Workloads'
  full name column) -- the hidden, scrolled-off Problem column's wrapped text was silently
  inflating every row to ~162px tall while only three narrow columns stayed visible, instead of a
  predictable single-line-per-row table with normal horizontal scroll. Fixed with a shared
  `.truncate` class (ellipsis + `title` attribute for the full text) applied to both. (2) Zero
  intentional keyboard-focus styling anywhere in the app -- not broken (no `outline: none`
  found), but inconsistent across browsers/themes. Added one `:focus-visible` treatment in
  `index.css` for every interactive element.
  **New `/analytics` page** (`Analytics.tsx`, `components/BarBreakdown.tsx`): every number is
  computed client-side from the same `/api/findings` response already used elsewhere -- no new
  backend endpoint, no persistence. KPI hero (total findings, workloads affected, findings
  priced, total estimated potential difference), proportional bar breakdowns by severity/
  category/rule and by workload (plain CSS bars, no charting dependency added for what is a
  category count, not a time series), and a cost-impact summary carrying the same "Estimated,
  never savings" disclaimer as a single finding's own cost card. Verified live via Playwright:
  desktop, dark mode, 390px mobile, zero console errors on all three.
- 2026-10-01: Breadcrumb + workload heatmap grid (owner-requested, pointing at a Datadog-style
  reference dashboard). Scope confirmed first, same as every other reference-UI request this
  project has had: adopt the two genuinely transferable pieces, decline the raw-count-tile
  pattern that would reintroduce "data without insight" (D-006's whole reason for existing).
  `components/Breadcrumb.tsx` (new): `{cluster_id} / {page}` wayfinding line above every page's
  content, reusing the same `/api/clusters` call the switcher already makes. `StatusStrip`
  (the chip strip from the earlier design pass) rewritten in place to a dense CSS-grid heatmap of
  fixed-size colored squares instead of wrapped text chips -- same underlying contract as before
  (one square per workload with an active finding, colored by worst severity, links to the real
  finding, workloads with no finding excluded rather than padded in as a fake "OK" square per
  R005), only the layout got denser; declined copying the reference's raw CPU/Memory-gradient
  heatmap, which carries no evidence or explanation. Verified live via Playwright across light,
  dark and 390px mobile: zero console errors.
- 2026-10-01: R006 -- unschedulable/Pending, the oldest backlog item (deferred since Phase 4,
  `demo/pending` fixture existed specifically for it). Prompted by a request to replicate a
  Datadog-style raw-metrics dashboard "exactly" -- declined as a straight copy (would reverse
  D-006), but the owner's real ask reframed it correctly: turn what those dashboards show as a
  raw "0 Pending" counter into an actual evidence-based finding, which is this project's whole
  purpose applied to a gap it already had. `evidence.SchedulingEvidence` (new, Prometheus-only
  like every other evidence field -- D-007 grants no Kubernetes Events access, so the builder
  still never touches the Kubernetes API): `Pending` and `Unschedulable`, from
  `kube_pod_status_phase{phase="Pending"}` and `kube_pod_status_unschedulable` (both named
  VERIFIED-available in the measurement model since Phase 1, just never wired into a rule).
  `rules.R006Unschedulable` fires only when **both** are true together -- Pending alone is the
  normal few-second state every pod passes through on start, firing on that alone would be a
  false positive on every healthy deployment. The workload's own requested CPU/memory is attached
  as evidence so a reader can judge the likely cause (e.g. a request exceeding any node's real
  capacity) without the rule asserting a specific FailedScheduling reason it was never given.
  5 new tests (2 in `internal/evidence` for the Prometheus queries themselves, 3 in
  `internal/rules` for the fire/no-fire boundary, including the critical "Pending but
  schedulable" case that must never fire). Live-verified via `cmd/verify` against the real
  `demo/pending` fixture before and after: `pending=true unschedulable=true` (matching the
  fixture's real cause, `cpu_request=64` cores against a 12-core node), and all 8 other demo
  fixtures confirmed `pending=false unschedulable=false` -- zero false positives.
  `demo/expected-findings.yaml`'s `pending` scenario updated from `rules_fired: []` to `[R006]`,
  closing the last gap in the project's own integration-test ground truth. No change to R001-R004,
  cost semantics, `/readyz`, or D-007's RBAC scope (still zero Kubernetes API calls from the
  evidence layer).
- 2026-10-01: Frontend design-system refactor (owner-requested: match the reference dashboards'
  production look, same functionality). **Frontend-only by construction** -- the diff touches
  `frontend/src` and nothing else: no backend, no API, no deploy manifests, no routing change,
  no new dependency (`package.json` untouched), and the backend suite still passes unchanged.
  One token system in `index.css` (surface/text/status scales, spacing, radius, one shadow,
  dark-first with full light parity) that every component now reads from -- no component
  hard-codes a colour. New `components/Icons.tsx`: eight hand-rolled inline SVGs rather than
  pulling in lucide-react, since AGENTS.md requires a stated reason per dependency and "eight
  glyphs" is not one (the exported Figma reference project carries 40+ dependencies for the same
  job). App shell gained a top bar (breadcrumb left, cluster switcher right) over the existing
  sidebar, which picked up icons; `Card` gained a real header slot (title/subtitle/actions) so
  panel headers are consistent everywhere; `KpiTile` gained an icon, a one-line hint and a tone
  accent edge; Overview moved to the reference's two-column shape -- ranked findings plus a
  "Needs attention" rail whose cards show each finding's own top evidence values. Three real
  bugs found and fixed during live verification, not assumed: uPlot strokes a canvas where a
  `var(--token)` is not a valid colour (the chart would have drawn nothing in the new theme, so
  colours are now resolved against computed style, including axis and gridline strokes, which
  previously used uPlot's light-theme defaults and were invisible on the dark panel); the card
  header crammed title and filters into one row at 390px and overflowed the panel edge; and four
  full-width KPI cards pushed the actual findings a screen and a half down on a phone (now
  two-up). Verified live against real cluster findings across all five pages, desktop dark,
  desktop light and 390px mobile: zero console errors throughout.

## MVP done-condition
See `docs/requirements.md` → MVP.
