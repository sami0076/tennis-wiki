import { Link } from 'react-router-dom'
import { getCoverage, getRankings, getTrajectories, simulateDraw } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  AbsentCell,
  ButtonLink,
  SeedingSheet,
  Skeleton,
  StatTable,
  SurfaceDot,
  TourFilter,
  type Column,
} from '../components'
import type {
  CoverageEntry,
  DrawOdds,
  DrawSimulation,
  RankingPage,
  Trajectories,
} from '../api/client'
import { FEATURED_DRAW, roundsReached } from '../lib/featuredDraw'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Home.module.css'

const SEEDS = 8

const columns: ReadonlyArray<Column<CoverageEntry>> = [
  { key: 'tour', header: 'Tour', value: (row) => row.tour.toUpperCase() },
  { key: 'tier', header: 'Tier', value: (row) => row.tier },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  { key: 'first', header: 'From', align: 'right', value: (row) => row.first_match, wide: true },
  { key: 'last', header: 'To', align: 'right', value: (row) => row.last_match, wide: true },
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
 * Home is the front of the sheet: the seeding, what the database holds, and a
 * draw the model replayed and can be checked against.
 *
 * The seeding sheet is the site's one piece of ambient motion, and the coverage
 * table below it is the argument the whole project makes, so neither is
 * decoration.
 */
export function Home() {
  const [tour, setTour] = useUrlParam('tour')
  const coverage = useResource((signal) => getCoverage(signal), [])
  const lines = useResource(
    (signal) => getTrajectories({ tour, players: SEEDS }, signal),
    [tour],
  )
  const leaders = useResource(
    (signal) => getRankings({ type: 'elo', tour, limit: SEEDS }, signal),
    [tour],
  )
  const draw = useResource((signal) => simulateDraw(FEATURED_DRAW, signal), [])

  return (
    <>
      <section className={styles.hero}>
        <div className={styles.head}>
          <div className={styles.headline}>
            <h1 className={styles.title}>Every match, and every gap between them.</h1>
            <p className={styles.standfirst}>
              1.6 million matches across both tours, back to 1922, rated on one Elo scale.
              Where a statistic was never recorded, this site explains which kind of never.
            </p>
          </div>
          <div className={styles.seeding}>
            <TourFilter value={tour} onChange={setTour} />
            <h2 className={styles.sectionTitle}>
              Elo leaders{leaders.state === 'ready' ? `, as of ${leaders.data.as_of}` : null}
            </h2>
          </div>
        </div>
        <Seeding lines={lines} leaders={leaders} />
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
              caption="Queried from the database on every request, so the claim cannot drift from the data. n/r is a tier where serve statistics were never recorded, which is a different thing from a zero."
              columns={columns}
              rows={coverage.data.tiers}
              rowKey={(row) => `${row.tour}-${row.tier}`}
              defaultSort={{ key: 'matches', direction: 'desc' }}
            />
          </>
        ) : null}
      </section>

      <section className={styles.section}>
        <Replay draw={draw} />
      </section>
    </>
  )
}

function Seeding({
  lines,
  leaders,
}: {
  lines: Resource<Trajectories>
  leaders: Resource<RankingPage>
}) {
  if (lines.state === 'loading' || leaders.state === 'loading') {
    return <Skeleton lines={8} />
  }

  if (lines.state === 'error') {
    return (
      <p className={styles.error}>
        The rating lines could not be loaded: {lines.error.message} The API may not be
        running; start it with <code>make api</code> and reload.
      </p>
    )
  }
  // The chart above already said the API is not answering. Saying it twice
  // would make one failure look like two.
  if (leaders.state === 'error') return null

  const rows = leaders.data.data
  const drawable = lines.data.lines.filter((line) => line.points.length > 1)
  if (rows.length === 0 || drawable.length < 2) {
    return (
      <p className={styles.caption}>
        Nothing on that tour has enough rated weeks in this window to draw. The rankings are
        unaffected.
      </p>
    )
  }

  return (
    <>
      <SeedingSheet
        seeds={rows}
        lines={drawable.map((line) => ({
          slug: line.slug,
          name: line.name,
          // The series carries as_of; a chart plots dates.
          points: line.points.map((point) => ({ date: point.as_of, elo: point.elo })),
        }))}
        animate
      />
      <div className={styles.foot}>
        <p className={styles.caption}>
          The {rows.length} highest-rated players to {lines.data.to}, on one shared scale,
          each line stepping across to its seed. Rated only in the weeks they played, which
          is why a line can stop before the edge.
        </p>
        <div className={styles.seedingFoot}>
          <p className={styles.caption}>
            Elo as of {leaders.data.as_of}, the last week that exists rather than today. The
            signed figure is how far the model puts a player from their published rank.
          </p>
          <ButtonLink to="/rankings">See the full rankings</ButtonLink>
        </div>
      </div>
    </>
  )
}

/**
 * The replayed draw, set out as a draw sheet: one column per round, the chance
 * of still being in it written where the score would go, and who actually won
 * marked in the margin.
 */
function Replay({ draw }: { draw: Resource<DrawSimulation> }) {
  if (draw.state === 'loading') {
    return (
      <>
        <h2 className={styles.sectionTitle}>A draw, replayed ten thousand times</h2>
        <Skeleton lines={8} />
      </>
    )
  }
  // The sections above already reported an API that is not answering.
  if (draw.state === 'error') return null

  const sim = draw.data
  const rounds = roundsReached(sim.rounds)
  const shown = [...sim.odds].sort((a, b) => b.title - a.title).slice(0, SEEDS)
  const rest = sim.odds.slice(SEEDS).reduce((sum, o) => sum + o.title, 0)
  const columns: Column<DrawOdds>[] = [
    {
      key: 'seed',
      header: 'Seed',
      value: (row) => row.seed,
      render: (row) => (row.seed === null ? '' : `[${row.seed}]`),
      sortable: false,
    },
    {
      key: 'name',
      header: 'Player',
      value: (row) => row.name,
      render: (row) => (
        <Link className={styles.player} to={`/players/${row.slug}`}>
          {row.name}
        </Link>
      ),
      sortable: false,
    },
    ...rounds.slice(0, -1).map(
      (round, index): Column<DrawOdds> => ({
        key: round,
        header: round,
        align: 'right',
        value: (row) => row.reached[index] ?? null,
        render: (row) => `${((row.reached[index] ?? 0) * 100).toFixed(1)}`,
        sortable: false,
        wide: index < rounds.length - 4,
      }),
    ),
    {
      key: 'title',
      header: 'W',
      align: 'right',
      value: (row) => row.title,
      render: (row) => (
        <span className={styles.title2}>
          {(row.title * 100).toFixed(1)}
          <span className={styles.interval}> ±{(row.title_interval * 100).toFixed(1)}</span>
        </span>
      ),
      sortable: false,
    },
    {
      key: 'won',
      header: '',
      value: (row) => (row.slug === sim.champion ? 1 : 0),
      render: (row) => (row.slug === sim.champion ? <span className={styles.won}>won</span> : ''),
      sortable: false,
    },
  ]

  return (
    <>
      <h2 className={styles.sectionTitle}>
        <SurfaceDot surface={sim.event.surface} label={false} /> {sim.event.name}{' '}
        {sim.event.season}, replayed {sim.runs.toLocaleString()} times
      </h2>
      <p className={styles.standfirst}>
        This draw was played. The ratings are as of {sim.event.ratings_as_of}, the week it
        began, and every figure is the share of runs in which that player was still in the
        draw at that round, with a 95% interval on the title.
      </p>
      <StatTable
        caption={`The columns read like a draw sheet: a row can only fall from left to right. The ${sim.entered - shown.length} players not listed share ${(rest * 100).toFixed(1)}% of the title between them. Seeded, so the same query gives the same answer.`}
        columns={columns}
        rows={shown}
        rowKey={(row) => row.slug}
      />
      <div className={styles.replayFoot}>
        <ButtonLink to="/simulator">Replay a draw</ButtonLink>
      </div>
    </>
  )
}
