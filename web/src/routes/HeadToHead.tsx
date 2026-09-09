import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  ApiError,
  type HeadToHead as Comparison,
  type HeadToHeadPlayer,
  type HeadToHeadRecord,
  type Meeting,
  type PlayerProfile,
  type RatingSeries,
  type ServeRates,
} from '../api/client'
import { getHeadToHead, getPlayer, getPlayerRatingSeries } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  Button,
  ButtonLink,
  EmptyState,
  Meta,
  PlayerSearch,
  RivalryStrip,
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
import { formatPercent, formatScore } from '../lib/format'
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

  return (
    <>
      <h1 className="sr-only">Head to head</h1>
      <Pickers a={a} b={b} />
      {a !== undefined && b !== undefined ? (
        <Rivalry a={a} b={b} />
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

/**
 * Two comboboxes, one per side. Choosing on either side rebuilds the URL, which
 * is what makes swapping a player a navigation rather than hidden state.
 */
function Pickers({ a, b }: { a?: string; b?: string }) {
  const navigate = useNavigate()
  const [queryA, setQueryA] = useState('')
  const [queryB, setQueryB] = useState('')
  const [clash, setClash] = useState(false)

  function choose(side: 'a' | 'b', slug: string) {
    const left = side === 'a' ? slug : a
    const right = side === 'b' ? slug : b
    if (left === right) {
      // The API answers 400 for this, correctly. Saying so here is faster and
      // does not cost a request.
      setClash(true)
      return
    }
    setClash(false)
    if (left === undefined || right === undefined) return
    navigate(`/h2h/${left}/${right}`)
  }

  return (
    <div className={styles.pickers}>
      <PlayerSearch
        label="First player"
        placeholder={a ?? 'Search by name'}
        value={queryA}
        onChange={setQueryA}
        onSelect={(player) => {
          setQueryA('')
          choose('a', player.slug)
        }}
      />
      <PlayerSearch
        label="Second player"
        placeholder={b ?? 'Search by name'}
        value={queryB}
        onChange={setQueryB}
        onSelect={(player) => {
          setQueryB('')
          choose('b', player.slug)
        }}
      />
      {clash ? (
        <p className={styles.error}>
          Pick two different players. A comparison with themselves would be 0&#8211;0 for both
          sides.
        </p>
      ) : null}
    </div>
  )
}

function Rivalry({ a, b }: { a: string; b: string }) {
  const [surface, setSurface] = useUrlParam('surface')
  const h2h = useResource((signal) => getHeadToHead(a, b, signal), [a, b])
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

  if (h2h.state === 'loading') return <Skeleton lines={10} />

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

  const comparison = h2h.data
  const [playerA, playerB] = comparison.players
  const met = comparison.meetings.filter(
    (meeting) => surface === null || meeting.surface === surface,
  )
  const record = surface === null ? comparison.record : recordOf(met)
  // Only the surfaces they actually met on: offering grass to a pair who never
  // played on it is a filter that can only ever empty the page.
  const played = comparison.surfaces
    .map((split) => split.name)
    .filter((name) => name !== 'unknown')

  return (
    <>
      <div className={styles.score}>
        <div>
          <Link className={styles.nameA} to={`/players/${playerA.slug}`}>
            {playerA.name}
          </Link>
          <Meta parts={[playerA.tour.toUpperCase(), playerA.country]} />
        </div>
        <div className={styles.tally}>
          {record.wins[0]}&#8211;{record.wins[1]}
        </div>
        <div>
          <Link className={styles.nameB} to={`/players/${playerB.slug}`}>
            {playerB.name}
          </Link>
          <Meta className={styles.metaB} parts={[playerB.tour.toUpperCase(), playerB.country]} />
        </div>
      </div>

      {played.length > 1 ? (
        <SurfaceToggle value={surface} onChange={setSurface} options={played} />
      ) : null}

      {record.matches === 0 ? (
        <NeverMet
          playerA={playerA}
          playerB={playerB}
          surface={surface}
          everMet={comparison.record.matches > 0}
          onClear={() => setSurface(null)}
        />
      ) : (
        <>
          <RivalrySection meetings={met} playerA={playerA} playerB={playerB} record={record} />
          <ServeSection
            comparison={comparison}
            playerA={playerA}
            playerB={playerB}
            surface={surface}
          />
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
        <Button disabled>Simulate this matchup</Button>
        <p className={styles.caption}>
          The simulator is Phase 3. Everything it needs from this pair is already on this page.
        </p>
      </div>
    </>
  )
}

/** A pair who never met, which is the normal case and reads as one. */
function NeverMet({
  playerA,
  playerB,
  surface,
  everMet,
  onClear,
}: {
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  surface: string | null
  everMet: boolean
  onClear: () => void
}) {
  if (everMet) {
    return (
      <EmptyState
        heading={`They never met on ${surfaceLabel(surface).toLowerCase()}`}
        reason="Every meeting between them was played on another surface."
        action={<Button onClick={onClear}>Show every surface</Button>}
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
}: {
  meetings: ReadonlyArray<Meeting>
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  record: HeadToHeadRecord
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
      <RivalryStrip results={results} nameA={playerA.name} nameB={playerB.name} />
      {record.incomplete > 0 ? (
        <p className={styles.caption}>
          {record.incomplete} of these {record.matches} ended in a retirement or a walkover.
          They count in the record, because somebody advanced, and are left out of every rate
          below.
        </p>
      ) : null}
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
  surface,
}: {
  comparison: Comparison
  playerA: HeadToHeadPlayer
  playerB: HeadToHeadPlayer
  surface: string | null
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
      <p className={styles.caption}>
        Over the {Number(serveA.matches_with_data)} of their meetings that recorded a serve
        line{surface === null ? '' : ', on every surface rather than only this one'}. The
        endpoint aggregates a rivalry once, so the surface filter above moves the record and
        the strip but not these.
      </p>
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

      <p className={styles.caption}>
        A rating a player never earned is absent rather than 1500: an unplayed surface is not a
        rating of average. A career that ended keeps its last rating, which is a real number
        and the wrong one to read as form.
      </p>
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
  const columns: ReadonlyArray<Column<Meeting>> = [
    { key: 'date', header: 'Date', value: (row) => row.date },
    {
      key: 'winner',
      header: 'Won by',
      value: (row) => (row.winner_index === 0 ? playerA.name : playerB.name),
      render: (row) => (
        <span className={row.winner_index === 0 ? styles.winnerA : styles.winnerB}>
          {row.winner_index === 0 ? playerA.name : playerB.name}
        </span>
      ),
    },
    {
      key: 'event',
      header: 'Event',
      value: (row) => row.tournament,
      render: (row) => (
        <>
          {row.tournament} {row.round}
          {row.qualifying ? ' Q' : ''}
          <div className={styles.event}>{tierLabel(row.tier) ?? row.tier}</div>
        </>
      ),
    },
    {
      key: 'surface',
      header: 'Surface',
      value: (row) => row.surface,
      render: (row) => <SurfaceDot surface={row.surface} />,
    },
    {
      key: 'score',
      header: 'Score',
      value: (row) => row.score,
      render: (row) => (
        <>
          {formatScore(row.score)}
          {row.incomplete ? <span className={styles.event}> incomplete</span> : null}
        </>
      ),
      sortable: false,
    },
  ]

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Every meeting</h2>
      <StatTable
        caption="Oldest first, which is how a rivalry reads. A retirement counts in the record and is marked."
        columns={columns}
        rows={meetings}
        rowKey={(row) => `${row.date}-${row.tournament}-${row.round}`}
      />
    </section>
  )
}

/** The record inside a surface filter, counted from the meetings themselves. */
function recordOf(meetings: ReadonlyArray<Meeting>): HeadToHeadRecord {
  let a = 0
  let b = 0
  let incomplete = 0
  for (const meeting of meetings) {
    if (meeting.winner_index === 0) a += 1
    else b += 1
    if (meeting.incomplete) incomplete += 1
  }
  return { matches: meetings.length, wins: [a, b], incomplete }
}
