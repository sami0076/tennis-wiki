import styles from './WinSplit.module.css'

interface WinSplitProps {
  nameA: string
  nameB: string
  /** A's share, between 0 and 1. B's is the rest. */
  share: number
}

/**
 * WinSplit is the headline of a simulation: two names, two percentages, and one
 * bar divided between them.
 *
 * Monochrome, like every other two-player comparison on this site. Hue means
 * surface, and the simulator page already spends it on the surface control and
 * the draw odds.
 */
export function WinSplit({ nameA, nameB, share }: WinSplitProps) {
  const a = Math.round(share * 1000) / 10
  const b = Math.round((1 - share) * 1000) / 10

  return (
    <div>
      <div className={styles.split}>
        <span className={styles.nameA}>
          {nameA} <span className={styles.share}>{a.toFixed(1)}%</span>
        </span>
        <span className={styles.nameB}>
          <span className={styles.share}>{b.toFixed(1)}%</span> {nameB}
        </span>
      </div>
      <div
        className={styles.track}
        role="img"
        aria-label={`${nameA} ${a.toFixed(1)}%, ${nameB} ${b.toFixed(1)}%`}
      >
        <div className={styles.fill} style={{ width: `${a}%` }} />
      </div>
    </div>
  )
}
