import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { NavLink, Link, useLocation } from 'react-router-dom'
import { CommandPalette, ThemeToggle } from '../components'
import { ScrollProgress } from './Atmosphere'
import styles from './Layout.module.css'

interface LayoutProps {
  children: ReactNode
}

// The nav in home.png, in its order, plus head-to-head and tournaments. Six
// items is the ceiling Phase 5 set; seasons hang off the tournaments page and
// the home page rather than taking a tab of their own.
const links = [
  { to: '/players', label: 'Players', short: 'Players' },
  { to: '/h2h', label: 'Head to head', short: 'H2H' },
  { to: '/rankings', label: 'Rankings', short: 'Rankings' },
  { to: '/leaders', label: 'Leaders', short: 'Leaders' },
  { to: '/tournaments', label: 'Tournaments', short: 'Draws' },
  { to: '/simulator', label: 'Simulator', short: 'Simulator' },
]

/**
 * Layout is the whole site chrome: the sheet head, typed, with the active tab
 * set in brackets the way a seed is; the content; and the attribution the data
 * licence requires in the foot of every page.
 */
export function Layout({ children }: LayoutProps) {
  const { pathname } = useLocation()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const closePalette = useCallback(() => setPaletteOpen(false), [])

  usePaletteHotkey(() => setPaletteOpen(true))

  // A route change closes it. Every row in the palette navigates, and one that
  // survived the navigation would sit over the page it just opened.
  useEffect(() => setPaletteOpen(false), [pathname])

  return (
    <div className={styles.shell}>
      <ScrollProgress />
      <header className={styles.header}>
        <nav className={styles.nav}>
          <Link to="/" className={styles.brand}>
            <span className={styles.ball} aria-hidden="true" />
            Deucepoint
          </Link>
          <div className={styles.links}>
            {links.map((link) => (
              <NavLink
                key={link.to}
                to={link.to}
                className={({ isActive }) =>
                  isActive ? `${styles.link} ${styles.active}` : styles.link
                }
              >
                <span className={styles.bracket} aria-hidden="true" />
                <span className={styles.long}>{link.label}</span>
                <span className={styles.short} aria-hidden="true">
                  {link.short}
                </span>
                <span className={styles.bracket} aria-hidden="true" />
              </NavLink>
            ))}
          </div>
          <div className={styles.tools}>
            <SearchTrigger onOpen={() => setPaletteOpen(true)} />
            <ThemeToggle />
          </div>
        </nav>
      </header>

      <main key={pathname} className={styles.main}>
        {children}
      </main>

      <footer className={styles.footer}>
        <p className={styles.footerBrand}>
          <span className={styles.ball} aria-hidden="true" />
          Deucepoint
        </p>
        <p>
          <Link className={styles.footerLink} to="/methodology">
            How these numbers are produced
          </Link>
        </p>
        <p>Data: Jeff Sackmann&apos;s tennis_atp and tennis_wta, CC BY-NC-SA 4.0</p>
      </footer>

      <CommandPalette open={paletteOpen} onClose={closePalette} />
    </div>
  )
}

/**
 * The trigger is a button rather than the combobox it replaced. One search
 * field in the chrome could only ever open a player page; the palette behind
 * this button reaches every player, every page and the two comparisons that
 * are the point of the site, and it is the same one keystroke away.
 *
 * The shortcut is written on the button, because a shortcut nobody is told
 * about is a shortcut nobody uses.
 */
function SearchTrigger({ onOpen }: { onOpen: () => void }) {
  return (
    <button type="button" className={styles.trigger} onClick={onOpen}>
      <span className={styles.triggerGlyph} aria-hidden="true">
        ⌕
      </span>
      <span className={styles.triggerLabel}>Search</span>
      <kbd className={styles.shortcut} aria-hidden="true">
        {shortcutLabel()}
      </kbd>
    </button>
  )
}

/** ⌘K where that is the convention, Ctrl K everywhere else. */
function shortcutLabel(): string {
  if (typeof navigator === 'undefined') return 'Ctrl K'
  return /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent) ? '⌘K' : 'Ctrl K'
}

/**
 * ⌘K and Ctrl-K anywhere, and / when the reader is not already typing. The
 * slash is a convenience and must never swallow a slash meant for a field, so
 * it checks what has focus first.
 */
function usePaletteHotkey(open: () => void) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        open()
        return
      }
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      if (target !== null && isTyping(target)) return
      event.preventDefault()
      open()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open])
}

function isTyping(element: HTMLElement): boolean {
  const tag = element.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || element.isContentEditable
}
