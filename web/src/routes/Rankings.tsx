import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { RankingPage, RankingRow, Trajectories } from '../api/client'
import { getRankings, getTrajectories } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  Button,
  EmptyState,
  Meta,
  RankDelta,
  Skeleton,
  StatTable,
  SurfaceToggle,
  TourFilter,
  TrajectoryChart,
  type Column,
} from '../components'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Rankings.module.css'

const TYPES = [
  { value: 'elo', label: 'Elo' },
  { value: 'official', label: 'Official' },
]

/**
 * The two leaderboards, in one table rather than two pages: the comparison
 * between a rating and a published rank is the reason this page exists, and the
 * RankDelta column is where it lands.
 *
 * Everything a reader chooses is in the URL, so a leaderboard is a link.
 */
export function Rankings() {
  const [type, setType] = useUrlParam('type')
  const [tour, setTour] = useUrlParam('tour')
  const [surface, setSurface] = useUrlParam('surface')
  const [date, setDate] = useUrlParam('date')
  const [cursors, setCursors] = useState<string[]>([])

  const kind = type === 'official' ? 'official' : 'elo'
  // The tours publish one list each, so a surface over the official type is a
  // question the data cannot answer. The API says so with a 400; this page does
  // not offer it in the first place.
  const chosen = kind === 'elo' ? surface : null
  const cursor = cursors.at(-1) ?? null

  const page = useResource(
    (signal) => getRankings({ type: kind, tour, surface: chosen, date, cursor, limit: 25 }, signal),
    [kind, tour, chosen, date, cursor],
  )
  // The leaders' lines end at the last week that exists rather than at a week a
  // date filter asked for, so the chart is only honest without one.
  const lines = useResource<Trajectories | null>(
    (signal) =>
      date !== null
        ? Promise.resolve(null)
        : getTrajectories({ surface: chosen, tour, players: 8 }, signal),
    [chosen, tour, date],
  )

  // Any change to the question invalidates a cursor into the answer before it.
  function ask(next: () => void) {
    setCursors([])
    next()
  }

  return (
    <>
      <h1 className={styles.title}>Rankings</h1>

      <div className={styles.controls}>
        <div className={styles.types} role="group" aria-label="Ranking type">
          {TYPES.map((option) => (
            <button
              key={option.value}
              type="button"
              className={kind === option.value ? `${styles.type} ${styles.active}` : styles.type}
              aria-pressed={kind === option.value}
              onClick={() => ask(() => setType(option.value === 'elo' ? null : option.value))}
            >
              {option.label}
            </button>
          ))}
        </div>
        <TourFilter value={tour} onChange={(next) => ask(() => setTour(next))} />
        <label className={styles.dateLabel} htmlFor="rankings-date">
          As of
          <input
            id="rankings-date"
            className={styles.date}
            type="date"
            value={date ?? ''}
            onChange={(event) => ask(() => setDate(event.target.value))}
          />
        </label>
      </div>

      {kind === 'elo' ? (
        <SurfaceToggle value={surface} onChange={(next) => ask(() => setSurface(next))} />
      ) : (
        <p className={styles.caption}>
          The tours publish one list, so this one has no surface split. The Elo list does.
        </p>
      )}

      {kind === 'elo' && date === null ? <Leaders lines={lines} /> : null}

      <Board
        page={page}
        elo={kind === 'elo'}
        surface={chosen}
        cursors={cursors}
        setCursors={setCursors}
        clearDate={() => ask(() => setDate(null))}
      />
    </>
  )
}

function Leaders({ lines }: { lines: Resource<Trajectories | null> }) {
  if (lines.state !== 'ready' || lines.data === null) return null
  const drawable = lines.data.lines.filter((line) => line.points.length > 1)
  if (drawable.length < 2) return null

  return (
    <section className={styles.chart}>
      <TrajectoryChart
        lines={drawable.map((line) => ({
          name: line.name,
          position: line.position,
          // The series carries as_of; a chart plots dates.
          points: line.points.map((point) => ({ date: point.as_of, elo: point.elo })),
        }))}
        animate
      />
      <p className={styles.caption}>
        The top eight to {lines.data.to}, on one shared scale, so a line crossing another is
        a lead changing hands rather than two charts drawn at different sizes.
      </p>
    </section>
  )
}

interface BoardProps {
  page: Resource<RankingPage>
  elo: boolean
  surface: string | null
  cursors: string[]
  setCursors: (update: (current: string[]) => string[]) => void
  clearDate: () => void
}

function Board({ page, elo, surface, cursors, setCursors, clearDate }: BoardProps) {
  if (page.state === 'loading') return <Skeleton lines={10} />

  if (page.state === 'error') {
    return (
      <p className={styles.error}>
        The rankings could not be loaded: {page.error.message} The API may not be running;
        start it with <code>make api</code> and reload.
      </p>
    )
  }

  const data = page.data

  if (data.data.length === 0) {
    return (
      <EmptyState
        heading="Nobody was ranked that week"
        reason={
          data.requested === null
            ? 'No ratings exist for that combination of tour and surface. Carpet is the usual reason: it left the tour in 2009 and nobody has been rated on it since.'
            : `Coverage does not reach ${data.requested}. Ratings run from 1922, and the published lists start a good deal later than that.`
        }
        action={<Button onClick={clearDate}>Show the latest list</Button>}
      />
    )
  }

  return (
    <>
      <Meta
        parts={[
          elo ? 'Elo' : 'Published',
          data.tour === null ? 'Both tours' : data.tour.toUpperCase(),
          elo ? surfaceName(surface) : null,
          `as of ${data.as_of}`,
        ]}
      />
      {data.requested !== null ? (
        <p className={styles.caption}>
          You asked for {data.requested}. This is the nearest week at or before it that
          exists, which is what every ranking here is: as of a week that happened, never as
          of today.
        </p>
      ) : null}

      <StatTable
        caption={
          elo
            ? 'Elo against the published rank, and the distance between them.'
            : 'The published list, with the rating this site gives the same players.'
        }
        columns={elo ? eloColumns : officialColumns}
        rows={data.data}
        rowKey={(row) => row.slug}
        defaultSort={{ key: 'position', direction: 'asc' }}
      />

      <p className={styles.caption}>
        {elo
          ? 'A rating more than a year old drops out of this list. Without that rule it would be a list of the retired, led by players who stopped in 2005. '
          : ''}
        Sorting reorders the {data.data.length} rows on this page rather than the list behind
        them.
      </p>

      <div className={styles.more}>
        {data.next_cursor !== null && data.next_cursor !== '' ? (
          <Button onClick={() => setCursors((current) => [...current, next(data)])}>
            Show the next 25
          </Button>
        ) : null}
        {cursors.length > 0 ? (
          <Button onClick={() => setCursors(() => [])}>Back to the top of the list</Button>
        ) : null}
      </div>
    </>
  )
}

/** Narrowed where it is used, so the button does not have to assert it. */
function next(page: RankingPage): string {
  return page.next_cursor ?? ''
}

function surfaceName(surface: string | null): string {
  if (surface === null) return 'All surfaces'
  return `${surface.slice(0, 1).toUpperCase()}${surface.slice(1)}`
}

const position: Column<RankingRow> = {
  key: 'position',
  header: '#',
  align: 'right',
  value: (row) => row.position,
}

const player: Column<RankingRow> = {
  key: 'player',
  header: 'Player',
  value: (row) => row.name,
  render: (row) => (
    <>
      <Link className={styles.player} to={`/players/${row.slug}`}>
        {row.name}
      </Link>
      {row.country === null ? null : <span className={styles.country}>{row.country}</span>}
    </>
  ),
}

const rating: Column<RankingRow> = {
  key: 'elo',
  header: 'Elo',
  align: 'right',
  value: (row) => row.elo,
  render: (row) => Math.round(row.elo ?? 0),
}

const peak: Column<RankingRow> = {
  key: 'peak',
  header: 'Peak',
  align: 'right',
  wide: true,
  value: (row) => row.peak_elo,
  render: (row) => Math.round(row.peak_elo ?? 0),
}

const age: Column<RankingRow> = {
  key: 'age',
  header: 'Age',
  align: 'right',
  wide: true,
  value: (row) => row.age,
}

const published: Column<RankingRow> = {
  key: 'official',
  header: 'Published',
  align: 'right',
  value: (row) => row.official_rank,
}

// Elo only. Under the published type this list's position is the published
// rank, so the delta would be a number compared with itself.
const delta: Column<RankingRow> = {
  key: 'delta',
  header: 'Against rank',
  align: 'right',
  value: (row) => row.delta,
  render: (row) => <RankDelta delta={row.delta ?? 0} />,
}

const matches: Column<RankingRow> = {
  key: 'matches',
  header: 'Matches',
  align: 'right',
  wide: true,
  value: (row) => row.matches,
}

const points: Column<RankingRow> = {
  key: 'points',
  header: 'Points',
  align: 'right',
  value: (row) => row.points,
}

const eloColumns: ReadonlyArray<Column<RankingRow>> = [
  position,
  player,
  rating,
  peak,
  published,
  delta,
  age,
  matches,
]

const officialColumns: ReadonlyArray<Column<RankingRow>> = [
  position,
  player,
  points,
  rating,
  peak,
  age,
]
