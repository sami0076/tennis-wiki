import { Link, useParams } from 'react-router-dom'
import type { SeasonEvent, SeasonEventsResponse } from '../api/client'
import { getSeasonEvents } from '../api/endpoints'
import { useResource } from '../api/useResource'
import { EmptyState, Meta, Score, Skeleton, StatTable, SurfaceDot, TourFilter, type Column } from '../components'
import { formatScore } from '../lib/format'
import { tierLabel } from '../lib/tier'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Season.module.css'

const TIERS = [
  { value: null, label: 'Tour' },
  { value: 'challenger', label: 'Challenger' },
  { value: 'futures', label: 'Futures' },
  { value: 'itf', label: 'ITF' },
]

const CATEGORY_WORDS: Record<string, string> = {
  slam: 'Grand Slams',
  masters: 'Masters',
  finals: 'Tour finals',
  olympics: 'Olympics',
  team: 'Team competitions',
  tour: 'Tour level',
  challenger: 'Challengers',
  futures: 'Futures',
  itf: 'ITF',
}

const CATEGORY_ORDER = ['slam', 'masters', 'finals', 'olympics', 'team', 'tour', 'challenger', 'futures', 'itf']

/**
 * One year: every event of it, grouped by level, with the champion, the
 * finalist and the score, each a link to its sheet. A tier switch, because a
 * year at Challenger level is a different and longer list, and it should be
 * reachable rather than silently cut. The tour and tier live in the URL.
 */
export function Season() {
  const { year = '' } = useParams()
  const season = Number(year)
  const [tour, setTour] = useUrlParam('tour')
  const [tier, setTier] = useUrlParam('tier')
  const events = useResource((signal) => getSeasonEvents(season, { tour, tier }, signal), [season, tour, tier])

  return (
    <>
      <p className={styles.path}>
        <Link to="/tournaments">Tournaments</Link>
        {'  /  '}
        <Link to="/seasons">Seasons</Link>
      </p>
      <h1 className={styles.title}>{year}</h1>
      <div className={styles.controls}>
        <TourFilter value={tour} onChange={setTour} />
        <div className={styles.tiers} role="group" aria-label="Filter by tier">
          {TIERS.map((option) => (
            <button
              key={option.label}
              type="button"
              className={tier === option.value ? `${styles.tier} ${styles.active}` : styles.tier}
              aria-pressed={tier === option.value}
              onClick={() => setTier(option.value)}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      {events.state === 'loading' ? (
        <Skeleton lines={12} />
      ) : events.state === 'error' ? (
        <p className={styles.error}>
          The year could not be loaded: {events.error.message} The API may not be running; start
          it with <code>make api</code> and reload.
        </p>
      ) : (
        <Calendar data={events.data} />
      )}
    </>
  )
}

function Calendar({ data }: { data: SeasonEventsResponse }) {
  const partial = Object.entries(data.partial)
    .sort()
    .map(([tour, through]) => `the ${tour.toUpperCase()}'s through ${through}`)

  if (data.events.length === 0) {
    return (
      <EmptyState
        heading={`Nothing at ${tierLabel(data.tier) ?? data.tier} level in ${data.season}`}
        reason={
          data.tier === 'tour'
            ? 'The men’s files start in 1968 and the women’s in 1923; before either there is no calendar to show.'
            : 'The files carry this level for some years and not others: Futures from 1998, the ITF circuit for the women from the 1990s, and neither below the tour after 2021 for the WTA.'
        }
      />
    )
  }

  const groups = CATEGORY_ORDER.map((category) => ({
    category,
    events: data.events.filter((event) => event.category === category),
  })).filter((group) => group.events.length > 0)

  return (
    <>
      <Meta
        parts={[
          data.tour === null ? 'Both tours' : data.tour.toUpperCase(),
          tierLabel(data.tier),
          `${data.events.length} ${data.events.length === 1 ? 'event' : 'events'}`,
        ]}
      />
      {partial.length > 0 ? (
        <p className={styles.partial}>
          A year in progress: complete {partial.join(' and ')}, not to the end of the year.
        </p>
      ) : null}
      {groups.map((group) => (
        <section key={group.category} className={styles.block}>
          <h2 className={styles.heading}>{CATEGORY_WORDS[group.category] ?? group.category}</h2>
          <StatTable
            caption={`${group.events.length} ${group.events.length === 1 ? 'event' : 'events'} in calendar order; a name opens its sheet.`}
            columns={group.category === 'team' ? teamColumns : columns}
            rows={group.events}
            rowKey={(row) => `${row.tour}-${row.slug ?? row.name}-${row.start_date}`}
            defaultSort={{ key: 'date', direction: 'asc' }}
          />
        </section>
      ))}
    </>
  )
}

const date: Column<SeasonEvent> = { key: 'date', header: 'From', value: (row) => row.start_date }

const name: Column<SeasonEvent> = {
  key: 'name',
  header: 'Event',
  wrap: true,
  value: (row) => row.name,
  render: (row) => (
    <>
      {row.slug === null ? (
        row.name
      ) : (
        <Link className={styles.event} to={`/tournaments/${row.slug}/${row.start_date.slice(0, 4)}`}>
          {row.name}
        </Link>
      )}
      <span className={styles.tour}> {row.tour.toUpperCase()}</span>
    </>
  ),
}

const surface: Column<SeasonEvent> = {
  key: 'surface',
  header: 'Surface',
  wide: true,
  value: (row) => row.surface,
  render: (row) => <SurfaceDot surface={row.surface} />,
}

const draw: Column<SeasonEvent> = { key: 'draw', header: 'Draw', align: 'right', wide: true, value: (row) => row.draw_size }

const champion: Column<SeasonEvent> = {
  key: 'champion',
  header: 'Champion',
  wrap: true,
  value: (row) => row.champion?.name ?? null,
  render: (row) =>
    row.champion === null ? null : (
      <Link className={styles.player} to={`/players/${row.champion.slug}`}>
        {row.champion.name}
      </Link>
    ),
}

const finalist: Column<SeasonEvent> = {
  key: 'finalist',
  header: 'Finalist',
  wrap: true,
  wide: true,
  value: (row) => row.finalist?.name ?? null,
  render: (row) =>
    row.finalist === null ? null : (
      <Link className={styles.player} to={`/players/${row.finalist.slug}`}>
        {row.finalist.name}
      </Link>
    ),
}

const score: Column<SeasonEvent> = {
  key: 'score',
  header: 'Score',
  wrap: true,
  wide: true,
  minWidth: '12ch',
  value: (row) => formatScore(row.final_score),
  render: (row) => <Score score={row.final_score} />,
}

const columns: ReadonlyArray<Column<SeasonEvent>> = [date, name, surface, draw, champion, finalist, score]

const teamColumns: ReadonlyArray<Column<SeasonEvent>> = [
  date,
  name,
  { key: 'ties', header: 'Ties', align: 'right', value: (row) => row.ties },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
]
