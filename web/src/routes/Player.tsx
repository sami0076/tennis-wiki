import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ApiError,
  type Career,
  type Clutch,
  type ClutchMetric,
  type Page,
  type PlayerMatch,
  type PlayerProfile,
  type RankingHistory,
} from '../api/client'
import {
  getCoverage,
  getPlayer,
  getPlayerClutch,
  getPlayerHighlights,
  getPlayerMatches,
  getPlayerRankings,
  getPlayerRatingSeries,
  getPlayerSeasons,
} from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  AbsentCell,
  AreaChart,
  ButtonLink,
  Button,
  Card,
  ChartedMark,
  Odometer,
  FormPills,
  Kicker,
  Note,
  ChartedSheet,
  EmptyState,
  EventLink,
  Meta,
  PartialAggregate,
  RankDelta,
  Score,
  SectionRail,
  Skeleton,
  StatRow,
  StatTable,
  SurfaceDot,
  SurfaceEloStrip,
  SurfaceToggle,
  WinLossMark,
  type Column,
  type RailItem,
} from '../components'
import { absenceReason, hasStatistics } from '../lib/absence'
import { ageOn, careerSpan, formatElo, formatHand, formatPercent, surname } from '../lib/format'
import { tierLabel } from '../lib/tier'
import { breadcrumbs, person, useJsonLd } from '../lib/jsonld'
import { useUrlParam } from '../lib/useUrlParam'
import { OpponentsSection, SeasonsSection } from './PlayerSplits'
import {
  BestWinsSection,
  RivalsSection,
  RoundsSection,
  RunsSection,
} from './PlayerHighlights'
import { rememberPlayer } from '../lib/useRecentPlayers'
import styles from './Player.module.css'

/**
 * A career is "running" if its last match is inside two years of where the data
 * ends, not of today. The coverage gap is real -- full-schema WTA data stops in
 * 2021 -- so measuring against now would retire every WTA player on the site.
 */
const ACTIVE_WINDOW_YEARS = 2

function stillPlaying(career: Career | null, currentThrough: string | undefined): boolean {
  if (career === null) return false
  const end = currentThrough ?? career.last_match
  const cutoff = new Date(end)
  if (Number.isNaN(cutoff.getTime())) return true
  cutoff.setUTCFullYear(cutoff.getUTCFullYear() - ACTIVE_WINDOW_YEARS)
  return new Date(career.last_match) >= cutoff
}

export function Player() {
  const { slug = '' } = useParams()
  const [surface, setSurface] = useUrlParam('surface')
  const [cursors, setCursors] = useState<string[]>([])

  const profile = useResource((signal) => getPlayer(slug, signal), [slug])
  const coverage = useResource((signal) => getCoverage(signal), [])
  const trajectory = useResource((signal) => getPlayerRatingSeries(slug, {}, signal), [slug])
  const rankings = useResource((signal) => getPlayerRankings(slug, signal), [slug])
  const clutch = useResource((signal) => getPlayerClutch(slug, signal), [slug])
  const seasons = useResource((signal) => getPlayerSeasons(slug, signal), [slug])
  const highlights = useResource((signal) => getPlayerHighlights(slug, signal), [slug])
  useJsonLd('person', profile.state === 'ready' ? person(profile.data) : null)
  useJsonLd(
    'breadcrumbs',
    profile.state === 'ready'
      ? breadcrumbs([{ name: 'Players', path: '/players' }, { name: profile.data.name, path: `/players/${slug}` }])
      : null,
  )
  const matches = useResource(
    (signal) =>
      getPlayerMatches(slug, { surface, limit: 25, cursor: cursors.at(-1) ?? null }, signal),
    [slug, surface, cursors.length],
  )
  const form = useResource((signal) => getPlayerMatches(slug, { limit: 10 }, signal), [slug])

  // The palette's list of recent players is what was actually read, so it is
  // written here rather than when a search result is clicked: a name that was
  // searched for and abandoned is not somewhere this browser has been.
  const loadedName = profile.state === 'ready' ? profile.data.name : null
  const loadedTour = profile.state === 'ready' ? profile.data.tour : null
  useEffect(() => {
    if (loadedName === null || loadedTour === null) return
    rememberPlayer({ slug, name: loadedName, tour: loadedTour })
  }, [slug, loadedName, loadedTour])

  if (profile.state === 'loading') {
    return (
      <>
        <div className={styles.identity}>
          <Skeleton lines={2} width="60%" />
        </div>
        <Skeleton lines={8} />
      </>
    )
  }

  if (profile.state === 'error') {
    const notFound = profile.error instanceof ApiError && profile.error.status === 404
    return (
      <EmptyState
        heading={notFound ? 'No player has that address' : 'That player could not be loaded'}
        reason={
          notFound
            ? `Nothing in the database answers to "${slug}". The name may be spelled differently in the source data.`
            : profile.error.message
        }
        action={<ButtonLink to="/">Back to coverage</ButtonLink>}
      />
    )
  }

  const player = profile.data
  const through =
    coverage.state === 'ready' ? coverage.data.current_through[player.tour] : undefined
  const active = stillPlaying(player.career, through)

  const sections = railFor(player)

  return (
    <>
      <PlayerHero player={player} active={active} rankings={rankings} form={form} />

      {player.career === null ? null : <CareerTiles career={player.career} />}

      {player.career === null ? null : <SectionRail items={sections} label="This career" />}

      <div id="rating" data-anchor className={styles.charts}>
        {trajectory.state === 'ready' && trajectory.data.points.length > 1 ? (
          <Card
            title="Rating history"
            aside={`Weekly · ${trajectory.data.from.slice(0, 4)} – ${trajectory.data.to.slice(0, 4)}`}
          >
            <AreaChart
              points={trajectory.data.points.map((p) => ({ date: p.as_of, elo: p.elo }))}
              label={`Overall Elo from ${trajectory.data.from} to ${trajectory.data.to}`}
            />
          </Card>
        ) : null}
        {player.ratings === null || player.ratings.every((s) => s.surface === 'overall') ? null : (
          <Card title="Elo by surface" delay={120}>
            <SurfaceEloStrip series={player.ratings} mode={active ? 'current' : 'peak'} overall={false} />
          </Card>
        )}
      </div>

      {player.career === null ? (
        <EmptyState
          heading="No matches in the database"
          reason={
            <>
              This player exists in the tour&apos;s own records but none of their matches are in
              the sources this site ingests. That is a gap in the data rather than a career
              that did not happen.
            </>
          }
          action={<ButtonLink to="/">See what the database covers</ButtonLink>}
        />
      ) : (
        <>
          <div id="highlights" data-anchor className={styles.stack}>
            <RunsSection highlights={highlights} />
            <BestWinsSection highlights={highlights} />
          </div>

          <div id="serve" data-anchor className={styles.pair}>
            <ServeSection player={player} />
            <ReturnSection player={player} />
            <PointsSection player={player} />
          </div>

          <div id="pressure" data-anchor className={styles.pair}>
            <ClutchSection clutch={clutch} />
            <RankingSection rankings={rankings} />
          </div>

          <div id="opponents" data-anchor className={styles.pair}>
            {player.splits === null ? null : <OpponentsSection splits={player.splits} />}
            <RivalsSection highlights={highlights} slug={slug} />
          </div>

          <div id="draws" data-anchor className={styles.pair}>
            <RoundsSection highlights={highlights} />
            <SplitsSection career={player.career} />
          </div>
        </>
      )}

      {player.career === null ? null : (
        <div id="seasons" data-anchor>
          <SeasonsSection seasons={seasons} />
        </div>
      )}

      {player.career === null ? null : (
        <div id="matches" data-anchor>
          <MatchesSection
            matches={matches}
            slug={slug}
            surface={surface}
            onSurface={(next) => {
              setCursors([])
              setSurface(next)
            }}
            onMore={(cursor) => setCursors((current) => [...current, cursor])}
            paged={cursors.length > 0}
            onFirst={() => setCursors([])}
          />
        </div>
      )}
    </>
  )
}

/**
 * The rail's entries, in the order the page runs. Serve and return share one
 * entry because they share one row, and a career with no statistics at all
 * still gets it: what the page shows there is why there are none, which is
 * worth being able to jump to.
 */
function railFor(player: PlayerProfile): ReadonlyArray<RailItem> {
  const items: RailItem[] = [{ id: 'rating', label: 'Rating' }]
  items.push({ id: 'highlights', label: 'Runs and best wins' })
  items.push({ id: 'serve', label: 'Serve and return' })
  items.push({ id: 'pressure', label: 'Under pressure' })
  if (player.splits !== null) items.push({ id: 'opponents', label: 'Opponents' })
  items.push({ id: 'draws', label: 'Draws and surfaces' })
  items.push({ id: 'seasons', label: 'Year by year' })
  items.push({ id: 'matches', label: 'Every match' })
  return items
}

function PlayerHero({
  player,
  active,
  rankings,
  form,
}: {
  player: PlayerProfile
  active: boolean
  rankings: Resource<RankingHistory>
  form: Resource<Page<PlayerMatch>>
}) {
  const career = player.career
  // An age while the career is running, its span once it is over. Both answer
  // "when was this player" and only one of them is right at a time.
  const when =
    active && player.birth_date !== null && career !== null
      ? ageOn(player.birth_date, career.last_match)
      : null
  const span = career !== null && !active ? careerSpan(career.first_match, career.last_match) : null
  const overall = player.ratings?.find((s) => s.surface === 'overall') ?? null
  const latest =
    rankings.state === 'ready' && active ? rankings.data.points[rankings.data.points.length - 1] : undefined
  const recent = form.state === 'ready' ? form.data.data.slice(0, 10) : []
  const rival = recent[0]?.opponent

  return (
    <div className={styles.hero}>
      <div className={styles.identity}>
        <div className={styles.kicker}>
          <span className={styles.dash} aria-hidden="true" />
          <Meta
            parts={[
              player.country,
              formatHand(player.hand),
              when,
              span,
              player.pro_since !== null ? `turned pro ${player.pro_since}` : null,
            ]}
          />
        </div>
        <h1 className={styles.name}>{player.name}</h1>
        <div className={styles.chips}>
          <span className={`${styles.chip} ${styles.chipTour}`}>{player.tour.toUpperCase()}</span>
          {latest !== undefined ? (
            <span className={styles.chip}>
              Ranked #{latest.rank}
            </span>
          ) : null}
          {rival !== undefined ? (
            <Link className={styles.chip} to={`/h2h/${player.slug}/${rival.slug}`}>
              Compare with {surname(rival.name)} →
            </Link>
          ) : null}
        </div>
      </div>

      {overall === null ? null : (
        <Card className={styles.eloCard} delay={100}>
          <Kicker>{active ? 'Elo rating' : 'Peak Elo'}</Kicker>
          <div className={styles.eloRow}>
            <Odometer className={styles.elo} value={Math.round(active ? overall.current.elo : overall.peak.elo)} />
            <div className={styles.eloAside}>
              <Kicker>{active ? 'Peak' : 'Last rated'}</Kicker>
              <div className={styles.eloAsideValue}>
                {formatElo(active ? overall.peak.elo : overall.current.elo)}
              </div>
              <div className={styles.eloAsideDate}>{active ? overall.peak.as_of : overall.current.as_of}</div>
            </div>
          </div>
          {recent.length > 0 ? (
            <>
              <Kicker className={styles.formLabel}>Last {recent.length}</Kicker>
              <FormPills results={recent.map((m) => m.won).reverse()} size="lg" />
            </>
          ) : null}
        </Card>
      )}
    </div>
  )
}

function CareerTiles({ career }: { career: Career }) {
  const tiles = [
    { label: 'Career record', value: `${career.wins}-${career.losses}` },
    { label: 'Win rate', value: formatPercent(career.win_percentage) },
    { label: 'Titles', value: String(career.titles), count: career.titles },
    { label: 'Majors', value: String(career.majors), count: career.majors },
  ]
  return (
    <div className={styles.tiles}>
      {tiles.map((tile, index) => (
        <Card key={tile.label} tilt className={styles.tile} delay={index * 80}>
          <Kicker>{tile.label}</Kicker>
          <div className={styles.tileValue}>
            {'count' in tile && tile.count !== undefined ? <Odometer value={tile.count} /> : tile.value}
          </div>
        </Card>
      ))}
    </div>
  )
}

/**
 * The panel the design has and the page shipped without.
 *
 * "+4" against an unnamed average is a number pretending to be a fact, so the
 * population is in the response and the caption states it: this player's own
 * levels and decades, weighted by where their matches actually fell.
 */
function ClutchSection({ clutch }: { clutch: Resource<Clutch> }) {
  if (clutch.state === 'loading') {
    return (
      <Card title="Under pressure, vs tour average">
        <Skeleton lines={3} />
      </Card>
    )
  }
  if (clutch.state === 'error') return null

  const data = clutch.data
  const nothing =
    data.break_points_saved === null &&
    data.tiebreaks_won === null &&
    data.deciding_sets_won === null

  if (nothing) {
    return (
      <Card title="Under pressure, vs tour average">
        <EmptyState
          heading="Nothing to measure under pressure"
          reason={
            <>
              None of their matches carried a break point, a tiebreak or a deciding set that
              this database can read. {absenceReason(data.availability)}
            </>
          }
        />
      </Card>
    )
  }

  return (
    <Card title="Under pressure, vs tour average">
      <ClutchRow label="Break points saved" metric={data.break_points_saved} />
      <ClutchRow label="Tiebreaks won" metric={data.tiebreaks_won} />
      <ClutchRow label="Deciding sets won" metric={data.deciding_sets_won} />
      <Note>
      <p>
        Against every {(data.baseline.tiers ?? []).map(tierName).join(' and ')} match from the{' '}
        {decade(data.baseline.from_decade)} to the {decade(data.baseline.to_decade)}, weighted
        by where this player&apos;s own matches fell.{' '}
        {data.break_points_saved === null
          ? null
          : `Break points saved covers the ${data.break_points_saved.played} break points their matches recorded. `}
        Tiebreaks and deciding sets cover the {data.baseline.scored_matches} of{' '}
        {data.baseline.matches} matches with a readable, completed score.
      </p>
      <p>
        Every tiebreak is won by somebody, so those two averages sit at 50% by construction
        and the figure above is the margin over a coin toss. Break points saved is a real
        aggregate and is not 50%.{' '}
        <Link className={styles.inline} to="/methodology#where-statistics-do-not-exist">
          What is missing, and why
        </Link>
        .
      </p>
      </Note>
    </Card>
  )
}

function ClutchRow({ label, metric }: { label: string; metric: ClutchMetric | null }) {
  if (metric === null) {
    return (
      <StatRow label={label}>
        <AbsentCell label={label} />
      </StatRow>
    )
  }
  return (
    <StatRow label={label}>
      {formatPercent(metric.percentage, 0)}
      {metric.delta === null ? null : (
        <>
          {' '}
          <RankDelta
            delta={Math.round(metric.delta)}
            label="percentage points against the same levels and years"
          />
        </>
      )}
    </StatRow>
  )
}

function decade(year: number): string {
  return `${year}s`
}

function tierName(tier: string): string {
  return (tierLabel(tier) ?? tier).toLowerCase()
}

function RankingSection({
  rankings,
}: {
  rankings: ReturnType<typeof useResource<import('../api/client').RankingHistory>>
}) {
  if (rankings.state === 'loading') {
    return (
      <Card title="Official ranking">
        <Skeleton lines={2} />
      </Card>
    )
  }
  if (rankings.state === 'error') return null

  const history = rankings.data
  if (history.points.length === 0) {
    return (
      <Card title="Official ranking">
        <EmptyState
          heading="Never appeared in the published rankings"
          reason="The ATP list begins in 1973 and the WTA list in 1975, and a player has to reach a qualifying level to enter either. The Elo above is computed from matches and does not have that floor."
        />
      </Card>
    )
  }

  const latest = history.points[history.points.length - 1]
  return (
    <Card title="Official ranking">
      {history.best !== null ? (
        <StatRow label={`Best, ${history.best.date}`}>{history.best.rank}</StatRow>
      ) : null}
      {latest !== undefined ? (
        <StatRow label={`Last published, ${latest.date}`}>{latest.rank}</StatRow>
      ) : null}
    </Card>
  )
}

function ServeSection({ player }: { player: PlayerProfile }) {
  const serve = player.serve

  if (!hasStatistics(serve.availability) || serve.rates === null) {
    return (
      <Card title="Serve">
        <EmptyState
          heading="No serve statistics for this career"
          reason={absenceReason(serve.availability)}
          action={<ButtonLink to="/">See which tiers recorded them</ButtonLink>}
        />
      </Card>
    )
  }

  const rates = serve.rates
  const eligible = player.career === null ? 0 : player.career.matches - player.career.incomplete_matches

  return (
    <Card title="Serve" aside="What they served">
      <PartialAggregate
        recorded={Number(serve.matches_with_data)}
        total={Number(eligible)}
        noun="These figures"
        dashMeans="the match recorded no serve statistics"
      >
        <Rate label="Service games held" value={rates.service_games_held_percentage} lead />
        <StatRow label="Aces per match">{rates.aces_per_match.toFixed(1)}</StatRow>
        <StatRow label="Double faults per match">{rates.double_faults_per_match.toFixed(1)}</StatRow>
        <Rate label="First serves in" value={rates.first_serve_in_percentage} />
        <Rate label="First serve points won" value={rates.first_serve_won_percentage} />
        <Rate label="Second serve points won" value={rates.second_serve_won_percentage} />
        <Rate label="Break points saved" value={rates.break_points_saved_percentage} />
        <StatRow label="Service games played">{rates.service_games.toLocaleString()}</StatRow>
      </PartialAggregate>
    </Card>
  )
}

/**
 * The half of a career that lives on the other side of the net.
 *
 * Every figure here is a share of what the opponent served, which is why it is
 * its own card rather than more rows under Serve: the denominators are
 * different, the matches they were counted over are a different set, and a
 * break rate sitting under a hold rate would look like it came from the same
 * column of the same file. It does not.
 */
function ReturnSection({ player }: { player: PlayerProfile }) {
  const returns = player.return

  if (!hasStatistics(returns.availability) || returns.rates === null) {
    return (
      <Card title="Return">
        <EmptyState
          heading="No return statistics for this career"
          reason={absenceReason(returns.availability)}
        />
      </Card>
    )
  }

  const rates = returns.rates
  const eligible = player.career === null ? 0 : player.career.matches - player.career.incomplete_matches

  return (
    <Card title="Return" aside="What they were served">
      <PartialAggregate
        recorded={Number(returns.matches_with_data)}
        total={Number(eligible)}
        noun="These figures"
        dashMeans="the opponent's serve line was never recorded"
      >
        <Rate label="Return games won" value={rates.return_games_won_percentage} lead />
        <Rate label="Return points won" value={rates.return_points_won_percentage} />
        <Rate label="First-serve returns won" value={rates.first_return_won_percentage} />
        <Rate label="Second-serve returns won" value={rates.second_return_won_percentage} />
        <Rate label="Break points converted" value={rates.break_points_won_percentage} />
        <StatRow label="Break points earned">
          {rates.break_points_created.toLocaleString()}
        </StatRow>
        <StatRow label="Return games played">{rates.return_games.toLocaleString()}</StatRow>
      </PartialAggregate>
    </Card>
  )
}

/**
 * The two figures that need both serve lines at once, so they exist only for
 * the matches that carried both.
 *
 * The dominance ratio is the one number on this page that is not a percentage:
 * return points won as a share of theirs, over serve points lost as a share of
 * this player's. One is a player who returns exactly as well as they are
 * returned against, and the winner of a match is above it almost by
 * definition, which is what makes a career average above 1.10 remarkable.
 */
function PointsSection({ player }: { player: PlayerProfile }) {
  const points = player.points
  if (points === null) return null

  return (
    <Card className={styles.span} title="Points won" aside={`${points.matches.toLocaleString()} matches with both serve lines`}>
      <div className={styles.points}>
        <div className={styles.point}>
          <Kicker>Total points won</Kicker>
          <div className={styles.pointValue}>
            {points.total_points_won_percentage === null ? (
              <AbsentCell label="Total points won" />
            ) : (
              formatPercent(points.total_points_won_percentage)
            )}
          </div>
          <p className={styles.pointFoot}>
            Serve and return together. Half of every match is won by somebody, so a career above
            52% is a career that mostly won.
          </p>
        </div>
        <div className={styles.point}>
          <Kicker>Dominance ratio</Kicker>
          <div className={styles.pointValue}>
            {points.dominance_ratio === null ? (
              <AbsentCell label="Dominance ratio" />
            ) : (
              points.dominance_ratio.toFixed(2)
            )}
          </div>
          <p className={styles.pointFoot}>
            Return points won over serve points lost. 1.00 is a player who returns as well as
            they are returned against.
          </p>
        </div>
      </div>
    </Card>
  )
}

/** A percentage row, or the absence in its place. Never a zero for a missing one. */
function Rate({
  label,
  value,
  lead = false,
}: {
  label: string
  value: number | null
  lead?: boolean
}) {
  return (
    <StatRow label={label}>
      <span className={lead ? styles.lead : undefined}>
        {value === null ? <AbsentCell label={label} /> : formatPercent(value)}
      </span>
    </StatRow>
  )
}

function SplitsSection({ career }: { career: Career }) {
  return (
    <Card title="By surface and by level">
      <StatTable
        caption="Every match counts once, on the surface and at the tier it was played."
        columns={[
          {
            key: 'surface',
            header: 'Surface',
            value: (row) => row.surface,
            render: (row) => <SurfaceDot surface={row.surface === 'unknown' ? null : row.surface} />,
          },
          { key: 'matches', header: 'Matches', align: 'right', value: (row) => Number(row.matches) },
          { key: 'wins', header: 'Won', align: 'right', value: (row) => Number(row.wins) },
          { key: 'losses', header: 'Lost', align: 'right', value: (row) => Number(row.losses) },
        ]}
        rows={career.surfaces}
        rowKey={(row) => row.surface}
        defaultSort={{ key: 'matches', direction: 'desc' }}
      />
      <StatTable
        caption="Matches with serve statistics, per tier. This is what makes never recorded at this level a checkable claim."
        columns={[
          { key: 'tier', header: 'Tier', wrap: true, value: (row) => row.tier },
          { key: 'matches', header: 'Matches', align: 'right', value: (row) => Number(row.matches) },
          {
            key: 'stats',
            header: 'Serve stats',
            align: 'right',
            // A tier that never recorded them has no figure, not a zero.
            value: (row) => (Number(row.matches_with_stats) === 0 ? null : Number(row.matches_with_stats)),
          },
        ]}
        rows={career.tiers}
        rowKey={(row) => row.tier}
        defaultSort={{ key: 'matches', direction: 'desc' }}
      />
      <StatRow label="Retirements and walkovers">{career.incomplete_matches}</StatRow>
      <Note>
        They count in the record above and are excluded from every rate on this page.
      </Note>
    </Card>
  )
}

/**
 * The history's columns. The score cell carries the charted mark on the rows
 * that have a sheet behind them, so the columns depend on which one is open.
 */
function matchColumns(
  open: string | null,
  onToggle: (id: string) => void,
): ReadonlyArray<Column<PlayerMatch>> {
  return [
  { key: 'date', header: 'Date', value: (row) => row.date },
  {
    key: 'result',
    header: 'Result',
    value: (row) => (row.won ? 1 : 0),
    render: (row) => <WinLossMark won={row.won} />,
  },
  {
    key: 'opponent',
    wrap: true,
    header: 'Opponent',
    value: (row) => row.opponent.name,
    render: (row) => (
      <>
        <Link to={`/players/${row.opponent.slug}`} className={styles.opponent}>
          {row.opponent.name}
        </Link>
        <div className={styles.event}>
          {/* The surface column steps aside on a phone; its square moves here. */}
          <span className={styles.eventSurface}>
            <SurfaceDot surface={row.surface} label={false} />{' '}
          </span>
          <EventLink name={row.tournament} slug={row.event_slug} season={row.season} /> {row.round}
          {row.qualifying ? ' Q' : ''}
        </div>
      </>
    ),
  },
  {
    key: 'surface',
    header: 'Surface',
    value: (row) => row.surface,
    render: (row) => <SurfaceDot surface={row.surface} />,
    wide: true,
  },
  {
    key: 'score',
    header: 'Score',
    // A long score may take two lines, but a set never breaks: there are no
    // spaces inside one.
    wrap: true,
    minWidth: '10ch',
    value: (row) => row.score,
    render: (row) => (
      <>
        <Score score={row.score} incomplete={row.incomplete} />
        {row.charting_id !== null ? (
          <ChartedMark
            open={row.charting_id === open}
            onToggle={() => onToggle(row.charting_id as string)}
          />
        ) : null}
      </>
    ),
    sortable: false,
  },
  {
    key: 'aces',
    header: 'Aces',
    align: 'right',
    value: (row) => row.serve.aces,
    // The column a phone has no room for; the score is the one it needs.
    wide: true,
  },
  ]
}

interface MatchesSectionProps {
  matches: ReturnType<typeof useResource<import('../api/client').Page<PlayerMatch>>>
  /** The page's player, who takes the A side of a charted sheet. */
  slug: string
  surface: string | null
  onSurface: (surface: string | null) => void
  onMore: (cursor: string) => void
  paged: boolean
  onFirst: () => void
}

function MatchesSection({
  matches,
  slug,
  surface,
  onSurface,
  onMore,
  paged,
  onFirst,
}: MatchesSectionProps) {
  const [openChart, setOpenChart] = useState<string | null>(null)
  const columns = useMemo(
    () => matchColumns(openChart, (id) => setOpenChart((current) => (current === id ? null : id))),
    [openChart],
  )
  return (
    <Card title="Every match" aside="Most recent first">
      <SurfaceToggle value={surface} onChange={onSurface} />

      {matches.state === 'loading' ? <Skeleton lines={8} /> : null}

      {matches.state === 'error' ? (
        <p className={styles.error}>
          The match list could not be loaded: {matches.error.message} Reload, or clear the
          surface filter.
        </p>
      ) : null}

      {matches.state === 'ready' ? (
        matches.data.data.length === 0 ? (
          <EmptyState
            heading="No matches on this surface"
            reason="Every match of theirs in the database was played on another surface, or the source never recorded which one."
            action={<Button onClick={() => onSurface(null)}>Show every surface</Button>}
          />
        ) : (
          <>
            <StatTable
              caption="Most recent first. Aces reading n/r is a match nobody recorded serve statistics for; ret. is a retirement and w/o a walkover, which count in the record and sit out of every rate."
              columns={columns}
              rows={matches.data.data}
              rowKey={(row) => `${row.date}-${row.tournament}-${row.opponent.slug}-${row.round}`}
              detail={(row) =>
                row.charting_id !== null && row.charting_id === openChart ? (
                  <ChartedSheet chartingId={row.charting_id} first={slug} />
                ) : null
              }
            />
            <div className={styles.more}>
              {matches.data.next_cursor !== null && matches.data.next_cursor !== '' ? (
                <Button onClick={() => onMore(matches.data.next_cursor as string)}>
                  Show earlier matches
                </Button>
              ) : null}
              {paged ? <Button onClick={onFirst}>Back to the most recent</Button> : null}
            </div>
          </>
        )
      ) : null}
    </Card>
  )
}
