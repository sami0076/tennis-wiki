import styles from './RankDelta.module.css'

interface RankDeltaProps {
  /** Positive means this player ranks higher by Elo than officially. */
  delta: number
  /** What the two rankings being compared are, for the accessible name. */
  label?: string
}

/**
 * RankDelta is a signed integer. The sign carries the meaning and the colour
 * repeats it, in that order -- an unsigned green number would be colour as the
 * only encoding.
 */
export function RankDelta({ delta, label = 'places' }: RankDeltaProps) {
  const className =
    delta > 0 ? `${styles.delta} ${styles.up}` : delta < 0 ? `${styles.delta} ${styles.down}` : `${styles.delta} ${styles.level}`
  const sign = delta > 0 ? '+' : ''
  const description =
    delta === 0 ? `Level on ${label}` : `${sign}${delta} ${label}`
  return (
    <span className={className} aria-label={description}>
      {delta === 0 ? '0' : `${sign}${delta}`}
    </span>
  )
}
