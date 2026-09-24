import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { LeaderRow, LeaderStat, Leaderboard, PlayerSearchResult } from '../api/client'
import { getLeaders } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import { EmptyState, Meta, PageHeader, PlayerSearch, Skeleton, StatTable, SurfaceToggle, TourFilter, type Column } from '../components'
import { formatPercent } from '../lib/format'
import { tierLabel } from '../lib/tier'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Leaders.module.css'
import { breadcrumbs, useJsonLd } from '../lib/jsonld'

const DEFAULT_STAT = 'serve_points_won'
const DEFAULT_FLOOR = 10
const LIMIT = 100

const TIERS = [
  { value: null, label: 'Every tier' },
  { value: 'tour', label: 'Tour' },
  { value: 'challenger', label: 'Challenger' },
  { value: 'futures', label: 'Futures' },
  { value: 'itf', label: 'ITF' },
]

const FAMILIES: Record<string, string> = {
  serve: 'Serve',
  return: 'Return',
  points: 'Points',
  score: 'From the score',
}

/**
 * The leaderboards: one stat at a time, every filter in the URL so a view is
 * a link, one ruled table with the value as the only column in emphasis, and
 * the population above it. A leaderboard is a ranking, so this page sits
 * beside /rankings rather than in the nav.
 */
export function Leaders() {
  const [stat, setStat] = useUrlParam('stat')
  const [tour, setTour] = useUrlParam('tour')
  const [tier, setTier] = useUrlParam('tier')
  const [surface, setSurface] = useUrlParam('surface')
  const [season, setSeason] = useUrlParam('season')
  const [floor, setFloor] = useUrlParam('min')
  const [sought, setSought] = useState<PlayerSearchResult | null>(null)
  const [query, setQuery] = useState('')
  useJsonLd('breadcrumbs', breadcrumbs([{ name: 'Leaders', path: '/leaders' }]))

  const key = stat ?? DEFAULT_STAT
  const minMatches = floor !== null && Number(floor) >= 1 ? Number(floor) : DEFAULT_FLOOR
  const year = season !== null && /^\d{4}$/.test(season) ? Number(season) : null
  const filters = { tour, tier, surface, season: year, min_matches: minMatches, limit: LIMIT }

  const board = useResource(
    (signal) => getLeaders(key, filters, signal),
    [key, tour, tier, surface, year, minMatches],
  )
  // The sought player's own figure, asked for separately so choosing a name
  // does not refetch the board.
  const theirs = useResource<Leaderboard | null>(
    (signal) =>
      sought === null
        ? Promise.resolve(null)
        : getLeaders(key, { ...filters, limit: 1, player: sought.slug }, signal),
    [key, tour, tier, surface, year, minMatches, sought?.slug],
  )

  const stats = board.state === 'ready' ? board.data.stats : []

  return (
    <>
      <PageHeader
        kicker="Serve · return · score"
        title="Leaders"
        accent="b"
        lede={
          <>
            A ranking by what a player did with a serve, a return or a score. The ratings are on{' '}
            <Link className={styles.kin} to="/rankings">
              Rankings
            </Link>
            .
          </>
        }
      />

      <div className={styles.controls}>
        <label className={styles.field} htmlFor="leaders-stat">
          <span className={styles.label}>Stat</span>
          <select
            id="leaders-stat"
            className={styles.select}
            value={key}
            onChange={(event) => setStat(event.target.value === DEFAULT_STAT ? null : event.target.value)}
          >
            {stats.length === 0 ? <option value={key}>{key}</option> : null}
            {Object.keys(FAMILIES).map((family) => (
              <optgroup key={family} label={FAMILIES[family]}>
                {stats
                  .filter((s) => s.family === family)
                  .map((s) => (
                    <option key={s.key} value={s.key}>
                      {s.label}
                    </option>
                  ))}
              </optgroup>
            ))}
          </select>
        </label>
        <TourFilter value={tour} onChange={setTour} />
        <div className={styles.cells} role="group" aria-label="Filter by tier">
          {TIERS.map((option) => (
            <button
              key={option.label}
              type="button"
              className={tier === option.value ? `${styles.cell} ${styles.active}` : styles.cell}
              aria-pressed={tier === option.value}
              onClick={() => setTier(option.value)}
            >
              {option.label}
            </button>
          ))}
        </div>
        <SurfaceToggle value={surface} onChange={setSurface} />
        <div className={styles.numbers}>
          <label className={styles.field} htmlFor="leaders-season">
            <span className={styles.label}>Season</span>
            <input
              id="leaders-season"
              className={styles.input}
              type="number"
              inputMode="numeric"
              min={1900}
              max={2100}
              placeholder="every season"
              value={season ?? ''}
              onChange={(event) => setSeason(event.target.value)}
            />
          </label>
          <label className={styles.field} htmlFor="leaders-floor">
            <span className={styles.label}>At least</span>
            <input
              id="leaders-floor"
              className={styles.input}
              type="number"
              inputMode="numeric"
              min={1}
              max={10000}
              value={floor ?? String(DEFAULT_FLOOR)}
              onChange={(event) => setFloor(event.target.value === String(DEFAULT_FLOOR) ? null : event.target.value)}
            />
            <span className={styles.unit}>matches</span>
          </label>
        </div>
      </div>

      <Board board={board} minMatches={minMatches} />

      {board.state === 'ready' && board.data.data.length > 0 ? (
        <section className={styles.find}>
          <PlayerSearch
            label="Find a player on this board"
            placeholder="A name that is not in the table"
            value={query}
            onChange={setQuery}
            onSelect={(player) => {
              setQuery(player.name)
              setSought(player)
            }}
            tour={tour}
          />
          {sought !== null ? <Sought player={sought} board={board.data} theirs={theirs} minMatches={minMatches} /> : null}
        </section>
      ) : null}
    </>
  )
}

function Board({ board, minMatches }: { board: Resource<Leaderboard>; minMatches: number }) {
  if (board.state === 'loading') return <Skeleton lines={12} />
  if (board.state === 'error') {
    return (
      <p className={styles.error}>
        The board could not be loaded: {board.error.message} The API may not be running; start it
        with <code>make api</code> and reload.
      </p>
    )
  }
  const data = board.data
  const stat = data.stat
  const filterWords = describeFilters(data)

  if (data.data.length === 0) {
    return (
      <>
        <Population data={data} minMatches={minMatches} />
        <EmptyState
          heading={`Nobody clears ${minMatches} matches here`}
          reason={
            data.population.with_stats === 0 && stat.family !== 'score'
              ? `No match ${filterWords} recorded serve statistics, so there is no ${stat.label.toLowerCase()} figure to rank. Serve lines exist for about 17% of matches: the tour level from 1991, Challengers from about 2010, and never at Futures or ITF level.`
              : `${data.population.players} players played ${filterWords} and none has ${minMatches} matches carrying this figure. Lower the floor to see them, with the smaller samples that implies.`
          }
        />
      </>
    )
  }

  return (
    <>
      <Population data={data} minMatches={minMatches} />
      <StatTable
        caption={caption(stat, data)}
        columns={columns(stat)}
        rows={data.data}
        rowKey={(row) => row.slug}
        defaultSort={{ key: 'position', direction: 'asc' }}
      />
    </>
  )
}

/** What the board is a board of, in one line above it. */
function Population({ data, minMatches }: { data: Leaderboard; minMatches: number }) {
  const p = data.population
  const words = describeFilters(data)
  const stats =
    data.stat.family === 'score'
      ? `${p.matches.toLocaleString('en-GB')} finished matches ${words}, ${p.players.toLocaleString('en-GB')} players`
      : `Of ${p.matches.toLocaleString('en-GB')} matches ${words}, ${p.with_stats.toLocaleString('en-GB')} recorded serve statistics`
  return (
    <Meta
      className={styles.population}
      parts={[
        stats,
        `${p.qualified.toLocaleString('en-GB')} ${p.qualified === 1 ? 'player clears' : 'players clear'} ${minMatches} matches`,
      ]}
    />
  )
}

function describeFilters(data: Leaderboard): string {
  const f = data.filters
  const parts = [
    f.tour === null ? 'on both tours' : `on the ${f.tour.toUpperCase()}`,
    f.tier === null ? null : f.tier === 'tour' ? 'at tour level' : `at ${tierLabel(f.tier)?.toLowerCase() ?? f.tier} level`,
    f.surface === null ? null : `on ${f.surface}`,
    f.season === null ? 'in every season' : `in ${f.season}`,
  ]
  return parts.filter((part) => part !== null).join(' ')
}

function caption(stat: LeaderStat, data: Leaderboard): string {
  const family =
    stat.family === 'serve'
      ? "over each player's own serve lines"
      : stat.family === 'return'
        ? "over the opponents' serve lines, which is what a return figure is made of"
        : stat.family === 'points'
          ? 'over the matches where both serve lines were recorded'
          : 'over every finished match whose score could be read'
  const kind =
    stat.kind === 'ratio'
      ? 'Return points won over serve points lost: above 1 is a player who won a larger share of their return points than their opponents won of theirs.'
      : `${stat.label} as a share of ${stat.sample}, ${stat.ascending ? 'fewest first' : 'most first'}.`
  return `${kind} Computed ${family}; the matches column is how many. Floor of ${data.filters.min_matches}.`
}

function Sought({
  player,
  board,
  theirs,
  minMatches,
}: {
  player: PlayerSearchResult
  board: Leaderboard
  theirs: Resource<Leaderboard | null>
  minMatches: number
}) {
  const on = board.data.find((row) => row.slug === player.slug)
  if (on !== undefined) {
    return (
      <p className={styles.found}>
        <Link to={`/players/${player.slug}`}>{player.name}</Link> is on the board at #{on.position}:{' '}
        {formatValue(board.stat, on)} over {on.sample} matches.
      </p>
    )
  }
  if (theirs.state === 'loading') return <Skeleton lines={2} />
  if (theirs.state === 'error') return <p className={styles.error}>{theirs.error.message}</p>
  const row = theirs.data?.player ?? null
  const words = describeFilters(board)
  if (row === null) {
    return (
      <EmptyState
        heading={`${player.name} is not on this board`}
        reason={
          board.stat.family === 'score'
            ? `They played no finished match ${words}.`
            : `None of their matches ${words} recorded the serve statistics this figure is made of, so there is no figure to place. That is an absence, not a low number.`
        }
      />
    )
  }
  if (row.sample < minMatches) {
    return (
      <EmptyState
        heading={`${player.name} is below the floor`}
        reason={`${row.sample} ${row.sample === 1 ? 'match' : 'matches'} carrying this figure ${words}, against a floor of ${minMatches}. Over those, ${formatValue(board.stat, row)}. Lower the floor to see them ranked.`}
      />
    )
  }
  return (
    <p className={styles.found}>
      <Link to={`/players/${player.slug}`}>{player.name}</Link> clears the floor at {formatValue(board.stat, row)}{' '}
      over {row.sample} matches, but is outside the top {board.data.length} shown.
    </p>
  )
}

function formatValue(stat: LeaderStat, row: LeaderRow): string {
  return stat.kind === 'ratio' ? row.value.toFixed(3) : formatPercent(row.value * 100)
}

function columns(stat: LeaderStat): ReadonlyArray<Column<LeaderRow>> {
  return [
    {
      key: 'position',
      header: '#',
      align: 'right',
      value: (row) => row.position,
      render: (row) => (
        <span className={row.position === 1 ? `${styles.position} ${styles.leader}` : styles.position}>
          {row.position}
        </span>
      ),
    },
    {
      key: 'player',
      header: 'Player',
      wrap: true,
      value: (row) => row.name,
      render: (row) => (
        <>
          <Link className={styles.player} to={`/players/${row.slug}`}>
            {row.name}
          </Link>
          <span className={styles.country}>
            {' '}
            {row.tour.toUpperCase()}
            {row.country === null ? '' : ` ${row.country}`}
          </span>
        </>
      ),
    },
    { key: 'sample', header: 'Matches', align: 'right', value: (row) => row.sample },
    {
      key: 'counts',
      header: stat.kind === 'ratio' ? 'Of' : 'Count',
      align: 'right',
      wide: true,
      sortable: false,
      value: (row) => (stat.kind === 'ratio' ? row.matches : (row.numerator ?? null)),
      render: (row) =>
        stat.kind === 'ratio' ? `${row.matches} played` : `${row.numerator?.toLocaleString('en-GB')} / ${row.denominator?.toLocaleString('en-GB')}`,
    },
    {
      key: 'value',
      header: stat.label,
      align: 'right',
      value: (row) => row.value,
      render: (row) => <span className={styles.value}>{formatValue(stat, row)}</span>,
    },
  ]
}
