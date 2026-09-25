# frontend

React + Vite + TypeScript + uPlot dashboard (Phase 5). Consumes the Go backend's JSON API
(`/api/findings`, `/api/runs/latest`, `/api/timeseries`) -- it never talks to Prometheus directly
(docs/architecture.md).

## Pages

| Route | What it answers |
|---|---|
| `/` (Overview) | Is anything wrong right now? Health counts, run status, top 5 findings. |
| `/findings` | The full ranked list, filterable by namespace/severity/category. |
| `/findings/:id` | One finding's full evidence, confidence reasoning, estimated cost, and (for R001/R002) a real usage-vs-request chart. |
| `/workloads` | Workloads with an active CPU/memory over-provisioning finding — request vs observed vs suggested vs estimated cost. |
| `/status` | The analyzer observing itself: last run duration, data source errors, links to `/healthz` `/readyz` `/metrics`. |

## Run it locally

```bash
npm install
npm run dev          # http://localhost:5173, proxies /api to http://localhost:8081 (vite.config.ts)
```

Needs a running backend (`cd ../backend && go run ./cmd/analyzer`) with `LISTEN_ADDR=127.0.0.1:8081`
to match the proxy target, or edit `vite.config.ts`.

## Build

```bash
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
