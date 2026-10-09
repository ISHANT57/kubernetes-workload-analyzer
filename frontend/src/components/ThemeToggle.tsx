import { useState } from 'react'
import { currentTheme, saveTheme } from '../theme'
import type { Theme } from '../theme'
import { IconMoon, IconSun } from './Icons'
import './ThemeToggle.css'

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(currentTheme)

  const choose = (next: Theme) => {
    saveTheme(next)
    setTheme(next)
  }

  return (
    <div className="theme-toggle" role="group" aria-label="Colour theme">
      <button type="button" className={theme === 'light' ? 'on' : ''} aria-pressed={theme === 'light'} aria-label="Light theme" onClick={() => choose('light')}>
        <IconSun size={14} />
        <span className="theme-label">Light</span>
      </button>
      <button type="button" className={theme === 'dark' ? 'on' : ''} aria-pressed={theme === 'dark'} aria-label="Dark theme" onClick={() => choose('dark')}>
        <IconMoon size={14} />
        <span className="theme-label">Dark</span>
      </button>
    </div>
  )
}
