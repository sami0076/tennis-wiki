import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  getCoverage,
  getHeadToHead,
  getRankings,
  getRecentFinals,
  getTrajectories,
  simulateDraw,
} from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  AbsentCell,
  ButtonLink,
  Card,
  CountUp,
  CourtArt,
  Flag,
  Note,
  Odometer,
  PageHeader,
  PlayerSearch,
  RankDelta,
  RecentFinals,
  Reveal,
  SeedingSheet,
  Skeleton,
  StatTable,
  SurfaceDot,
  Ticker,
  TourFilter,
  type Column,
  type TickerItem,
} from '../components'
import type {
  RecentFinals as RecentFinalsData,
  CoverageEntry,
  CoverageResponse,
  DrawOdds,
  DrawSimulation,
  HeadToHead,
  RankingPage,
  RankingRow,
  Trajectories,
} from '../api/client'
import { FEATURED_DRAW, roundsReached } from '../lib/featuredDraw'
import { formatElo, surname } from '../lib/format'
import { useJsonLd, website } from '../lib/jsonld'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Home.module.css'

const SEEDS = 8
const TOP = 5

const columns: ReadonlyArray<Column<CoverageEntry>> = [
  { key: 'tour', header: 'Tour', value: (row) => row.tour.toUpperCase() },
  { key: 'tier', header: 'Tier', wrap: true, value: (row) => row.tier },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  { key: 'first', header: 'From', align: 'right', value: (row) => row.first_match, wide: true },
  { key: 'last', header: 'To', align: 'right', value: (row) => row.last_match, wide: true },
  {
    key: 'stats',
    header: 'Serve stats',
    align: 'right',
    // Zero matches with statistics is a real, recorded zero for a tier that
    // never had them -- the percentage is what is genuinely absent.
    value: (row) => (row.matches_with_stats === 0 ? null : row.stats_percentage),
    render: (row) =>
      row.matches_with_stats === 0 ? (
        <AbsentCell label="Serve stats" />
      ) : (
        `${row.stats_percentage.toFixed(1)}%`
      ),
  },
]

/**
 * Home: the search, the top of the ratings, the rivalry between the two at the
 * top, the week's finals, then the leaders' lines, what the database holds and
 * a draw the model replayed.
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
  const recent = useResource((signal) => getRecentFinals(signal), [])
  useJsonLd('website', website())

  const top = leaders.state === 'ready' ? leaders.data.data : []

  return (
    <>
      <PageHeader
        kicker={<Kicker coverage={coverage} />}
        title="Every match, and every gap between them."
        mark="gap"
        lede="1.6 million matches across both tours, back to 1922, rated on one Elo scale. Where a statistic was never recorded, this site explains which kind of never."
        art={<CourtArt cycle />}
      >
        <HeroSearch />
        <TryChips top={top} />
      </PageHeader>

      <Ticker label="Elo leaders and last week's finals" items={tickerItems(top, recent)} />

      <div className={styles.trio}>
        <section className={styles.column}>
          <ColumnHead title="Elo top 5" to="/rankings" link="All rankings" />
          <TopFive leaders={leaders} />
        </section>
        <section className={styles.column}>
          <ColumnHead title="The rivalry at the top" />
          <TopRivalry top={top} />
        </section>
        <section className={styles.column}>
          <ColumnHead title="Last week's finals" />
          <RecentFinals recent={recent} />
        </section>
      </div>

      <Card
        className={styles.block}
        title={
          <>Elo leaders{leaders.state === 'ready' ? `, as of ${leaders.data.as_of}` : null}</>
        }
        aside={<TourFilter value={tour} onChange={setTour} />}
      >
        <Seeding lines={lines} leaders={leaders} />
      </Card>

      <Card className={styles.block} title="What is actually in the database">
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
                <span key={tourName} className={styles.throughChip}>
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
            <p className={styles.seasons}>
              <Link className={styles.columnLink} to="/seasons">
                Browse by season →
              </Link>
            </p>
          </>
        ) : null}
      </Card>

      <Card className={styles.block}>
        <Replay draw={draw} />
      </Card>
    </>
  )
}

function tickerItems(top: ReadonlyArray<RankingRow>, recent: Resource<RecentFinalsData>): TickerItem[] {
  const leaders: TickerItem[] = top.map((row) => ({
    key: `elo-${row.slug}`,
    label: `#${row.position} ${surname(row.name)}`,
    value: row.elo === null ? 'n/r' : formatElo(row.elo),
  }))
  const finals: TickerItem[] =
    recent.state === 'ready'
      ? recent.data.finals.map((final) => ({
          key: `final-${final.tour}-${final.name}`,
          label: `${final.name} final · ${surname(final.champion.name)} d. ${surname(final.finalist.name)}`,
          value: final.final_score ?? '',
        }))
      : []
  return [...leaders, ...finals]
}

function Kicker({ coverage }: { coverage: Resource<CoverageResponse> }) {
  if (coverage.state !== 'ready') return <>Both tours · every level</>
  const tiers = coverage.data.tiers
  const matches = tiers.reduce((sum, tier) => sum + tier.matches, 0)
  const first = tiers.reduce((min, tier) => (tier.first_match < min ? tier.first_match : min), '9999')
  return (
    <>
      {matches.toLocaleString('en-US')} matches · {first.slice(0, 4)} – today
    </>
  )
}

function HeroSearch() {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const submit = (value: string) => {
    if (value.trim() === '') return
    navigate(`/players?q=${encodeURIComponent(value.trim())}`)
  }
  return (
    <form
      className={styles.search}
      role="search"
      onSubmit={(event) => {
        event.preventDefault()
        submit(query)
      }}
    >
      <div className={styles.searchField}>
        <PlayerSearch
          label="Find a player"
          hideLabel
          placeholder="Find a player or a rivalry"
          value={query}
          onChange={setQuery}
          onSelect={(player) => navigate(`/players/${player.slug}`)}
          onSubmit={submit}
        />
      </div>
      <button type="submit" className={styles.searchButton}>
        Search
      </button>
    </form>
  )
}

function TryChips({ top }: { top: ReadonlyArray<RankingRow> }) {
  if (top.length < 2) return null
  const [first, second] = top as [RankingRow, RankingRow]
  const chips = [
    { to: `/h2h/${first.slug}/${second.slug}`, label: `${surname(first.name)} vs ${surname(second.name)}` },
    ...top.slice(2, 5).map((row) => ({ to: `/players/${row.slug}`, label: surname(row.name) })),
  ]
  return (
    <div className={styles.chips}>
      <span className={styles.try}>Try</span>
      {chips.map((chip) => (
        <Link key={chip.to} className={styles.chip} to={chip.to}>
          {chip.label}
        </Link>
      ))}
    </div>
  )
}

function ColumnHead({ title, to, link }: { title: string; to?: string; link?: string }) {
  return (
    <div className={styles.columnHead}>
      <h2 className={styles.columnTitle}>{title}</h2>
      {to !== undefined ? (
        <Link className={styles.columnLink} to={to}>
          {link} →
        </Link>
      ) : null}
    </div>
  )
}

function TopFive({ leaders }: { leaders: Resource<RankingPage> }) {
  if (leaders.state === 'loading') return <Skeleton lines={5} />
  if (leaders.state === 'error') return null
  return (
    <ol className={styles.top}>
      {leaders.data.data.slice(0, TOP).map((row, index) => (
        <Reveal as="li" key={row.slug} delay={index * 70} className={styles.topRow}>
          <span className={styles.topPosition}>{row.position}</span>
          <span className={styles.topName}>
            <Link to={`/players/${row.slug}`}>{row.name}</Link>
            <span className={styles.topCountry}>
              {row.country === null ? (
                row.tour.toUpperCase()
              ) : (
                <Flag country={row.country} />
              )}
            </span>
          </span>
          <span className={styles.topElo}>
            {row.elo === null ? '–' : <CountUp value={row.elo} format={formatElo} />}
          </span>
          <span className={styles.topDelta}>
            {row.delta === null ? null : <RankDelta delta={row.delta} label="places against the official ranking" />}
          </span>
        </Reveal>
      ))}
    </ol>
  )
}

/** The two highest-rated players, and what happened when they met. */
function TopRivalry({ top }: { top: ReadonlyArray<RankingRow> }) {
  const a = top[0]?.slug
  const b = top[1]?.slug
  const h2h = useResource<HeadToHead | null>(
    (signal) => (a === undefined || b === undefined ? Promise.resolve(null) : getHeadToHead(a, b, {}, signal)),
    [a, b],
  )
  if (a === undefined || b === undefined || h2h.state === 'loading') {
    return (
      <div className={styles.rivalry}>
        <Skeleton lines={5} />
      </div>
    )
  }
  if (h2h.state === 'error' || h2h.data === null) return null

  const [playerA, playerB] = h2h.data.players
  const [winsA, winsB] = h2h.data.record.wins
  const total = winsA + winsB
  const finals = h2h.data.meetings.filter((m) => m.round === 'F').length
  const eloA = top[0]?.elo
  const eloB = top[1]?.elo

  return (
    <Card tilt className={styles.rivalry}>
      <div className={styles.rivalryNames}>
        <div>
          <span className={`${styles.dash} ${styles.dashA}`} aria-hidden="true" />
          <Link className={styles.rivalA} to={`/players/${playerA.slug}`}>
            {surname(playerA.name)}
          </Link>
          <div className={styles.rivalMeta}>
            {[playerA.country, eloA != null ? `Elo ${formatElo(eloA)}` : null].filter(Boolean).join(' · ')}
          </div>
        </div>
        <div className={styles.rivalRight}>
          <span className={`${styles.dash} ${styles.dashB}`} aria-hidden="true" />
          <Link className={styles.rivalB} to={`/players/${playerB.slug}`}>
            {surname(playerB.name)}
          </Link>
          <div className={styles.rivalMeta}>
            {[playerB.country, eloB != null ? `Elo ${formatElo(eloB)}` : null].filter(Boolean).join(' · ')}
          </div>
        </div>
      </div>
      <div className={styles.tally} aria-label={`${winsA} to ${winsB}`}>
        <Odometer className={styles.tallyA} value={winsA} />
        <span className={styles.tallyDash} aria-hidden="true" />
        <Odometer className={styles.tallyB} value={winsB} />
      </div>
      <div className={styles.tallyBar} aria-hidden="true">
        <span style={{ flexGrow: total === 0 ? 1 : winsA }} />
        <span style={{ flexGrow: total === 0 ? 1 : winsB }} />
      </div>
      <div className={styles.rivalryFoot}>
        <span>
          {h2h.data.record.matches} {h2h.data.record.matches === 1 ? 'meeting' : 'meetings'}
          {finals > 0 ? ` · ${finals} ${finals === 1 ? 'final' : 'finals'}` : null}
        </span>
        <Link className={styles.columnLink} to={`/h2h/${playerA.slug}/${playerB.slug}`}>
          Compare →
        </Link>
      </div>
    </Card>
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
        <Note>
          <p>
            Elo as of {leaders.data.as_of}, the last week that exists rather than today. The
            signed figure is how far the model puts a player from their published rank.
          </p>
        </Note>
        <ButtonLink to="/rankings">See the full rankings</ButtonLink>
      </div>
    </>
  )
}

/**
 * The replayed draw: one column per round, the chance of still being in it
 * written where the score would go, and who actually won marked in the margin.
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
      wrap: true,
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
        wide: index < rounds.length - 2,
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
      <StatTable
        caption={`This draw was played. Ratings are as of ${sim.event.ratings_as_of}, the week it began; each figure is the share of runs in which that player was still in the draw at that round, with a 95% interval on the title. The ${sim.entered - shown.length} players not listed share ${(rest * 100).toFixed(1)}% of the title between them.`}
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
