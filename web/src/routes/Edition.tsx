import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, AvailabilityPartial, AvailabilityRecorded, type Edition as EditionData, type Event } from '../api/client'
import { getEdition, getEvent } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  Button,
  ButtonLink,
  CourtArt,
  DrawSheet,
  EmptyState,
  MatchList,
  Meta,
  RoundList,
  PageHeader,
  RoundStepper,
  Skeleton,
  SurfaceBadge,
} from '../components'
import { absenceReason } from '../lib/absence'
import { buildBracket, pageSheets, roundGroups, splitRounds, ROUND_WORDS, type SheetPage } from '../lib/bracket'
import { breadcrumbs, sportsEvent, useJsonLd } from '../lib/jsonld'
import { levelLabel } from '../lib/tier'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Edition.module.css'

/**
 * One season of a tournament as its draw sheet. From 880px the bracket is
 * drawn, paged as the printed sheet is; below it, one round at a time. The
 * page is the title, the sheet, the seeds and one caption, and nothing else.
 */
export function Edition() {
  const { slug = '', season: year = '' } = useParams()
  const season = Number(year)
  const edition = useResource((signal) => getEdition(slug, season, signal), [slug, season])
  // The event's other seasons, for the editions line. Its own request so the
  // sheet does not wait on it, and the line is simply absent if it fails.
  const event = useResource((signal) => getEvent(slug, signal), [slug])
  useJsonLd('event', edition.state === 'ready' ? sportsEvent(edition.data) : null)
  useJsonLd(
    'breadcrumbs',
    edition.state === 'ready'
      ? breadcrumbs([
          { name: 'Tournaments', path: '/tournaments' },
          { name: `${edition.data.event.name}, ${edition.data.event.tour.toUpperCase()}`, path: `/tournaments/${slug}` },
          { name: `${edition.data.name} ${edition.data.season}`, path: `/tournaments/${slug}/${season}` },
        ])
      : null,
  )

  if (edition.state === 'loading') {
    return (
      <>
        <Skeleton lines={3} width="40%" />
        <div className={styles.block}>
          <Skeleton lines={16} />
        </div>
      </>
    )
  }

  if (edition.state === 'error') {
    const notFound = edition.error instanceof ApiError && edition.error.status === 404
    return (
      <EmptyState
        heading={notFound ? 'No sheet for that season' : 'That sheet could not be loaded'}
        reason={notFound ? edition.error.message : edition.error.message}
        action={<ButtonLink to={`/tournaments/${slug}`}>See every edition</ButtonLink>}
      />
    )
  }

  return <Sheet edition={edition.data} event={event} />
}

function Sheet({ edition, event }: { edition: EditionData; event: Resource<Event> }) {
  const tour = edition.event.tour.toUpperCase()
  const team = edition.link === 'team'
  const court =
    edition.surface === 'hard' || edition.surface === 'clay' || edition.surface === 'grass' ? edition.surface : null
  const bracket = useMemo(() => {
    const rounds = splitRounds(edition.matches)
    const main = buildBracket(edition.matches, rounds.main)
    const qualifying = buildBracket(edition.matches, rounds.qualifying)
    return {
      rounds,
      mainPages: pageSheets(main, rounds.main, 'main'),
      qualifyingPages: pageSheets(qualifying, rounds.qualifying, 'qualifying'),
    }
  }, [edition])

  return (
    <>
      <PageHeader
        kicker={
          <span className={styles.path}>
            <Link to="/tournaments">Tournaments</Link>
            {'  /  '}
            <Link to={`/tournaments/${edition.event.slug}`}>
              {edition.event.name}, {tour}
            </Link>
          </span>
        }
        title={
          <>
            {edition.name} {edition.season}
          </>
        }
        art={court === null ? undefined : <CourtArt surface={court} />}
      >
        <Meta
          className={styles.meta}
          parts={[
            tour,
            levelLabel(edition.level, edition.tier, edition.event.tour),
            edition.surface === null ? null : <SurfaceBadge surface={edition.surface} />,
            edition.draw_size === null ? null : `${edition.draw_size}\u00a0draw`,
            edition.start_date,
          ]}
        />
        <div className={styles.actions}>
          {!team && bracket.rounds.main.length > 0 ? (
            <ButtonLink to={`/simulator?event=${edition.event.slug}&season=${edition.season}`}>
              Replay this draw
            </ButtonLink>
          ) : null}
          <Editions edition={edition} event={event} />
        </div>
      </PageHeader>

      {team ? (
        <section className={styles.block}>
          <MatchList matches={edition.matches} groupBy={(match) => match.tie} />
        </section>
      ) : (
        <Draw edition={edition} pages={bracket.mainPages} qualifying={bracket.qualifyingPages} rounds={bracket.rounds} />
      )}

      {bracket.rounds.other.length > 0 ? (
        <section className={styles.block}>
          <h2 className={styles.heading}>{ROUND_WORDS[bracket.rounds.other[0]?.round ?? ''] ?? 'Other matches'}</h2>
          <MatchList matches={bracket.rounds.other} groupBy={(match) => ROUND_WORDS[match.round] ?? match.round} />
        </section>
      ) : null}

      {edition.seeds.length > 0 ? (
        <section className={styles.block}>
          <h2 className={styles.heading}>Seeds</h2>
          <ol className={styles.seeds}>
            {edition.seeds.map((seed) => (
              <li key={seed.seed} className={styles.seed}>
                <span className={styles.pre}>[{seed.seed}]</span>
                <Link className={styles.player} to={`/players/${seed.player.slug}`}>
                  {seed.player.name}
                </Link>
                <span className={styles.lead} aria-hidden="true" />
                <span className={seed.exit === 'W' ? `${styles.exit} ${styles.won}` : styles.exit}>{seed.exit}</span>
              </li>
            ))}
          </ol>
        </section>
      ) : null}

      <p className={styles.caption}>{caption(edition, event)}</p>
    </>
  )
}

interface DrawProps {
  edition: EditionData
  pages: SheetPage[]
  qualifying: SheetPage[]
  rounds: ReturnType<typeof splitRounds>
}

function Draw({ edition, pages, qualifying, rounds }: DrawProps) {
  const [lit, setLit] = useState<string | null>(null)
  const [showQualifying, setShowQualifying] = useState(false)
  const [round, setRound] = useUrlParam('round')
  const all = [...rounds.main, ...rounds.qualifying]
  const current = round !== null && all.includes(round) ? round : (rounds.main[0] ?? '')
  const inQualifying = rounds.qualifying.includes(current)
  const col = (inQualifying ? rounds.qualifying : rounds.main).indexOf(current) + 1

  if (pages.length === 0) {
    return (
      <EmptyState
        heading="No draw to show"
        reason={`The file carries ${edition.matches.length} matches for this season and none of them in a round the sheet knows how to place.`}
      />
    )
  }

  return (
    <>
      <div className={styles.desk}>
        {pages.map((page, i) => (
          <section key={i} className={styles.block}>
            {page.title !== null ? <h2 className={styles.heading}>{page.title}</h2> : null}
            <DrawSheet page={page} kind="main" lit={lit} onLit={setLit} />
          </section>
        ))}
        {qualifying.length > 0 ? (
          <section className={styles.block}>
            <h2 className={styles.heading}>Qualifying</h2>
            {showQualifying ? (
              qualifying.map((page, i) => (
                <div key={i} className={styles.qualifyingPage}>
                  {page.title !== null ? <h3 className={styles.subheading}>{page.title}</h3> : null}
                  <DrawSheet page={page} kind="qualifying" lit={lit} onLit={setLit} />
                </div>
              ))
            ) : (
              <Button onClick={() => setShowQualifying(true)}>Show the qualifying draw</Button>
            )}
          </section>
        ) : null}
      </div>

      <div className={styles.phone}>
        <RoundStepper
          rounds={rounds.main}
          qualifying={rounds.qualifying}
          value={current}
          onChange={(next) => setRound(next === rounds.main[0] ? null : next)}
        />
        <h2 className={styles.roundTitle}>{ROUND_WORDS[current] ?? current}</h2>
        <RoundList groups={roundGroups(inQualifying ? qualifying : pages, col)} final={current === 'F'} />
      </div>
    </>
  )
}

/** The seasons either side of this one, with this one in brackets. */
function Editions({ edition, event }: { edition: EditionData; event: Resource<Event> }) {
  if (event.state !== 'ready') return null
  const seasons = event.data.editions.map((e) => e.season)
  const at = seasons.indexOf(edition.season)
  const near = seasons.slice(Math.max(0, at - 2), at + 3)
  return (
    <p className={styles.editions}>
      Editions
      {near.map((season) => (
        <span key={season}>
          {'  '}
          {season === edition.season ? (
            <span className={styles.current}>[{season}]</span>
          ) : (
            <Link to={`/tournaments/${edition.event.slug}/${season}`}>{season}</Link>
          )}
        </span>
      ))}
      {seasons.length > near.length ? (
        <>
          {'  '}
          <Link to={`/tournaments/${edition.event.slug}`}>all {seasons.length}</Link>
        </>
      ) : null}
    </p>
  )
}

function caption(edition: EditionData, event: Resource<Event>): string {
  const tour = edition.event.tour.toUpperCase()
  const serve =
    edition.serve.availability === AvailabilityRecorded
      ? `Serve statistics were recorded for all ${edition.serve.matches} matches.`
      : edition.serve.availability === AvailabilityPartial
        ? `Serve statistics were recorded for ${edition.serve.matches_with} of ${edition.serve.matches} matches.`
        : absenceReason(edition.serve.availability)
  const number = event.state === 'ready' ? event.data.number : null
  const filed: Record<string, string> = {
    number: number === null ? `Filed under the ${tour}'s number.` : `Filed under the ${tour}'s number ${number}.`,
    override: 'Filed under a checked renumbering.',
    bridged: 'Joined to this event by name, bridging to its number.',
    name: 'Joined to this event by name; the file gives it no number.',
    team: 'One row per tie in the file, folded into a season.',
  }
  const lines = edition.link === 'team' ? '' : ' Within a pair the winner is written first: the file has no draw lines.'
  return `From the ${tour} ${edition.season} match file.${lines} ${serve} ${filed[edition.link] ?? ''}`.trim()
}
