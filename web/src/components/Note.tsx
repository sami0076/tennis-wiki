import type { ReactNode } from 'react'
import styles from './Note.module.css'

/**
 * Note keeps a caption out of the way: a small ⓘ that opens the explanation
 * on request. The text stays in the page for search and assistive tech.
 */
export function Note({ children, label = 'About these figures' }: { children: ReactNode; label?: string }) {
  return (
    <details className={styles.note}>
      <summary className={styles.summary}>
        <span className={styles.icon} aria-hidden="true">
          i
        </span>
        <span className={styles.label}>{label}</span>
      </summary>
      <div className={styles.body}>{children}</div>
    </details>
  )
}
