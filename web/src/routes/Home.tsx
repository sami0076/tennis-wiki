import { Link } from 'react-router-dom'
import { getCoverage, getRankings, getTrajectories } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  AbsentCell,
  ButtonLink,
  RankDelta,
  Skeleton,
  StatTable,
  TourFilter,
  TrajectoryChart,
  type Column,
} from '../components'
import type { CoverageEntry, RankingPage, Trajectories } from '../api/client'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Home.module.css'

const columns: ReadonlyArray<Column<CoverageEntry>> = [
  { key: 'tour', header: 'Tour', value: (row) => row.tour.toUpperCase() },
  { key: 'tier', header: 'Tier', value: (row) => row.tier },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  { key: 'first', header: 'From', align: 'right', value: (row) => row.first_match },
  { key: 'last', header: 'To', align: 'right', value: (row) => row.last_match },
  {
    key: 'stats',
    header: 'With serve stats',
    align: 'right',
    // Zero matches with statistics is a real, recorded zero for a tier that
    // never had them -- the percentage is what is genuinely absent.
    value: (row) => (row.matches_with_stats === 0 ? null : row.stats_percentage),
    render: (row) =>
      row.matches_with_stats === 0 ? (
        <AbsentCell label="With serve stats" />
      ) : (
        `${row.stats_percentage.toFixed(1)}%`
      ),
  },
]

/**
 * Home is the leaders and the coverage: what the ratings say at the top, and
 * what the database actually holds underneath it.
 *
 * The chart is the site's one piece of ambient motion, and the coverage table
 * below it is the argument the whole project makes, so neither is decoration.
 */
export function Home() {
  const [tour, setTour] = useUrlParam('tour')
  const coverage = useResource((signal) => getCoverage(signal), [])
  const lines = useResource((signal) => getTrajectories({ tour, players: 8 }, signal), [tour])
  const leaders = useResource(
    (signal) => getRankings({ type: 'elo', tour, limit: 5 }, signal),
    [tour],
  )

  return (
    <>
      <section className={styles.hero}>
        <h1 className={styles.title}>Every match, and every gap between them.</h1>
        <p className={styles.standfirst}>
          1.6 million matches across both tours, back to 1922, rated on one Elo scale. Where
          a statistic was never recorded, this site explains which kind of never.
        </p>
        <TourFilter value={tour} onChange={setTour} />
        <Hero lines={lines} />
        <Leaders leaders={leaders} />
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>What is actually in the database</h2>

        {coverage.state === 'loading' ? <Skeleton lines={6} /> : null}

        {coverage.state === 'error' ? (
          <p className={styles.error}>
            The coverage figures could not be loaded: {coverage.error.message} The API may not
            be running; start it with <code>make api</code> and reload.
          </p>
        ) : null}

        {coverage.state === 'ready' ? (
          <>
            <div className={styles.through}>
              {Object.entries(coverage.data.current_through).map(([tourName, date]) => (
                <span key={tourName}>
                  <span className={styles.throughLabel}>{tourName.toUpperCase()} through </span>
                  {date}
                </span>
              ))}
            </div>
            <StatTable
              caption="Queried from the database on every request, so the claim cannot drift from the data."
              columns={columns}
              rows={coverage.data.tiers}
              rowKey={(row) => `${row.tour}-${row.tier}`}
              defaultSort={{ key: 'matches', direction: 'desc' }}
            />
          </>
        ) : null}
      </section>
    </>
  )
}

function Hero({ lines }: { lines: Resource<Trajectories> }) {
  if (lines.state === 'loading') return <Skeleton lines={4} />

  if (lines.state === 'error') {
    return (
      <p className={styles.error}>
        The rating lines could not be loaded: {lines.error.message} The API may not be
        running; start it with <code>make api</code> and reload.
      </p>
    )
  }

  const drawable = lines.data.lines.filter((line) => line.points.length > 1)
  if (drawable.length < 2) {
    return (
      <p className={styles.caption}>
        Nothing on that tour has enough rated weeks in this window to draw. The rankings
        below are unaffected.
      </p>
    )
  }

  return (
    <>
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
        The eight highest-rated players to {lines.data.to}, on one shared scale. Rated only
        in the weeks they played, which is why the lines stop and start.
      </p>
    </>
  )
}

function Leaders({ leaders }: { leaders: Resource<RankingPage> }) {
  if (leaders.state === 'loading') return <Skeleton lines={5} />
  // The chart above already said the API is not answering. Saying it twice
  // would make one failure look like two.
  if (leaders.state === 'error') return null

  const rows = leaders.data.data
  if (rows.length === 0) return null

  return (
    <div className={styles.leaders}>
      <ol className={styles.strip}>
        {rows.map((row) => (
          <li key={row.slug} className={styles.leader}>
            <span className={styles.position}>{row.position}</span>
            <Link className={styles.name} to={`/players/${row.slug}`}>
              {row.name}
            </Link>
            <span className={styles.elo}>{Math.round(row.elo ?? 0)}</span>
            {row.delta === null ? null : <RankDelta delta={row.delta} label="on rank" />}
          </li>
        ))}
      </ol>
      <p className={styles.caption}>
        Elo as of {leaders.data.as_of}, the last week that exists rather than today. The
        signed figure is how far the model puts a player from their published rank.
      </p>
      <ButtonLink to="/rankings">See the full rankings</ButtonLink>
    </div>
  )
}
