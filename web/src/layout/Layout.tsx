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
            Tennis Wiki
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
        <p>
          Built on data originating from the work of Jeff Sackmann and Tennis Abstract,
          licensed CC BY-NC-SA 4.0. This site is non-commercial and is not affiliated with
          the ATP, the WTA or the ITF.
        </p>
        <p>
          Where a statistic was never recorded, this site says so rather than showing a
          zero.
        </p>
      </footer>
    </div>
  )
}
