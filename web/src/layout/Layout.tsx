import { useState, type ReactNode } from 'react'
import { NavLink, Link, useNavigate } from 'react-router-dom'
import { PlayerSearch } from '../components'
import styles from './Layout.module.css'

interface LayoutProps {
  children: ReactNode
}

// The nav in home.png, in its order, plus head-to-head. The mockup reaches that
// page from a player; until a player page links to one, a headline feature with
// no way in is worse than a fourth tab. Simulator is Phase 3 and leads to a
// placeholder, because leaving it out would make every later page edit the nav.
const links = [
  { to: '/players', label: 'Players' },
  { to: '/h2h', label: 'Head to head' },
  { to: '/rankings', label: 'Rankings' },
  { to: '/simulator', label: 'Simulator' },
]

/**
 * Layout is the whole site chrome: a hairline under the nav, content on paper,
 * and the attribution the data licence requires in the footer of every page.
 */
export function Layout({ children }: LayoutProps) {
  return (
    <div className={styles.shell}>
      <header>
        <nav className={styles.nav}>
          <Link to="/" className={styles.brand}>
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
                {link.label}
              </NavLink>
            ))}
          </div>
          <div className={styles.search}>
            <HeaderSearch />
          </div>
        </nav>
      </header>

      <main className={styles.main}>{children}</main>

      <footer className={styles.footer}>
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
      label="Search players"
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
