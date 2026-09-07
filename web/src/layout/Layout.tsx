import type { ReactNode } from 'react'
import { NavLink, Link } from 'react-router-dom'
import styles from './Layout.module.css'

interface LayoutProps {
  children: ReactNode
}

// The nav in home.png, in its order. Simulator is Phase 3 and leads to a
// placeholder for now: leaving it out would mean every later page had to come
// back and edit the layout, and the design already says how many tabs there are.
const links = [
  { to: '/players', label: 'Players' },
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
        </nav>
      </header>

      <main className={styles.main}>{children}</main>

      <footer className={styles.footer}>
        <p>Data: Jeff Sackmann&apos;s tennis_atp and tennis_wta, CC BY-NC-SA 4.0</p>
      </footer>
    </div>
  )
}
