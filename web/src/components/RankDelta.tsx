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
  // A real minus sign, not a hyphen: in a tabular-nums column the hyphen is
  // narrower than the plus it sits under and the signs stop lining up.
  const rendered = delta === 0 ? '0' : delta > 0 ? `+${delta}` : `\u2212${Math.abs(delta)}`
  const description = delta === 0 ? `Level on ${label}` : `${rendered} ${label}`
  return (
    <span className={className} aria-label={description}>
      {rendered}
    </span>
  )
}
