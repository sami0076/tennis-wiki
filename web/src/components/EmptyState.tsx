import type { ReactNode } from 'react'
import styles from './EmptyState.module.css'

interface EmptyStateProps {
  /** Names what is absent. Not "No data" -- say which data. */
  heading: string
  /** One or two sentences on why it is absent. */
  reason: ReactNode
  /**
   * Somewhere to go that does have data. Emptiness is direction, not mood, so
   * this is the part that makes the component worth having.
   */
  action?: ReactNode
}

/**
 * EmptyState is a whole section with nothing to show.
 *
 * No panel, no illustration, no icon. It reads as a footnote in the page rather
 * than a hole in it, because at 83% of matches carrying no serve statistics
 * this renders more often than the numbers do.
 */
export function EmptyState({ heading, reason, action }: EmptyStateProps) {
  return (
    <section className={styles.empty}>
      <h3 className={styles.heading}>{heading}</h3>
      <p className={styles.reason}>{reason}</p>
      {action ? <div className={styles.action}>{action}</div> : null}
    </section>
  )
}
