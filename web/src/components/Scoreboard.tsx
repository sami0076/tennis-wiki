import type { Side, Snapshot } from '../lib/playback'
import { surfaceVar } from '../lib/surface'
import styles from './Scoreboard.module.css'

interface ScoreboardProps {
  snapshot: Snapshot
  /** Display names, A first. */
  names: readonly [string, string]
  /** The match's surface: the one hue on the board, on the serve square and the current set. */
  surface: string
  /** Whether a match is being played out, which is when the serve square pulses. */
  playing: boolean
}

/**
 * Scoreboard is the board a match is followed on: a column per set, the
 * loser's tiebreak points as a superscript on their 6, the set in progress
 * boxed in the surface's hue, and a square in that hue before whoever serves.
 *
 * Each side is in its own hue, teal and rose, as in every comparison; a set
 * they lost is pencil. A broken player's row flashes the highlight and a
 * moving figure pops.
 */
export function Scoreboard({ snapshot, names, surface, playing }: ScoreboardProps) {
  const hue = surfaceVar(surface)
  const columns = snapshot.sets.length + (snapshot.current ? 1 : 0)

  const row = (side: Side) => {
    const serving = snapshot.current !== null && snapshot.server === side
    const flashing = snapshot.flash === side
    const own = side === 0 ? styles.sideA : styles.sideB
    return (
      <tr className={flashing ? `${styles.row} ${styles.flash} ${own}` : `${styles.row} ${own}`}>
        <th scope="row" className={styles.name}>
          <span
            className={serving ? (playing ? `${styles.serve} ${styles.pulse}` : styles.serve) : styles.serveGap}
            style={serving ? { background: hue } : undefined}
            aria-hidden="true"
          />
          {names[side]}
          {serving ? <span className="sr-only"> serving</span> : null}
        </th>
        {snapshot.sets.map((set, index) => {
          const games = side === 0 ? set.a : set.b
          const tiebreak = side === 0 ? set.tiebreakA : set.tiebreakB
          const won = side === 0 ? set.aWon : !set.aWon
          return (
            <td key={index} className={won ? styles.setWon : styles.setLost}>
              {games}
              {tiebreak !== null ? <sup className={styles.tiebreak}>{tiebreak}</sup> : null}
            </td>
          )
        })}
        {snapshot.current ? (
          <td className={styles.currentCell} style={{ borderColor: hue }}>
            <span className={snapshot.pop === side ? styles.pop : undefined}>
              {side === 0 ? snapshot.current.a : snapshot.current.b}
            </span>
          </td>
        ) : null}
      </tr>
    )
  }

  return (
    <table className={styles.board}>
      <thead>
        <tr>
          <td className={styles.corner} />
          {Array.from({ length: columns }, (_, index) => {
            const current = snapshot.current !== null && index === columns - 1
            return (
              <th
                key={index}
                scope="col"
                className={current ? styles.setHeadCurrent : styles.setHead}
                style={current ? { color: hue, borderColor: hue } : undefined}
              >
                <span className="sr-only">Set </span>
                {index + 1}
              </th>
            )
          })}
        </tr>
      </thead>
      <tbody>
        {row(0)}
        {row(1)}
      </tbody>
    </table>
  )
}
