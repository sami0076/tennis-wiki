import type { CSSProperties } from 'react'
import { useInView } from '../lib/useInView'
import styles from './FormPills.module.css'

interface FormPillsProps {
  /** Oldest first. */
  results: ReadonlyArray<boolean>
  side?: 'a' | 'b'
  size?: 'sm' | 'lg'
  label?: string
}

/** FormPills is recent form as a row of pills: filled for a win, grey for a loss. */
export function FormPills({ results, side, size = 'sm', label = 'Recent form' }: FormPillsProps) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const wins = results.filter(Boolean).length
  return (
    <div
      ref={ref}
      className={[styles.row, styles[size], side ? styles[side] : '', seen ? styles.seen : ''].join(' ')}
      role="img"
      aria-label={`${label}: ${wins} won, ${results.length - wins} lost, oldest first`}
    >
      {results.map((won, index) => (
        <span
          key={index}
          className={won ? `${styles.pill} ${styles.won}` : styles.pill}
          style={{ '--i': index } as CSSProperties}
        />
      ))}
    </div>
  )
}
