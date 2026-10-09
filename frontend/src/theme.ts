export type Theme = 'light' | 'dark'

const KEY = 'analyzer-theme'

function stored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null // storage blocked (private mode): fall back to the OS preference
  }
}

export function currentTheme(): Theme {
  return stored() ?? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
}

export function applyTheme(theme: Theme): void {
  document.documentElement.setAttribute('data-theme', theme)
}

export function saveTheme(theme: Theme): void {
  applyTheme(theme)
  try {
    localStorage.setItem(KEY, theme)
  } catch {
    // Not persisted; the choice still applies for this session.
  }
}
