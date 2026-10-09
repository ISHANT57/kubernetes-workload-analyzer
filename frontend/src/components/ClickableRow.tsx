import type { MouseEvent, ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'

/** Where the detail page's back link should return to. */
export interface BackState {
  back: { to: string; label: string }
}

/** A table row that opens its finding when clicked anywhere. The link inside the row stays the
 * keyboard and screen-reader path (and supports open-in-new-tab); this only widens the mouse and
 * touch target. Clicks on links, buttons and form controls are left to those elements, and
 * selecting text in a row does not navigate. */
export function ClickableRow({ to, back, children }: { to: string; back: BackState['back']; children: ReactNode }) {
  const navigate = useNavigate()

  const onClick = (e: MouseEvent<HTMLTableRowElement>) => {
    if ((e.target as HTMLElement).closest('a, button, select, input, textarea')) return
    if (window.getSelection()?.toString()) return
    navigate(to, { state: { back } satisfies BackState })
  }

  return (
    <tr className="row-link" onClick={onClick}>
      {children}
    </tr>
  )
}
