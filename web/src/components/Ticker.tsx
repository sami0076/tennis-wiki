import type { ReactNode } from 'react'
import styles from './Ticker.module.css'

export interface TickerItem {
  key: string
  label: ReactNode
  value: ReactNode
  tone?: 'a' | 'b' | 'lime'
}

/**
 * Ticker scrolls a strip of figures across the page like a broadcast crawl.
 * The second copy is only there to make the loop seamless, so it is hidden
 * from assistive tech; hovering pauses it.
 */
export function Ticker({ items, label }: { items: ReadonlyArray<TickerItem>; label: string }) {
  if (items.length === 0) return null
  const run = (hidden: boolean) => (
    <ul className={styles.run} aria-hidden={hidden || undefined}>
      {items.map((item) => (
        <li key={item.key} className={styles.item}>
          <span className={`${styles.dot} ${item.tone ? styles[item.tone] : ''}`} aria-hidden="true" />
          <span className={styles.label}>{item.label}</span>
          <span className={styles.value}>{item.value}</span>
        </li>
      ))}
    </ul>
  )
  return (
    <section className={styles.ticker} aria-label={label}>
      <span className={styles.live} aria-hidden="true">
        Latest
      </span>
      <div className={styles.viewport}>
        <div className={styles.track}>
          {run(false)}
          {run(true)}
        </div>
      </div>
    </section>
  )
}
