import type { ReactNode } from 'react'
import styles from './StatRow.module.css'

interface StatRowProps {
  label: string
  children: ReactNode
}

/** StatRow is a label and a figure on one hairline-separated line. */
export function StatRow({ label, children }: StatRowProps) {
  return (
    <div className={styles.row}>
      <span className={styles.label}>{label}</span>
      <span className={styles.value}>{children}</span>
    </div>
  )
}
