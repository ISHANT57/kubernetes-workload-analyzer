import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Dev only: the built app is served same-origin by the Go backend (see internal/httpserver),
    // so this proxy exists purely so `npm run dev` can talk to a locally running `go run
    // ./cmd/analyzer` without a CORS setup that production never needs.
    proxy: {
      '/api': 'http://localhost:8081',
      '/healthz': 'http://localhost:8081',
      '/readyz': 'http://localhost:8081',
    },
  },
})
