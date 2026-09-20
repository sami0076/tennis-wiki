import { Link } from 'react-router-dom'
import type { EditionMatch, EditionSide } from '../api/client'
import { sides, type RoundGroup, type Slot } from '../lib/bracket'
import { Score } from './Score'
import styles from './RoundList.module.css'

interface RoundStepperProps {
  rounds: ReadonlyArray<string>
  qualifying: ReadonlyArray<string>
  value: string
  onChange: (round: string) => void
}

/**
 * RoundStepper is the phone's way through a draw: one round at a time, the
 * round codes as typed cells with the current one boxed, the qualifying
 * rounds a step apart from the main draw's.
 */
export function RoundStepper({ rounds, qualifying, value, onChange }: RoundStepperProps) {
  const cell = (round: string) => (
    <button
      key={round}
      type="button"
      role="tab"
      aria-selected={round === value}
      className={round === value ? `${styles.cell} ${styles.active}` : styles.cell}
      onClick={() => onChange(round)}
    >
      {round}
    </button>
  )
  return (
    <div className={styles.stepper} role="tablist" aria-label="Round">
      {rounds.map(cell)}
      {qualifying.length > 0 ? <span className={styles.gap} aria-hidden="true" /> : null}
      {qualifying.map(cell)}
    </div>
  )
}

interface RoundListProps {
  groups: ReadonlyArray<RoundGroup>
  /** The final's winner is the champion, set in 700. */
  final?: boolean
}

/**
 * RoundList is one round of the draw as a list: each match two lines, the
 * winner in ink first and the loser in pencil, the score on the winner's line
 * and on its own line only when it does not fit. The sheet's page headings
 * come along so the draw keeps its shape on a phone.
 */
export function RoundList({ groups, final = false }: RoundListProps) {
  return (
    <div>
      {groups.map((group, i) => (
        <section key={group.title ?? i}>
          {group.title !== null ? <h3 className={styles.heading}>{group.title}</h3> : null}
          {group.slots.map((slot, j) => (
            <Line key={j} slot={slot} final={final} />
          ))}
        </section>
      ))}
    </div>
  )
}

function Line({ slot, final }: { slot: Slot; final: boolean }) {
  if (slot.side === null) return null
  if (slot.match === null) {
    return (
      <div className={styles.match}>
        <div className={styles.line}>
          <Name side={slot.side} won />
          <span className={styles.score}>{slot.void}</span>
        </div>
      </div>
    )
  }
  return <MatchBlock match={slot.match} final={final} />
}

interface MatchListProps {
  matches: ReadonlyArray<EditionMatch>
  /** A heading per tie or group, keyed by the field that names it. */
  groupBy?: (match: EditionMatch) => string | null
}

/**
 * MatchList lists matches that are not a bracket's: the ties of a team
 * competition, a round robin, a bronze medal match.
 */
export function MatchList({ matches, groupBy }: MatchListProps) {
  const groups: { title: string | null; matches: EditionMatch[] }[] = []
  for (const match of matches) {
    const title = groupBy ? groupBy(match) : null
    const last = groups[groups.length - 1]
    if (last !== undefined && last.title === title) {
      last.matches.push(match)
    } else {
      groups.push({ title, matches: [match] })
    }
  }
  return (
    <div>
      {groups.map((group, i) => (
        <section key={group.title ?? i}>
          {group.title !== null ? <h3 className={styles.heading}>{group.title}</h3> : null}
          {group.matches.map((match) => (
            <MatchBlock key={`${match.round}-${match.match_num}`} match={match} />
          ))}
        </section>
      ))}
    </div>
  )
}

function MatchBlock({ match, final = false }: { match: EditionMatch; final?: boolean }) {
  const [winner, loser] = sides(match)
  return (
    <div className={final ? `${styles.match} ${styles.final}` : styles.match}>
      <div className={styles.line}>
        <Name side={winner} won />
        <span className={styles.score}>
          <Score score={match.score} incomplete={match.incomplete} />
        </span>
      </div>
      <div className={styles.line}>
        <Name side={loser} />
      </div>
    </div>
  )
}

function Name({ side, won = false }: { side: EditionSide; won?: boolean }) {
  const pre = side.seed !== null ? `[${side.seed}] ` : side.entry !== null ? `${side.entry} ` : ''
  return (
    <span className={won ? `${styles.name} ${styles.won}` : styles.name}>
      {pre ? <span className={styles.pre}>{pre}</span> : null}
      <Link className={styles.player} to={`/players/${side.slug}`}>
        {side.name}
      </Link>
      {side.country !== null ? <span className={styles.country}> ({side.country})</span> : null}
    </span>
  )
}
