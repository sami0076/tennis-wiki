import { THEMES, useTheme, type Theme } from '../lib/useTheme'
import styles from './ThemeToggle.module.css'

const LABELS: Record<Theme, string> = {
  system: 'Match my system',
  light: 'Light',
  dark: 'Dark',
}

/**
 * One button that cycles system → light → dark and back. A cycle rather than a
 * three-cell tray because the header already carries a brand, five tabs and a
 * search field, and a fourth control with three cells in it would be the widest
 * thing up there for the least-used decision on the site.
 *
 * The mark is drawn rather than lettered: a full disc for light, a bitten one
 * for dark, and a half-filled one for following the system. The button's
 * accessible name always says which of the three is on and what pressing it
 * does, so the drawing never has to carry the meaning alone.
 */
export function ThemeToggle() {
  const { theme, resolved, setTheme } = useTheme()
  const next = THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length] ?? 'system'

  return (
    <button
      type="button"
      className={styles.toggle}
      data-theme-choice={theme}
      onClick={() => setTheme(next)}
      aria-label={`Theme: ${LABELS[theme]}${
        theme === 'system' ? ` (${resolved})` : ''
      }. Switch to ${LABELS[next].toLowerCase()}.`}
      title={`Theme: ${LABELS[theme]}`}
    >
      <Mark theme={theme} />
    </button>
  )
}

/**
 * The three marks are one circle drawn three ways, so the switch between them
 * reads as the same object changing rather than three unrelated pictures.
 */
function Mark({ theme }: { theme: Theme }) {
  return (
    <svg className={styles.mark} viewBox="0 0 16 16" aria-hidden="true" focusable="false">
      {theme === 'light' ? (
        <>
          <circle cx="8" cy="8" r="3.4" className={styles.disc} />
          {[0, 45, 90, 135, 180, 225, 270, 315].map((angle) => (
            <line
              key={angle}
              x1="8"
              y1="1.6"
              x2="8"
              y2="3.2"
              className={styles.ray}
              transform={`rotate(${angle} 8 8)`}
            />
          ))}
        </>
      ) : null}

      {theme === 'dark' ? (
        // A crescent cut from a disc rather than drawn as a path, so it keeps
        // the same radius as the other two marks.
        <>
          <mask id="dp-theme-crescent">
            <rect width="16" height="16" fill="black" />
            <circle cx="8" cy="8" r="5.4" fill="white" />
            <circle cx="11.4" cy="5.2" r="4.6" fill="black" />
          </mask>
          <rect width="16" height="16" className={styles.disc} mask="url(#dp-theme-crescent)" />
        </>
      ) : null}

      {theme === 'system' ? (
        <>
          <circle cx="8" cy="8" r="5" className={styles.ring} />
          <path d="M8 3 A5 5 0 0 1 8 13 Z" className={styles.disc} />
        </>
      ) : null}
    </svg>
  )
}
