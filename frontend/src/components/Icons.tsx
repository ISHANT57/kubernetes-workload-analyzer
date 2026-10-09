/** Inline SVG icon set, hand-rolled rather than pulling in an icon package. The reference
 * dashboards this UI is modelled on use lucide-react (~1MB of icons for the handful actually
 * rendered); AGENTS.md requires a stated reason for every dependency, and "eight 24x24 glyphs"
 * is not one. Each icon inherits currentColor and sizes from the `size` prop, so a caller
 * controls both through normal CSS. */

interface IconProps {
  size?: number
  className?: string
}

function svgProps(size: number, className?: string) {
  return {
    width: size,
    height: size,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.8,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    className,
    'aria-hidden': true,
  }
}

export function IconOverview({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <rect x="3" y="3" width="7" height="9" rx="1.5" />
      <rect x="14" y="3" width="7" height="5" rx="1.5" />
      <rect x="14" y="12" width="7" height="9" rx="1.5" />
      <rect x="3" y="16" width="7" height="5" rx="1.5" />
    </svg>
  )
}

export function IconFindings({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M10.3 3.6 1.9 18a2 2 0 0 0 1.7 3h16.8a2 2 0 0 0 1.7-3L13.7 3.6a2 2 0 0 0-3.4 0Z" />
      <path d="M12 9v4" />
      <path d="M12 17h.01" />
    </svg>
  )
}

export function IconWorkloads({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" />
      <path d="m3.3 7 8.7 5 8.7-5" />
      <path d="M12 22V12" />
    </svg>
  )
}

export function IconAnalytics({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M3 3v16a2 2 0 0 0 2 2h16" />
      <path d="M7 15l3-4 3 3 4-6" />
    </svg>
  )
}

export function IconPlatform({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
    </svg>
  )
}

export function IconCritical({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v4.5" />
      <path d="M12 16h.01" />
    </svg>
  )
}

export function IconWarning({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M10.3 3.6 1.9 18a2 2 0 0 0 1.7 3h16.8a2 2 0 0 0 1.7-3L13.7 3.6a2 2 0 0 0-3.4 0Z" />
    </svg>
  )
}

export function IconInfo({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5" />
      <path d="M12 8h.01" />
    </svg>
  )
}

export function IconCube({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <path d="M3 9h18" />
      <path d="M9 21V9" />
    </svg>
  )
}

export function IconClock({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  )
}

export function IconCoin({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M14.5 9.5a2.5 2.5 0 0 0-2.5-1.5c-1.4 0-2.5.8-2.5 2s1.1 2 2.5 2 2.5.8 2.5 2-1.1 2-2.5 2a2.5 2.5 0 0 1-2.5-1.5" />
      <path d="M12 6.5v11" />
    </svg>
  )
}

export function IconPulse({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M3 12h4l2.5-6 4 13 2.5-7h5" />
    </svg>
  )
}

export function IconSun({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  )
}

export function IconMoon({ size = 16, className }: IconProps) {
  return (
    <svg {...svgProps(size, className)}>
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </svg>
  )
}
