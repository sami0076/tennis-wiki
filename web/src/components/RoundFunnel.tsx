import { useInView } from '../lib/useInView'
import styles from './RoundFunnel.module.css'

export interface RoundRow {
  round: string
  matches: number
  wins: number
}

interface RoundFunnelProps {
  rounds: ReadonlyArray<RoundRow>
}

/** What the source's codes mean, spelled out once. */
const ROUND_WORDS: Record<string, string> = {
  R128: 'First round',
  R64: 'Second round',
  R32: 'Third round',
  R16: 'Last 16',
  QF: 'Quarter-final',
  SF: 'Semi-final',
  F: 'Final',
  RR: 'Round robin',
  BR: 'Bronze medal',
}

/**
 * The record round by round, drawn as the draw narrows.
 *
 * A career is a funnel: everybody plays first rounds and almost nobody plays
 * finals, so a bare table of eight rows hides the shape that matters. Each bar
 * is scaled against the busiest round, which makes how far a career usually
 * got legible before a single number is read, and the split inside each bar is
 * won against lost.
 *
 * R128 is "first round" only in a 128 draw; in a 32 draw the first round is
 * R32. The words here are the common case and the code stays beside them, so
 * a reader who knows the difference can see which is which.
 */
export function RoundFunnel({ rounds }: RoundFunnelProps) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const busiest = rounds.reduce((max, row) => Math.max(max, row.matches), 0)
  if (rounds.length === 0 || busiest === 0) return null

  return (
    <div ref={ref} className={seen ? `${styles.funnel} ${styles.seen}` : styles.funnel}>
      {rounds.map((row, index) => {
        const losses = row.matches - row.wins
        return (
          <div key={row.round} className={styles.row}>
            <span className={styles.label}>
              {ROUND_WORDS[row.round] ?? row.round}
              <span className={styles.code}>{row.round}</span>
            </span>
            <span
              className={styles.track}
              style={{ width: `${(row.matches / busiest) * 100}%`, '--delay': `${index * 60}ms` } as React.CSSProperties}
            >
              {row.wins > 0 ? (
                <span className={styles.won} style={{ flexGrow: row.wins }} />
              ) : null}
              {losses > 0 ? (
                <span className={styles.lost} style={{ flexGrow: losses }} />
              ) : null}
            </span>
            <span className={styles.record}>
              {row.wins}&#8209;{losses}
            </span>
          </div>
        )
      })}
      <p className={styles.key}>
        <span className={styles.keyPair}>
          <span className={styles.keyWon} aria-hidden="true" /> won
        </span>
        <span className={styles.keyPair}>
          <span className={styles.keyLost} aria-hidden="true" /> lost
        </span>
        <span className={styles.keyNote}>
          Bars are scaled against the busiest round. Qualifying is not counted, and R128 is the
          first round of a 128 draw rather than of every draw.
        </span>
      </p>
    </div>
  )
}
