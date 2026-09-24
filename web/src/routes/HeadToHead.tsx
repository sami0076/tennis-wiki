import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  ApiError,
  type HeadToHead as Comparison,
  type HeadToHeadCloseness,
  type HeadToHeadFilters,
  type HeadToHeadPlayer,
  type HeadToHeadRecord,
  type Meeting,
  type Page,
  type Pair,
  type PlayerMatch,
  type PlayerProfile,
  type PlayerSearchResult,
  type RatingSeries,
  type ServeRates,
} from '../api/client'
import {
  getHeadToHead,
  getPlayer,
  getPlayerMatches,
  getPlayerRatingSeries,
  simulateMatch,
  type MeetingFilters,
} from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  Button,
  Card,
  CountUp,
  Odometer,
  FormPills,
  Kicker,
  Note,
  Reveal,
  SurfaceBadge,
  ButtonLink,
  ChartedMark,
  ChartedSheet,
  EmptyState,
  EventLink,
  Meta,
  PlayerSearch,
  RivalryStrip,
  Score,
  Skeleton,
  SplitBar,
  StatTable,
  SurfaceDot,
  SurfaceToggle,
  TrajectoryPair,
  type Column,
  type RivalryResult,
} from '../components'
import { absenceReason } from '../lib/absence'
import { formatHand, formatPercent, formatScore, surname } from '../lib/format'
import { breadcrumbs, useJsonLd } from '../lib/jsonld'
import { surfaceLabel } from '../lib/surface'
import { tierLabel } from '../lib/tier'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './HeadToHead.module.css'

/**
 * The head-to-head page: two players, one comparison, one URL.
 *
 * The URL is the state. /h2h/a/b and /h2h/b/a are the same rivalry read from
 * opposite ends -- the API guarantees that and this page never recomputes it --
 * so a comparison can be sent to somebody and arrive as the same page.
 */
export function HeadToHead() {
  const { a, b } = useParams()

  // Fetched here rather than in Rivalry so the pickers can show who is being
  // compared. Resolving null when a slug is missing keeps this a hook that runs
  // every render, which is what the rules of hooks require and what a
  // conditional fetch would break.
  const filters = useMeetingFilters()
  const h2h = useResource<Comparison | null>(
    (signal) =>
      a === undefined || b === undefined
        ? Promise.resolve(null)
        : getHeadToHead(a, b, filters.values, signal),
    [a, b, filters.key],
  )
  const players = h2h.state === 'ready' && h2h.data !== null ? h2h.data.players : undefined
  useJsonLd(
    'breadcrumbs',
    breadcrumbs(
      players === undefined
        ? [{ name: 'Head to head', path: '/h2h' }]
        : [
            { name: 'Head to head', path: '/h2h' },
            { name: `${players[0].name} v ${players[1].name}`, path: `/h2h/${a}/${b}` },
          ],
    ),
  )

  return (
    <>
      <h1 className="sr-only">Head to head</h1>
      <Pickers a={a} b={b} players={players} />
      {a !== undefined && b !== undefined ? (
        <Rivalry a={a} b={b} h2h={h2h} filters={filters} />
      ) : (
        <EmptyState
          heading="Pick two players"
          reason="Any two, from either tour and any level. Most pairs of 115,000 players have never met, and that is a real answer this page will give you."
          action={<ButtonLink to="/players">Browse players</ButtonLink>}
        />
      )}
    </>
  )
}

type Side = 'a' | 'b'

/**
 * Two comboboxes, one per side.
 *
 * A comparison needs both players, and the URL cannot hold half of one, so the
 * first pick is held here until the second arrives. Throwing it away was the
 * original bug: choosing on an empty page cleared the box and remembered
 * nothing, so no pair could ever be built from /h2h.
 *
 * What each box shows is the player actually being compared, taken from the
 * loaded comparison, and the pending pick only while there is no comparison
 * yet. Typing takes over from either.
 */
function Pickers({
  a,
  b,
  players,
}: {
  a?: string
  b?: string
  players?: Pair<HeadToHeadPlayer>
}) {
  const navigate = useNavigate()
  const [picked, setPicked] = useState<Partial<Record<Side, PlayerSearchResult>>>({})
  const [typed, setTyped] = useState<Partial<Record<Side, string>>>({})
  const [clash, setClash] = useState(false)

  // The URL is the comparison; a pending pick is only a comparison waiting for
  // its other half.
  function slugFor(side: Side): string | undefined {
    return (side === 'a' ? a : b) ?? picked[side]?.slug
  }

  function nameFor(side: Side): string {
    if (players !== undefined) return side === 'a' ? players[0].name : players[1].name
    return picked[side]?.name ?? ''
  }

  function choose(side: Side, player: PlayerSearchResult) {
    const other: Side = side === 'a' ? 'b' : 'a'
    if (player.slug === slugFor(other)) {
      // The API answers 400 for this, correctly. Saying so here is faster and
      // does not cost a request.
      setClash(true)
      return
    }
    setClash(false)
    setPicked((current) => ({ ...current, [side]: player }))
    // Drop what was typed so the box shows the name that was chosen.
    setTyped((current) => ({ ...current, [side]: undefined }))

    const otherSlug = slugFor(other)
    if (otherSlug === undefined) return
    navigate(
      side === 'a' ? `/h2h/${player.slug}/${otherSlug}` : `/h2h/${otherSlug}/${player.slug}`,
    )
  }

  return (
    <div className={styles.pickers}>
      {(['a', 'b'] as const).map((side) => (
        <PlayerSearch
          key={side}
          label={side === 'a' ? 'First player' : 'Second player'}
          placeholder="Search by name"
          value={typed[side] ?? nameFor(side)}
          onChange={(value) => setTyped((current) => ({ ...current, [side]: value }))}
          onSelect={(player) => choose(side, player)}
        />
      ))}
      {clash ? (
        <p className={styles.error}>
          Pick two different players. A comparison with themselves would be 0-0 for both
          sides.
        </p>
      ) : null}
    </div>
  )
}

function Rivalry({
  a,
  b,
  h2h,
  filters,
}: {
  a: string
  b: string
  h2h: Resource<Comparison | null>
  filters: ReturnType<typeof useMeetingFilters>
}) {
  // The rating lines follow the surface filter, the one cut a rating has.
  const surface = filters.values.surface ?? null
  const profileA = useResource((signal) => getPlayer(a, signal), [a])
  const profileB = useResource((signal) => getPlayer(b, signal), [b])
  const series = surface ?? 'overall'
  const ratingsA = useResource(
    (signal) => getPlayerRatingSeries(a, { surface: series }, signal),
    [a, series],
  )
  const ratingsB = useResource(
    (signal) => getPlayerRatingSeries(b, { surface: series }, signal),
    [b, series],
  )
  const formA = useResource((signal) => getPlayerMatches(a, { limit: 10 }, signal), [a])
  const formB = useResource((signal) => getPlayerMatches(b, { limit: 10 }, signal), [b])

  if (h2h.state === 'error') {
    const notFound = h2h.error instanceof ApiError && h2h.error.status === 404
    return (
      <EmptyState
        heading={notFound ? 'One of those players does not exist' : 'That comparison failed'}
        reason={
          notFound
            ? `Nothing in the database answers to "${a}" or "${b}". Names are spelled as the tour's own records spell them.`
            : h2h.error.message
        }
        action={<ButtonLink to="/players">Search for a player</ButtonLink>}
      />
    )
  }

  // Null is the resource standing in for "no comparison asked for", which this
  // component is only rendered with a moment before the slugs arrive.
  if (h2h.state !== 'ready' || h2h.data === null) return <Skeleton lines={10} />

  const comparison = h2h.data
  const [playerA, playerB] = comparison.players
  // The API has already cut the meetings, the record and the serve figures
  // to the filters in the URL; the closeness summary it never cuts.
  const met = comparison.meetings
  const record = comparison.record
  const filtered = isFiltered(comparison.filters)

  const [winsA, winsB] = record.wins
  const leader =
    winsA === winsB ? 'Level' : `${surname(winsA > winsB ? playerA.name : playerB.name)} leads`

  return (
    <>
      <div className={styles.score}>
        <PlayerPanel side="a" player={playerA} profile={profileA} form={formA} surface={surface} />
        <div className={styles.tally}>
          <Kicker>Head-to-head</Kicker>
          <div className={styles.tallyFigures} aria-hidden="true">
            <Odometer className={styles.tallyA} value={winsA} />
            <span className={styles.tallyDash} />
            <Odometer className={styles.tallyB} value={winsB} />
          </div>
          <span className="sr-only">{`${winsA}-${winsB}`}</span>
          {record.matches > 0 ? <span className={styles.leads}>{leader}</span> : null}
          <span className={styles.tallyWords}>
            {filtered
              ? `${describeFilters(comparison.filters)}, of ${comparison.total_meetings} ${comparison.total_meetings === 1 ? 'meeting' : 'meetings'}`
              : `over ${record.matches} ${record.matches === 1 ? 'meeting' : 'meetings'}`}
          </span>
        </div>
        <PlayerPanel side="b" player={playerB} profile={profileB} form={formB} surface={surface} />
        <div className={styles.scoreBar} aria-hidden="true">
          <span style={{ flexGrow: winsA + winsB === 0 ? 1 : winsA }} />
          <span style={{ flexGrow: winsA + winsB === 0 ? 1 : winsB }} />
        </div>
      </div>

      {comparison.total_meetings > 0 ? (
        <MeetingFilterControls comparison={comparison} filters={filters} />
      ) : null}

      <div className={styles.grid}>
        <SimulatorCard a={playerA} b={playerB} surface={surface} />
        {record.matches > 0 ? <SurfaceCard comparison={comparison} /> : null}
      </div>

      {record.matches > 0 ? <MeetingTiles comparison={comparison} /> : null}
      {record.matches > 0 ? <RecentMeetings comparison={comparison} /> : null}

      {record.matches === 0 ? (
        <NeverMet
          playerA={playerA}
          playerB={playerB}
          filters={comparison.filters}
          everMet={comparison.total_meetings > 0}
          onClear={filters.clear}
        />
      ) : (
        <>
          <RivalrySection
            meetings={met}
            playerA={playerA}
            playerB={playerB}
            record={record}
            closeness={comparison.closeness}
            total={comparison.total_meetings}
          />
          <ServeSection comparison={comparison} playerA={playerA} playerB={playerB} filtered={filtered} />
        </>
      )}

      <RatingsSection
        profileA={profileA}
        profileB={profileB}
        ratingsA={ratingsA}
        ratingsB={ratingsB}
        playerA={playerA}
        playerB={playerB}
        surface={surface}
      />

      {record.matches === 0 ? null : (
        <MeetingsSection meetings={met} playerA={playerA} playerB={playerB} />
      )}

      <div className={styles.simulate}>
        <ButtonLink to={`/simulator?a=${playerA.slug}&b=${playerB.slug}`}>
          Simulate this matchup
        </ButtonLink>
      </div>
    </>
  )
}

function PlayerPanel({
  side,
  player,
  profile,
  form,
  surface,
}: {
  side: 'a' | 'b'
  player: HeadToHeadPlayer
  profile: Resource<PlayerProfile>
  form: Resource<Page<PlayerMatch>>
  surface: string | null
}) {
  const elo = ratingOf(profile, surface ?? 'overall')
  const recent = form.state === 'ready' ? form.data.data.slice(0, 10) : []
  const hand = profile.state === 'ready' ? formatHand(profile.data.hand) : null
  return (
    <Card className={side === 'a' ? styles.panel : `${styles.panel} ${styles.panelB}`}>
      <span className={side === 'a' ? styles.dashA : styles.dashB} aria-hidden="true" />
      <Link className={side === 'a' ? styles.nameA : styles.nameB} to={`/players/${player.slug}`}>
        {player.name}
      </Link>
      <Meta
        className={side === 'b' ? styles.metaB : undefined}
        parts={[player.tour.toUpperCase(), player.country, hand]}
      />
      <div className={styles.panelStats}>
        {elo === null ? null : (
          <div>
            <Kicker>{surface === null ? 'Elo' : `${surfaceLabel(surface)} Elo`}</Kicker>
            <Odometer className={styles.panelElo} value={Math.round(elo)} />
          </div>
        )}
        {recent.length > 0 ? (
          <div>
            <Kicker>Form</Kicker>
            <FormPills results={recent.map((m) => m.won).reverse()} side={side} label={`${player.name}, recent form`} />
          </div>
        ) : null}
      </div>
    </Card>
  )
}

/** What the model makes of the next meeting between them. */
function SimulatorCard({
  a,
  b,
  surface,
}: {
  a: HeadToHeadPlayer
  b: HeadToHeadPlayer
  surface: string | null
}) {
  const [bestOf, setBestOf] = useState(3)
  const sim = useResource(
    (signal) => simulateMatch(a.slug, b.slug, { surface, best_of: bestOf }, signal),
    [a.slug, b.slug, surface, bestOf],
  )
  const chain = sim.state === 'ready' ? sim.data.chain : null
  const pa = chain === null ? null : chain.match[0]

  return (
    <Card
      title="Match simulator"
      aside={`${surface === null ? 'All surfaces' : surfaceLabel(surface)} · Bo${bestOf}`}
    >
      {sim.state === 'loading' ? <Skeleton lines={4} /> : null}
      {sim.state === 'ready' && pa === null ? (
        <p className={styles.caption}>{absenceReason(sim.data.availability)}</p>
      ) : null}
      {pa !== null ? (
        <>
          <div className={styles.odds}>
            <div>
              <span className={styles.oddsA}>
                <CountUp value={pa * 100} format={(v) => v.toFixed(1)} />
                <small>%</small>
              </span>
              <div className={styles.oddsName}>{surname(a.name)}</div>
            </div>
            <div className={styles.oddsRight}>
              <span className={styles.oddsB}>
                <CountUp value={(1 - pa) * 100} format={(v) => v.toFixed(1)} />
                <small>%</small>
              </span>
              <div className={styles.oddsName}>{surname(b.name)}</div>
            </div>
          </div>
          <div className={styles.oddsBar} aria-hidden="true">
            <span style={{ width: `${pa * 100}%` }} />
          </div>
        </>
      ) : null}
      <div className={styles.simActions}>
        <ButtonLink to={`/simulator?a=${a.slug}&b=${b.slug}`}>Open full simulator</ButtonLink>
        <div className={styles.cells} role="group" aria-label="Match length">
          {[3, 5].map((n) => (
            <button
              key={n}
              type="button"
              className={bestOf === n ? `${styles.cell} ${styles.cellActive}` : styles.cell}
              aria-pressed={bestOf === n}
              onClick={() => setBestOf(n)}
            >
              Bo{n}
            </button>
          ))}
        </div>
      </div>
    </Card>
  )
}

/** The record on each surface they met on, as one split bar per surface. */
function SurfaceCard({ comparison }: { comparison: Comparison }) {
  const rows = comparison.surfaces.filter((split) => split.matches > 0)
  return (
    <Card title="By surface" delay={100}>
      <div className={styles.surfaces}>
        {rows.map((split) => {
          const [wa, wb] = split.wins
          return (
            <div key={split.name} className={styles.surfaceRow}>
              <SurfaceBadge surface={split.name === 'unknown' ? null : split.name} />
              <div className={styles.surfaceBar} aria-hidden="true">
                {wa > 0 ? <span className={styles.barA} style={{ flexGrow: wa }} /> : null}
                {wb > 0 ? <span className={styles.barB} style={{ flexGrow: wb }} /> : null}
              </div>
              <span className={styles.pair} aria-label={`${wa} to ${wb}`}>
                <span className={styles.pairA}>{wa}</span>
                <span className={styles.pairDash} aria-hidden="true">–</span>
                <span className={styles.pairB}>{wb}</span>
              </span>
            </div>
          )
        })}
      </div>
    </Card>
  )
}

function MeetingTiles({ comparison }: { comparison: Comparison }) {
  const tally = (list: ReadonlyArray<Meeting>): [number, number] => [
    list.filter((m) => m.winner_index === 0).length,
    list.filter((m) => m.winner_index === 1).length,
  ]
  const finals = comparison.meetings.filter((m) => m.round === 'F')
  const tiles: { label: string; wins: [number, number] | null }[] = [
    { label: 'Finals', wins: finals.length > 0 ? tally(finals) : null },
    { label: 'Slam finals', wins: tally(finals.filter((m) => m.level === 'G')) },
    {
      label: 'Deciding sets',
      wins:
        comparison.closeness.deciders.matches > 0
          ? [comparison.closeness.deciders.wins[0], comparison.closeness.deciders.wins[1]]
          : null,
    },
    {
      label: 'Tiebreaks',
      wins:
        comparison.closeness.tiebreaks.matches > 0
          ? [comparison.closeness.tiebreaks.wins[0], comparison.closeness.tiebreaks.wins[1]]
          : null,
    },
  ]
  return (
    <div className={styles.tiles}>
      {tiles.map((tile, index) => (
        <Card key={tile.label} tilt className={styles.tile} delay={index * 70}>
          <Kicker>{tile.label}</Kicker>
          {tile.wins === null || tile.wins[0] + tile.wins[1] === 0 ? (
            <span className={styles.tileNone}>none</span>
          ) : (
            <span className={styles.tileValue} aria-label={`${tile.wins[0]} to ${tile.wins[1]}`}>
              <span className={styles.pairA}>{tile.wins[0]}</span>
              <span className={styles.pairDash} aria-hidden="true">–</span>
              <span className={styles.pairB}>{tile.wins[1]}</span>
            </span>
          )}
        </Card>
      ))}
    </div>
  )
}

/** The latest meetings as cards, newest first; the full sheet is further down. */
function RecentMeetings({ comparison }: { comparison: Comparison }) {
  const recent = [...comparison.meetings].reverse().slice(0, 6)
  const [playerA, playerB] = comparison.players
  return (
    <section className={styles.recent}>
      <div className={styles.recentHead}>
        <h2 className={styles.sectionTitle}>Recent meetings</h2>
        <a className={styles.recentLink} href="#every-meeting">
          All {comparison.meetings.length} ↓
        </a>
      </div>
      <div className={styles.recentGrid}>
        {recent.map((meeting, index) => {
          const winner = meeting.winner_index === 0 ? playerA : playerB
          return (
            <Reveal
              key={`${meeting.date}-${meeting.tournament}-${meeting.round}`}
              className={styles.meeting}
              delay={index * 60}
            >
              <div className={styles.meetingEvent}>
                <span className={styles.meetingName}>{meeting.tournament}</span>
                <span className={styles.meetingWhen}>
                  {meeting.round} · {meeting.date.slice(0, 7)}
                </span>
              </div>
              <SurfaceBadge surface={meeting.surface} />
              <div className={styles.meetingResult}>
                <span className={meeting.winner_index === 0 ? styles.winnerA : styles.winnerB}>
                  {surname(winner.name)}
                </span>
                <span className={styles.meetingScore}>{formatScore(meeting.score) ?? 'n/r'}</span>
              </div>
            </Reveal>
          )
        })}
      </div>
    </section>
  )
}

/** A pair who never met, which is the normal case and reads as one. */
function NeverMet({
  playerA,
  playerB,
  filters,
  everMet,
  onClear,
}: {
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  filters: HeadToHeadFilters
  everMet: boolean
  onClear: () => void
}) {
  if (everMet) {
    return (
      <EmptyState
        heading={`They never met ${describeFilters(filters)}`}
        reason="Every meeting between them falls outside that cut. The record above is 0-0 under it, not 0-0 between them."
        action={<Button onClick={onClear}>Show every meeting</Button>}
      />
    )
  }

  const differentTours = playerA.tour !== playerB.tour
  return (
    <EmptyState
      heading="These two have never met"
      reason={
        differentTours
          ? `${playerA.name} plays the ${playerA.tour.toUpperCase()} and ${playerB.name} the ${playerB.tour.toUpperCase()}, so no draw could ever have paired them.`
          : 'No match between them is in the database. Most pairs of 115,000 players have never played each other, and careers that miss by a decade or a tier never could have.'
      }
      action={<ButtonLink to={`/players/${playerA.slug}`}>See {playerA.name}</ButtonLink>}
    />
  )
}

function RivalrySection({
  meetings,
  playerA,
  playerB,
  record,
  closeness,
  total,
}: {
  meetings: ReadonlyArray<Meeting>
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  record: HeadToHeadRecord
  closeness: HeadToHeadCloseness
  total: number
}) {
  // Oldest first, which is how a rivalry reads. The API returns them that way
  // and the surface filter above preserves the order.
  const results: RivalryResult[] = meetings.map((meeting) => ({
    wonByA: meeting.winner_index === 0,
    surface: meeting.surface,
    description: `${meeting.tournament} ${meeting.round}, ${meeting.date}`,
  }))

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>The rivalry, oldest to newest</h2>
      <RivalryStrip results={results} nameA={playerA.name} nameB={playerB.name} legend={false} />
      {closeness.deciders.matches > 0 ? (
        <SplitBar
          label="Deciding sets won"
          left={closeness.deciders.wins[0]}
          right={closeness.deciders.wins[1]}
          max={closeness.deciders.matches}
        />
      ) : null}
      {closeness.tiebreaks.matches > 0 ? (
        <SplitBar
          label="Tiebreaks won"
          left={closeness.tiebreaks.wins[0]}
          right={closeness.tiebreaks.wins[1]}
          max={closeness.tiebreaks.matches}
        />
      ) : null}
      <Note>
        <p>
          Filled squares are wins for {playerA.name}, outlined are wins for {playerB.name}. Colour
          is the surface.
        </p>
        {record.incomplete > 0 ? (
          <p>
            {record.incomplete} of these {record.matches} ended in a retirement or a walkover.
            They count in the record and are left out of every rate.
          </p>
        ) : null}
        <p>
        {closeness.deciders.matches} of the {closeness.scored} meetings whose score could be read went
        to a deciding set, and {closeness.tiebreaks.matches}{' '}
        {closeness.tiebreaks.matches === 1 ? 'tiebreak was' : 'tiebreaks were'} played between them.
        Both are over all {total} meetings whatever the cut above; the record and the strip are
        the cut.
        </p>
      </Note>
    </section>
  )
}

/** One comparable number per player, or the fact that there is not one. */
interface Comparable {
  label: string
  a: number | null
  b: number | null
  max?: number
  format: (value: number) => string
}

function ServeSection({
  comparison,
  playerA,
  playerB,
  filtered,
}: {
  comparison: Comparison
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  filtered: boolean
}) {
  const [serveA, serveB] = comparison.serve

  if (serveA.rates === null && serveB.rates === null) {
    return (
      <section className={styles.section}>
        <EmptyState
          heading="No serve statistics for these meetings"
          reason={absenceReason(serveA.availability)}
          action={
            <ButtonLink to={`/players/${playerA.slug}`}>See their careers instead</ButtonLink>
          }
        />
      </section>
    )
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>In these meetings</h2>
      {compare(serveA.rates, serveB.rates).map((row) => (
        <CompareRow key={row.label} row={row} nameA={playerA.name} nameB={playerB.name} />
      ))}
      <Note>
        Over the {Number(serveA.matches_with_data)} of {filtered ? 'these' : 'their'} meetings
        that recorded a serve line{filtered ? ', under the cut above' : ''}.
      </Note>
    </section>
  )
}

function compare(a: ServeRates | null, b: ServeRates | null): Comparable[] {
  const percent = (value: number) => formatPercent(value, 0)
  return [
    {
      label: 'First serve points won',
      a: a?.first_serve_won_percentage ?? null,
      b: b?.first_serve_won_percentage ?? null,
      max: 100,
      format: percent,
    },
    {
      label: 'Second serve points won',
      a: a?.second_serve_won_percentage ?? null,
      b: b?.second_serve_won_percentage ?? null,
      max: 100,
      format: percent,
    },
    {
      label: 'Break points saved',
      a: a?.break_points_saved_percentage ?? null,
      b: b?.break_points_saved_percentage ?? null,
      max: 100,
      format: percent,
    },
    {
      label: 'Aces per match',
      a: a?.aces_per_match ?? null,
      b: b?.aces_per_match ?? null,
      format: (value) => value.toFixed(1),
    },
  ]
}

/**
 * A SplitBar needs both halves. Where one player has the number and the other
 * does not, a bar against nothing would read as a win by default, so the row
 * names the side that is missing instead.
 */
function CompareRow({ row, nameA, nameB }: { row: Comparable; nameA: string; nameB: string }) {
  if (row.a !== null && row.b !== null) {
    return (
      <SplitBar label={row.label} left={row.a} right={row.b} max={row.max} format={row.format} />
    )
  }
  if (row.a === null && row.b === null) return null

  const present = row.a !== null ? nameA : nameB
  const value = row.a ?? row.b ?? 0
  return (
    <p className={styles.absent}>
      <span>{row.label}</span>
      <span>
        {row.format(value)} for {present}, not recorded for the other
      </span>
    </p>
  )
}

function RatingsSection({
  profileA,
  profileB,
  ratingsA,
  ratingsB,
  playerA,
  playerB,
  surface,
}: {
  profileA: Resource<PlayerProfile>
  profileB: Resource<PlayerProfile>
  ratingsA: Resource<RatingSeries>
  ratingsB: Resource<RatingSeries>
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  surface: string | null
}) {
  const series = surface ?? 'overall'
  const eloA = ratingOf(profileA, series)
  const eloB = ratingOf(profileB, series)

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>
        {surface === null ? 'Overall Elo' : `${surfaceLabel(surface)} Elo`}
      </h2>

      {eloA !== null && eloB !== null ? (
        <SplitBar
          label={
            surface === null ? 'Current Elo' : `Current ${surfaceLabel(surface).toLowerCase()} Elo`
          }
          left={eloA}
          right={eloB}
          format={(value) => String(Math.round(value))}
        />
      ) : (
        <p className={styles.absent}>
          <span>Current Elo</span>
          <span>
            {eloA === null && eloB === null
              ? 'Neither player has a rating in this series'
              : `Only ${eloA !== null ? playerA.name : playerB.name} has a rating in this series`}
          </span>
        </p>
      )}

      {ratingsA.state === 'ready' && ratingsB.state === 'ready' ? (
        <TrajectoryPair
          a={{ name: playerA.name, points: ratingsA.data.points.map(toPoint) }}
          b={{ name: playerB.name, points: ratingsB.data.points.map(toPoint) }}
        />
      ) : null}

      <Note>
        A rating a player never earned is absent rather than 1500: an unplayed surface is not a
        rating of average. A career that ended keeps its last rating, which is a real number
        and the wrong one to read as form.
      </Note>
    </section>
  )
}

function toPoint(point: { as_of: string; elo: number }) {
  return { date: point.as_of, elo: point.elo }
}

function ratingOf(profile: Resource<PlayerProfile>, series: string): number | null {
  if (profile.state !== 'ready' || profile.data.ratings === null) return null
  return profile.data.ratings.find((rating) => rating.surface === series)?.current.elo ?? null
}

function MeetingsSection({
  meetings,
  playerA,
  playerB,
}: {
  meetings: ReadonlyArray<Meeting>
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
}) {
  const [openChart, setOpenChart] = useState<string | null>(null)
  const columns: ReadonlyArray<Column<Meeting>> = [
    { key: 'date', header: 'Date', value: (row) => row.date },
    {
      key: 'winner',
      wrap: true,
      header: 'Won by',
      value: (row) => (row.winner_index === 0 ? playerA.name : playerB.name),
      render: (row) => {
        const winner = row.winner_index === 0 ? playerA.name : playerB.name
        return (
          <span className={row.winner_index === 0 ? styles.winnerA : styles.winnerB}>
            <span className={styles.full}>{winner}</span>
            <span className={styles.short} aria-hidden="true">
              {surname(winner)}
            </span>
          </span>
        )
      },
    },
    {
      key: 'event',
      wrap: true,
      header: 'Event',
      value: (row) => row.tournament,
      render: (row) => (
        <>
          <EventLink name={row.tournament} slug={row.event_slug} season={row.season} /> {row.round}
          {row.qualifying ? ' Q' : ''}
          <div className={styles.event}>
            {/* The surface column steps aside on a phone; its square moves here. */}
            <span className={styles.eventSurface}>
              <SurfaceDot surface={row.surface} label={false} />{' '}
            </span>
            {tierLabel(row.tier) ?? row.tier}
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
      wrap: true,
      minWidth: '10ch',
      value: (row) => row.score,
      render: (row) => (
        <>
          <Score score={row.score} incomplete={row.incomplete} />
          {row.charting_id !== null ? (
            <ChartedMark
              open={row.charting_id === openChart}
              onToggle={() =>
                setOpenChart((current) => (current === row.charting_id ? null : row.charting_id))
              }
            />
          ) : null}
        </>
      ),
      sortable: false,
    },
  ]

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle} id="every-meeting">Every meeting</h2>
      <StatTable
        caption="Oldest first, which is how a rivalry reads. A retirement counts in the record and is marked."
        columns={columns}
        rows={meetings}
        rowKey={(row) => `${row.date}-${row.tournament}-${row.round}`}
        detail={(row) =>
          row.charting_id !== null && row.charting_id === openChart ? (
            <ChartedSheet chartingId={row.charting_id} first={playerA.slug} />
          ) : null
        }
      />
    </section>
  )
}

const LEVELS = [
  { value: null, label: 'Every level' },
  { value: 'slam', label: 'Slams' },
  { value: 'masters', label: 'Masters' },
  { value: 'finals', label: 'Finals' },
  { value: 'tour', label: 'Tour' },
  { value: 'challenger', label: 'Challenger' },
  { value: 'futures', label: 'Futures' },
  { value: 'itf', label: 'ITF' },
]

const ROUNDS = [
  { value: '', label: 'Any round' },
  { value: 'F', label: 'Finals' },
  { value: 'SF', label: 'Semifinals' },
  { value: 'QF', label: 'Quarterfinals' },
  { value: 'R16', label: 'Round of 16' },
  { value: 'R32', label: 'Round of 32' },
  { value: 'R64', label: 'Round of 64' },
  { value: 'R128', label: 'Round of 128' },
  { value: 'RR', label: 'Round robin' },
  { value: 'Q', label: 'Qualifying' },
]

const ROUND_WORDS: Record<string, string> = {
  F: 'in finals',
  SF: 'in semifinals',
  QF: 'in quarterfinals',
  R16: 'in the round of 16',
  R32: 'in the round of 32',
  R64: 'in the round of 64',
  R128: 'in the round of 128',
  RR: 'in round robins',
  Q: 'in qualifying',
}

const LEVEL_WORDS: Record<string, string> = {
  slam: 'at Slams',
  masters: 'at Masters',
  finals: 'at tour finals',
  olympics: 'at the Olympics',
  team: 'in team competitions',
  tour: 'at tour level',
  challenger: 'at Challenger level',
  futures: 'at Futures level',
  itf: 'at ITF level',
}

/** The filters as the URL carries them, with one key for the fetch to depend on. */
function useMeetingFilters() {
  const [level, setLevel] = useUrlParam('level')
  const [round, setRound] = useUrlParam('round')
  const [bestOf, setBestOf] = useUrlParam('best_of')
  const [surface, setSurface] = useUrlParam('surface')
  const [deciders, setDeciders] = useUrlParam('deciders')
  const [tiebreaks, setTiebreaks] = useUrlParam('tiebreaks')
  const [from, setFrom] = useUrlParam('from')
  const [to, setTo] = useUrlParam('to')
  const values: MeetingFilters = { level, round, best_of: bestOf, surface, deciders, tiebreaks, from, to }
  return {
    values,
    key: JSON.stringify(values),
    setLevel,
    setRound,
    setBestOf,
    setSurface,
    setDeciders,
    setTiebreaks,
    setFrom,
    setTo,
    clear() {
      for (const set of [setLevel, setRound, setBestOf, setSurface, setDeciders, setTiebreaks, setFrom, setTo]) set(null)
    },
  }
}

function isFiltered(f: HeadToHeadFilters): boolean {
  return (
    f.level !== null || f.round !== null || f.best_of !== null || f.surface !== null ||
    f.deciders || f.tiebreaks || f.from !== null || f.to !== null
  )
}

/** The cut in words, so "3-1 in finals" is a whole sentence. */
function describeFilters(f: HeadToHeadFilters): string {
  const parts: string[] = []
  if (f.round !== null) parts.push(ROUND_WORDS[f.round] ?? f.round)
  if (f.level !== null) parts.push(LEVEL_WORDS[f.level] ?? f.level)
  if (f.surface !== null) parts.push(`on ${f.surface}`)
  if (f.best_of !== null) parts.push(`in best of ${f.best_of}`)
  if (f.deciders) parts.push('that went the distance')
  if (f.tiebreaks) parts.push('with a tiebreak')
  if (f.from !== null && f.to !== null) parts.push(f.from === f.to ? `in ${f.from}` : `from ${f.from} to ${f.to}`)
  else if (f.from !== null) parts.push(`since ${f.from}`)
  else if (f.to !== null) parts.push(`up to ${f.to}`)
  return parts.join(', ')
}

/**
 * The cut: typed cells and fields, every one in the URL. Surfaces are only
 * the ones they met on, because a surface they never met on is a filter that
 * can only ever empty the page; the rest are offered whole, and an empty cut
 * is an answer the page gives in words.
 */
function MeetingFilterControls({
  comparison,
  filters,
}: {
  comparison: Comparison
  filters: ReturnType<typeof useMeetingFilters>
}) {
  const f = comparison.filters
  const played = comparison.surfaces.map((split) => split.name).filter((name) => name !== 'unknown')
  const cell = (active: boolean, label: string, onClick: () => void, pressed = true) => (
    <button
      key={label}
      type="button"
      className={active ? `${styles.cell} ${styles.cellActive}` : styles.cell}
      aria-pressed={pressed ? active : undefined}
      onClick={onClick}
    >
      {label}
    </button>
  )
  return (
    <div className={styles.filters}>
      <div className={styles.cells} role="group" aria-label="Filter by level">
        {LEVELS.map((option) => cell(f.level === option.value, option.label, () => filters.setLevel(option.value)))}
      </div>
      <div className={styles.row}>
        <label className={styles.field} htmlFor="h2h-round">
          <span className={styles.fieldLabel}>Round</span>
          <select
            id="h2h-round"
            className={styles.select}
            value={f.round ?? ''}
            onChange={(event) => filters.setRound(event.target.value || null)}
          >
            {ROUNDS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <div className={styles.cells} role="group" aria-label="Filter by match length">
          {cell(f.best_of === null, 'Any length', () => filters.setBestOf(null))}
          {cell(f.best_of === 3, 'Best of 3', () => filters.setBestOf('3'))}
          {cell(f.best_of === 5, 'Best of 5', () => filters.setBestOf('5'))}
        </div>
      </div>
      {played.length > 1 ? (
        <SurfaceToggle value={f.surface} onChange={filters.setSurface} options={played} />
      ) : null}
      <div className={styles.row}>
        <div className={styles.cells} role="group" aria-label="Filter by closeness">
          {cell(f.deciders, 'Went the distance', () => filters.setDeciders(f.deciders ? null : 'true'))}
          {cell(f.tiebreaks, 'With a tiebreak', () => filters.setTiebreaks(f.tiebreaks ? null : 'true'))}
        </div>
        <label className={styles.field} htmlFor="h2h-from">
          <span className={styles.fieldLabel}>From</span>
          <input
            id="h2h-from"
            className={styles.year}
            type="number"
            inputMode="numeric"
            min={1900}
            max={2100}
            placeholder="first"
            value={f.from ?? ''}
            onChange={(event) => filters.setFrom(event.target.value)}
          />
        </label>
        <label className={styles.field} htmlFor="h2h-to">
          <span className={styles.fieldLabel}>To</span>
          <input
            id="h2h-to"
            className={styles.year}
            type="number"
            inputMode="numeric"
            min={1900}
            max={2100}
            placeholder="last"
            value={f.to ?? ''}
            onChange={(event) => filters.setTo(event.target.value)}
          />
        </label>
        {isFiltered(f) ? <Button onClick={filters.clear}>Show every meeting</Button> : null}
      </div>
    </div>
  )
}
