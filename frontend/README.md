# frontend

React + Vite + TypeScript + uPlot dashboard (Phase 5). Consumes the Go backend's JSON API
(`/api/findings`, `/api/runs/latest`, `/api/timeseries`, `/api/cluster/summary`, `/api/clusters`) -- it never talks to Prometheus directly
(docs/architecture.md).

## Pages

| Route | What it answers |
|---|---|
| `/` (Dashboard) | Is anything wrong right now? Severity counts, workload-health gauge, most severe findings with evidence, run status, findings table. |
| `/cluster` | Cluster-wide capacity and health (nodes, pods, CPU and memory allocatable vs requested vs used) with plain-language highlights; missing metrics are listed, never shown as zero. |
| `/findings` | The full ranked list, filterable by namespace/severity/category. Filters live in the URL, so they survive going back from a finding. Whole rows are clickable. |
| `/findings/:id` | A rule-based plain-language summary (what happened, why it matters, what to check), full evidence, confidence reasoning, estimated cost, and (for R001/R002) a usage-vs-request chart. |
| `/workloads` | Workloads with an active CPU/memory over-provisioning finding — request vs observed vs suggested vs estimated cost. |
| `/analytics` | Aggregates of the current findings: by rule (stem chart), severity, category, workload, and summed estimated cost. A snapshot, not a trend. |
| `/status` | The analyzer observing itself: last run duration, data source errors, links to `/healthz` `/readyz` `/metrics`. |

## Theme and time

- Light and dark themes via `data-theme` on `<html>` (`src/theme.ts`; stored choice, else the OS
  preference). Text meets WCAG AA contrast in both.
- Chart timestamps are Unix seconds (UTC instants) from the API; the browser converts to its own
  zone only for display, and the page states the zone and the latest sample in UTC (D-012).

## Run it locally

```bash
npm install
npm run dev          # http://localhost:5173, proxies /api to http://localhost:8081 (vite.config.ts)
```

Needs a running backend (`cd ../backend && go run ./cmd/analyzer`) with `LISTEN_ADDR=127.0.0.1:8081`
to match the proxy target, or edit `vite.config.ts`.

## Build

```bash
npm run lint
npm test              # vitest: time-zone and time-range helpers
npm run build         # -> dist/
```

The backend serves `dist/` directly when started with `STATIC_DIR=$(pwd)/dist` (see
`backend/README.md`) — one binary, no separate Node process at runtime, with client-side routes
(e.g. `/findings/abc123`) falling back to `index.html` correctly on a hard refresh.

## Design notes

- No component library, no CSS framework: plain CSS with theme variables in `src/index.css`
  (light/dark via `prefers-color-scheme`), kept small and dependency-light per AGENTS.md ("no new
  dependency without a stated reason").
- `usePolling` (src/hooks) keeps the last good data on screen and marks it "stale" if a refresh
  fails, instead of replacing a working page with an error screen — the same "sticky snapshot"
  principle the backend itself uses (docs/architecture.md).
- Verified live with a real backend and a real cluster via Playwright, not just eyeballed:
  desktop, mobile width (390px), and dark mode all checked, zero console errors. A CSS overflow
  bug (long PromQL query text breaking the evidence table's layout on narrow screens) was found
  and fixed this way, not left for someone else to find.
