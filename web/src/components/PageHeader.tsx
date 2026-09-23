import type { ReactNode } from 'react'
import styles from './PageHeader.module.css'

interface PageHeaderProps {
  kicker?: ReactNode
  title: ReactNode
  lede?: ReactNode
  /** Beside the title on a wide screen: an illustration or a panel. */
  art?: ReactNode
  accent?: 'a' | 'b' | 'lime'
  children?: ReactNode
}

/** PageHeader opens a page: a kicker with its accent dash, a big title, a standfirst. */
export function PageHeader({ kicker, title, lede, art, accent = 'lime', children }: PageHeaderProps) {
  return (
    <header className={art ? `${styles.header} ${styles.withArt}` : styles.header}>
      <div className={styles.text}>
        {kicker !== undefined ? (
          <div className={styles.kicker}>
            <span className={`${styles.dash} ${styles[accent]}`} aria-hidden="true" />
            {kicker}
          </div>
        ) : null}
        <h1 className={styles.title}>{title}</h1>
        {lede !== undefined ? <p className={styles.lede}>{lede}</p> : null}
        {children}
      </div>
      {art ? <div className={styles.art}>{art}</div> : null}
    </header>
  )
}
