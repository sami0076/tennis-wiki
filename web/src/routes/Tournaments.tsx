import { Fragment, useState } from 'react'
import { Link } from 'react-router-dom'
import type { EventSummary, Page } from '../api/client'
import { getEvents } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import { Button, EmptyState, Skeleton, TourFilter } from '../components'
import { useDebounced } from '../lib/useDebounced'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Tournaments.module.css'
import { breadcrumbs, useJsonLd } from '../lib/jsonld'

const LEVELS = [
  { value: null, label: 'Every level' },
  { value: 'slam', label: 'Grand Slams' },
  { value: 'masters', label: 'Masters' },
  { value: 'finals', label: 'Finals' },
  { value: 'olympics', label: 'Olympics' },
  { value: 'team', label: 'Team' },
  { value: 'tour', label: 'Tour' },
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

const PAGE = 100

/**
 * The tournament index: every event on both tours, grouped by the level of
 * its latest edition, with a search, because ten thousand Challengers and ITF
 * events as one wall is not a page. The search, the tour and the level live
 * in the URL, so a filtered index is a link.
 */
export function Tournaments() {
  const [q, setQ] = useUrlParam('q')
  const [tour, setTour] = useUrlParam('tour')
  const [level, setLevel] = useUrlParam('level')
  const [cursors, setCursors] = useState<string[]>([])
  const query = useDebounced(q?.trim() ?? '')
  useJsonLd('breadcrumbs', breadcrumbs([{ name: 'Tournaments', path: '/tournaments' }]))

  const page = useResource(
    (signal) => getEvents({ q: query, tour, level, limit: PAGE, cursor: cursors.at(-1) ?? null }, signal),
    [query, tour, level, cursors.length],
  )

  // Any change to the question invalidates a cursor into the answer before it.
  function ask(next: () => void) {
    setCursors([])
    next()
  }

  return (
    <>
      <h1 className={styles.title}>Tournaments</h1>
      <p className={styles.seasons}>
        Or the calendar a year at a time: <Link to="/seasons">Seasons</Link>
      </p>

      <div className={styles.controls}>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="tournament-search">
            Tournament
          </label>
          <input
            id="tournament-search"
            className={styles.input}
            type="search"
            autoComplete="off"
            placeholder="Any part of a name"
            value={q ?? ''}
            onChange={(event) => ask(() => setQ(event.target.value))}
          />
        </div>
        <TourFilter value={tour} onChange={(next) => ask(() => setTour(next))} />
        <div className={styles.levels} role="group" aria-label="Filter by level">
          {LEVELS.map((option) => (
            <button
              key={option.label}
              type="button"
              className={level === option.value ? `${styles.level} ${styles.active}` : styles.level}
              aria-pressed={level === option.value}
              onClick={() => ask(() => setLevel(option.value))}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <Index page={page} query={query} cursors={cursors} setCursors={setCursors} />
    </>
  )
}

interface IndexProps {
  page: Resource<Page<EventSummary>>
  query: string
  cursors: string[]
  setCursors: (update: (current: string[]) => string[]) => void
}

function Index({ page, query, cursors, setCursors }: IndexProps) {
  if (page.state === 'loading') return <Skeleton lines={12} />

  if (page.state === 'error') {
    return (
      <p className={styles.error}>
        The index could not be loaded: {page.error.message} The API may not be running; start it
        with <code>make api</code> and reload.
      </p>
    )
  }

  const rows = page.data.data
  if (rows.length === 0) {
    return (
      <EmptyState
        heading={query === '' ? 'No tournament at that level' : `No tournament matches "${query}"`}
        reason="Names are the tour's own, and an event is filed under the name of its latest edition: Canada Masters rather than Montreal or Toronto, Dallas rather than San Jose. Any part of a name will do."
      />
    )
  }

  return (
    <>
      <ol className={styles.index}>
        {rows.map((event, i) => (
          <Fragment key={event.slug}>
            {i === 0 || rows[i - 1]?.category !== event.category ? (
              <li className={styles.group} aria-hidden="true">
                {CATEGORY_WORDS[event.category] ?? event.category}
              </li>
            ) : null}
            <li className={styles.row}>
              <Link className={styles.name} to={`/tournaments/${event.slug}`}>
                {event.name}
              </Link>
              <span className={styles.meta}>
                <span>{event.tour.toUpperCase()}</span>
                <span>
                  {event.first_season === event.last_season
                    ? event.first_season
                    : `${event.first_season}-${event.last_season}`}
                </span>
                <span>
                  {event.editions} {event.editions === 1 ? 'edition' : 'editions'}
                </span>
              </span>
            </li>
          </Fragment>
        ))}
      </ol>
      <p className={styles.caption}>
        Grouped by the level of the latest edition. A name is an event across seasons: keyed by
        the tour&apos;s number where the tour keeps one, and by name where it does not.
      </p>
      <div className={styles.more}>
        {page.data.next_cursor ? (
          <Button onClick={() => setCursors((current) => [...current, page.data.next_cursor as string])}>
            Show the next {PAGE}
          </Button>
        ) : null}
        {cursors.length > 0 ? (
          <Button onClick={() => setCursors(() => [])}>Back to the top of the list</Button>
        ) : null}
      </div>
    </>
  )
}
