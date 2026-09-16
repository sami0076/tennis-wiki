import { Link } from 'react-router-dom'
import { request, type ChartedFigures, type ChartedMatch } from '../api/client'
import { useResource } from '../api/useResource'
import { AbsentCell } from './AbsentCell'
import { Skeleton } from './Skeleton'
import styles from './ChartedSheet.module.css'

interface ChartedMarkProps {
  open: boolean
  onToggle: () => void
}

/**
 * ChartedMark is the typed word on a match row that has a charted sheet
 * behind it, and the control that opens it. It appears only on such rows:
 * the other 96% of the sheet carries nothing where it would go.
 */
export function ChartedMark({ open, onToggle }: ChartedMarkProps) {
  return (
    <button
      type="button"
      className={open ? styles.markOpen : styles.mark}
      aria-expanded={open}
      onClick={onToggle}
    >
      charted
    </button>
  )
}

interface ChartedSheetProps {
  /** The Match Charting Project's id, from the row's `charting_id`. */
  chartingId: string
  /** The slug of the player to put on the A side; the other takes B. */
  first?: string
}

/**
 * ChartedSheet is one match as the Match Charting Project's volunteers
 * recorded it: a row per figure, a column per set and one for the match, and
 * both players in every cell -- A in orange, B in turquoise, per the two
 * sides rule. Rates are derived here from the counts, with the counts shown
 * where a rate would need a denominator the set did not supply.
 */
export function ChartedSheet({ chartingId, first }: ChartedSheetProps) {
  const charted = useResource(
    (signal) => request<ChartedMatch>(`/charted/${encodeURIComponent(chartingId)}`, {}, signal),
    [chartingId],
  )

  if (charted.state === 'loading') {
    return <Skeleton lines={6} />
  }
  if (charted.state === 'error') {
    return (
      <p className={styles.note}>
        The charted sheet could not be loaded: {charted.error.message}
      </p>
    )
  }

  const match = charted.data
  const [winner, loser] = match.players
  if (winner === undefined || loser === undefined) {
    return <p className={styles.note}>The charted sheet names fewer than two players.</p>
  }
  // Sides: A is the player asked for, or the winner.
  const swap = first !== undefined && loser.slug === first
  const a = swap ? loser : winner
  const b = swap ? winner : loser
  const sets = [...match.sets].sort((x, y) => (x.set === 0 ? 1 : y.set === 0 ? -1 : x.set - y.set))
  const line = (set: (typeof sets)[number], side: 0 | 1) => set.lines[swap ? 1 - side : side]

  return (
    <div className={styles.sheet}>
      <p className={styles.head}>
        <span className={styles.a}>
          <Link to={`/players/${a.slug}`}>{a.name}</Link>
        </span>{' '}
        v{' '}
        <span className={styles.b}>
          <Link to={`/players/${b.slug}`}>{b.name}</Link>
        </span>
        <span className={styles.when}>
          {' '}
          {match.tournament} {match.round}, {match.played_on}
        </span>
      </p>
      <div className={styles.wrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th className={styles.th} scope="col">
                Per set
              </th>
              {sets.map((set) => (
                <th key={set.set} className={styles.thNum} scope="col">
                  {set.set === 0 ? 'Match' : `Set ${set.set}`}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {ROWS.map((row) => (
              <tr key={row.label} className={styles.row}>
                <th className={styles.label} scope="row">
                  {row.label}
                </th>
                {sets.map((set) => {
                  const fa = line(set, 0)
                  const fb = line(set, 1)
                  return (
                    <td key={set.set} className={styles.cell}>
                      <span className={styles.a}>
                        {fa ? row.value(fa) : <AbsentCell label={row.label} />}
                      </span>{' '}
                      <span className={styles.b}>
                        {fb ? row.value(fb) : <AbsentCell label={row.label} />}
                      </span>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className={styles.note}>
        Charted point by point by a volunteer of the Match Charting Project
        {match.charted_by ? ` (${match.charted_by})` : ''}, CC BY-NC-SA 4.0. The charter&apos;s
        counts, not the tour&apos;s: they sit beside the tour&apos;s figures for this match and
        are in no career total on this site.
      </p>
    </div>
  )
}

interface FigureRow {
  label: string
  value: (f: ChartedFigures) => string
}

/** A rate, or the counts when the set gave the rate no denominator. */
function rate(num: number, den: number): string {
  if (den === 0) return `${num}/${den}`
  return `${Math.round((100 * num) / den)}%`
}

const ROWS: ReadonlyArray<FigureRow> = [
  { label: 'Serve points', value: (f) => String(f.serve_points) },
  { label: 'First serves in', value: (f) => rate(f.first_in, f.serve_points) },
  { label: 'First serve points won', value: (f) => rate(f.first_won, f.first_in) },
  { label: 'Second serve points won', value: (f) => rate(f.second_won, f.second_in) },
  { label: 'Aces', value: (f) => String(f.aces) },
  { label: 'Double faults', value: (f) => String(f.double_faults) },
  { label: 'Break points saved', value: (f) => `${f.bp_saved}/${f.bp_faced}` },
  { label: 'Return points won', value: (f) => rate(f.return_points_won, f.return_points) },
  { label: 'Winners', value: (f) => String(f.winners) },
  { label: 'Unforced errors', value: (f) => String(f.unforced) },
]
