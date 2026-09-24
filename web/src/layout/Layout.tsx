import { useState, type ReactNode } from 'react'
import { NavLink, Link, useLocation, useNavigate } from 'react-router-dom'
import { PlayerSearch } from '../components'
import { Backdrop, ScrollProgress } from './Atmosphere'
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
  return (
    <div className={styles.shell}>
      <Backdrop />
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
          <div className={styles.search}>
            <HeaderSearch />
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
    </div>
  )
}

/**
 * The search is in the chrome rather than on a page because it is how anyone
 * reaches a player, and needing to go back to a search page first would make
 * every other page a dead end.
 *
 * Choosing a result goes straight to that player. Pressing Enter without
 * choosing goes to the full result list instead, which is the right answer when
 * the eight rows on offer were not enough to settle a namesake.
 */
function HeaderSearch() {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')

  return (
    <PlayerSearch
      label="Player"
      hideLabel
      placeholder="Search 115,000 players"
      value={query}
      onChange={setQuery}
      onSelect={(player) => {
        setQuery('')
        navigate(`/players/${player.slug}`)
      }}
      onSubmit={(value) => {
        if (value === '') return
        setQuery('')
        navigate(`/players?q=${encodeURIComponent(value)}`)
      }}
    />
  )
}
