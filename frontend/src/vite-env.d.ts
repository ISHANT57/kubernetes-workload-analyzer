/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Grafana base URL for the "view in Grafana" links (Phase 6). Defaults to
   * http://localhost:3000 (a `kubectl port-forward svc/kps-grafana 3000:80`) if unset. */
  readonly VITE_GRAFANA_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
