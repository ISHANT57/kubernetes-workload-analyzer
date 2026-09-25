import { useEffect, useRef, useState } from 'react'

export type Fetched<T> =
  | { status: 'loading' }
  | { status: 'error'; error: string }
  | { status: 'ok'; data: T; stale: boolean }

/**
 * Polls `fetcher` every `intervalMs` and exposes the latest result. On a failed poll, the
 * previous good data is kept and marked `stale: true` rather than being replaced with an error
 * screen -- mirrors the backend's own "sticky snapshot, marked stale" design
 * (docs/architecture.md), so a transient blip never flashes the whole page to an error state.
 * The very first load has no prior data to fall back on, so it does show `status: 'error'`.
 */
export function usePolling<T>(fetcher: () => Promise<T>, intervalMs = 15000): Fetched<T> {
  const [state, setState] = useState<Fetched<T>>({ status: 'loading' })
  const lastGood = useRef<T | null>(null)

  useEffect(() => {
    let cancelled = false

    async function tick() {
      try {
        const data = await fetcher()
        if (cancelled) return
        lastGood.current = data
        setState({ status: 'ok', data, stale: false })
      } catch (err) {
        if (cancelled) return
        if (lastGood.current !== null) {
          setState({ status: 'ok', data: lastGood.current, stale: true })
        } else {
          setState({ status: 'error', error: err instanceof Error ? err.message : String(err) })
        }
      }
    }

    tick()
    const id = setInterval(tick, intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetcher is expected to be stable per call site (see usage)
  }, [intervalMs])

  return state
}
