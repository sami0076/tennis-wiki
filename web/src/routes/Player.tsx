import { useMemo, useState } from 'react'
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
  Skeleton,
  StatRow,
  StatTable,
  SurfaceDot,
  SurfaceEloStrip,
  SurfaceToggle,
  WinLossMark,
  type Column,
} from '../components'
import { absenceReason, hasStatistics } from '../lib/absence'
import { ageOn, careerSpan, formatElo, formatHand, formatPercent, surname } from '../lib/format'
import { tierLabel } from '../lib/tier'
import { breadcrumbs, person, useJsonLd } from '../lib/jsonld'
import { useUrlParam } from '../lib/useUrlParam'
import { OpponentsSection, SeasonsSection } from './PlayerSplits'
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

  return (
    <>
      <PlayerHero player={player} active={active} rankings={rankings} form={form} />

      {player.career === null ? null : <CareerTiles career={player.career} />}

      <div className={styles.charts}>
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
        <div className={styles.columns}>
          <div className={styles.left}>
            <ClutchSection clutch={clutch} />
            <CareerSection career={player.career} />
            {player.splits === null ? null : <OpponentsSection splits={player.splits} />}
            <RankingSection rankings={rankings} />
          </div>
          <div className={styles.right}>
            <ServeSection player={player} />
            <SplitsSection career={player.career} />
          </div>
        </div>
      )}

      {player.career === null ? null : <SeasonsSection seasons={seasons} />}

      {player.career === null ? null : (
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
      )}
    </>
  )
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
        <Card tint="a" className={styles.eloCard} delay={100}>
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
    { label: 'Majors', value: String(career.majors), count: career.majors, accent: true },
  ]
  return (
    <div className={styles.tiles}>
      {tiles.map((tile, index) => (
        <Card key={tile.label} tilt className={styles.tile} delay={index * 80}>
          <Kicker>{tile.label}</Kicker>
          <div className={tile.accent ? `${styles.tileValue} ${styles.tileAccent}` : styles.tileValue}>
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
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Under pressure, vs tour average</h2>
        <Skeleton lines={3} />
      </section>
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
      <section className={styles.section}>
        <EmptyState
          heading="Nothing to measure under pressure"
          reason={
            <>
              None of their matches carried a break point, a tiebreak or a deciding set that
              this database can read. {absenceReason(data.availability)}
            </>
          }
        />
      </section>
    )
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Under pressure, vs tour average</h2>
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
    </section>
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

function CareerSection({ career }: { career: Career }) {
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Career</h2>
      <StatRow label="Matches">{career.matches.toLocaleString()}</StatRow>
      <StatRow label="Retirements and walkovers">{career.incomplete_matches}</StatRow>
      <Note>Retirements and walkovers count in the record and are excluded from every rate.</Note>
    </section>
  )
}

function RankingSection({
  rankings,
}: {
  rankings: ReturnType<typeof useResource<import('../api/client').RankingHistory>>
}) {
  if (rankings.state === 'loading') {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Official ranking</h2>
        <Skeleton lines={2} />
      </section>
    )
  }
  if (rankings.state === 'error') return null

  const history = rankings.data
  if (history.points.length === 0) {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Official ranking</h2>
        <EmptyState
          heading="Never appeared in the published rankings"
          reason="The ATP list begins in 1973 and the WTA list in 1975, and a player has to reach a qualifying level to enter either. The Elo above is computed from matches and does not have that floor."
        />
      </section>
    )
  }

  const latest = history.points[history.points.length - 1]
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Official ranking</h2>
      {history.best !== null ? (
        <StatRow label={`Best, ${history.best.date}`}>{history.best.rank}</StatRow>
      ) : null}
      {latest !== undefined ? (
        <StatRow label={`Last published, ${latest.date}`}>{latest.rank}</StatRow>
      ) : null}
    </section>
  )
}

function ServeSection({ player }: { player: PlayerProfile }) {
  const serve = player.serve

  if (!hasStatistics(serve.availability) || serve.rates === null) {
    return (
      <section className={styles.section}>
        <EmptyState
          heading="No serve statistics for this career"
          reason={absenceReason(serve.availability)}
          action={<ButtonLink to="/">See which tiers recorded them</ButtonLink>}
        />
      </section>
    )
  }

  const rates = serve.rates
  const eligible = player.career === null ? 0 : player.career.matches - player.career.incomplete_matches

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Serve</h2>
      <PartialAggregate
        recorded={Number(serve.matches_with_data)}
        total={Number(eligible)}
        noun="These figures"
        dashMeans="the match recorded no serve statistics"
      >
        <StatRow label="Aces per match">{rates.aces_per_match.toFixed(1)}</StatRow>
        <StatRow label="Double faults per match">{rates.double_faults_per_match.toFixed(1)}</StatRow>
        <StatRow label="First serves in">
          {rates.first_serve_in_percentage === null ? (
            <AbsentCell label="First serves in" />
          ) : (
            formatPercent(rates.first_serve_in_percentage)
          )}
        </StatRow>
        <StatRow label="First serve points won">
          {rates.first_serve_won_percentage === null ? (
            <AbsentCell label="First serve points won" />
          ) : (
            formatPercent(rates.first_serve_won_percentage)
          )}
        </StatRow>
        <StatRow label="Second serve points won">
          {rates.second_serve_won_percentage === null ? (
            <AbsentCell label="Second serve points won" />
          ) : (
            formatPercent(rates.second_serve_won_percentage)
          )}
        </StatRow>
        <StatRow label="Break points saved">
          {rates.break_points_saved_percentage === null ? (
            <AbsentCell label="Break points saved" />
          ) : (
            formatPercent(rates.break_points_saved_percentage)
          )}
        </StatRow>
      </PartialAggregate>
    </section>
  )
}

function SplitsSection({ career }: { career: Career }) {
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>By surface and by level</h2>
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
    </section>
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
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Matches</h2>
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
    </section>
  )
}
