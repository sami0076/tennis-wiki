import type { Snapshot } from '../lib/playback'
import { SplitBar } from './SplitBar'
import styles from './Tracker.module.css'

interface TrackerProps {
  snapshot: Snapshot
}

/**
 * Tracker is the tally under a played-out match: points won as a centre-out
 * bar, break points converted, and service holds. Every figure is counted
 * from the points the sample actually played, never invented.
 */
export function Tracker({ snapshot }: TrackerProps) {
  const [pointsA, pointsB] = snapshot.points
  return (
    <div className={styles.tracker}>
      <SplitBar label="Points won" left={pointsA} right={pointsB} max={Math.max(1, pointsA + pointsB)} />
      <div className={styles.line}>
        <span className={styles.a}>
          {snapshot.breakPointsWon[0]} of {snapshot.breakPointsFaced[0]}
        </span>
        <span className={styles.label}>Break points won</span>
        <span className={styles.b}>
          {snapshot.breakPointsWon[1]} of {snapshot.breakPointsFaced[1]}
        </span>
      </div>
      <div className={styles.line}>
        <span className={styles.a}>{snapshot.holds[0]}</span>
        <span className={styles.label}>Service holds</span>
        <span className={styles.b}>{snapshot.holds[1]}</span>
      </div>
    </div>
  )
}
