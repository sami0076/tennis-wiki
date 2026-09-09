import { useState } from 'react'
import { Link } from 'react-router-dom'
import { MIN_QUERY, usePlayerSearch } from '../api/useSearch'
import { Button, ButtonLink, EmptyState, PlayerSummary, Skeleton, TourFilter } from '../components'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Players.module.css'

/**
 * The full result list, which is where the header typeahead sends anyone whose
 * namesake was not in the first eight rows.
 *
 * The query and the tour filter both live in the URL, so a search is a link
 * somebody can send.
 */
export function Players() {
  const [query, setQuery] = useUrlParam('q')
  const [tour, setTour] = useUrlParam('tour')
  const [cursors, setCursors] = useState<string[]>([])

  const search = usePlayerSearch(query ?? '', {
    tour,
    limit: 25,
    cursor: cursors.at(-1) ?? null,
  })

  // Any change to what is being asked invalidates a cursor into the old answer.
  function ask(next: () => void) {
    setCursors([])
    next()
  }

  return (
    <>
      <h1 className={styles.title}>Players</h1>

      <div className={styles.controls}>
        <label className={styles.label} htmlFor="player-search">
          Search by name
        </label>
        <input
          id="player-search"
          className={styles.input}
          type="search"
          autoComplete="off"
          placeholder="Surname, or any part of a name"
          value={query ?? ''}
          onChange={(event) => ask(() => setQuery(event.target.value))}
        />
        <TourFilter value={tour} onChange={(next) => ask(() => setTour(next))} />
      </div>

      <Results search={search} cursors={cursors} setCursors={setCursors} />
    </>
  )
}

interface ResultsProps {
  search: ReturnType<typeof usePlayerSearch>
  cursors: string[]
  setCursors: (update: (current: string[]) => string[]) => void
}

function Results({ search, cursors, setCursors }: ResultsProps) {
  if (search.error !== null) {
    return (
      <p className={styles.error}>
        The search could not run: {search.error.message} Reload, or check that the API is
        running with <code>make api</code>.
      </p>
    )
  }

  if (search.tooShort) {
    return (
      <p className={styles.hint}>
        Type at least {MIN_QUERY} characters. Diacritics and near-misses are handled by the
        search itself, so &ldquo;Djokovi&#263;&rdquo; and &ldquo;djokovic&rdquo; find the same
        player.
      </p>
    )
  }

  // The first query for a term has nothing to show yet. Later ones keep the
  // previous rows up rather than flashing the layout away and back.
  if (search.query === '') return <Skeleton lines={8} />

  if (search.results.length === 0) {
    return (
      <EmptyState
        heading={`No player matches "${search.query}"`}
        reason="Names are spelled as the tour's own records spell them, and a player with no ingested matches is still listed. A surname on its own usually finds more than a full name does."
        action={<ButtonLink to="/">See what the database covers</ButtonLink>}
      />
    )
  }

  return (
    <>
      <ul className={styles.results}>
        {search.results.map((player) => (
          <li key={player.slug}>
            <Link className={styles.row} to={`/players/${player.slug}`}>
              <PlayerSummary player={player} />
            </Link>
          </li>
        ))}
      </ul>
      <p className={styles.caption}>
        Ranked by name similarity, weighted by the best level a player reached. Within a level
        it favours shorter names, so the match count is there to settle the ties it cannot.
      </p>
      <div className={styles.more}>
        {search.nextCursor !== null ? (
          <Button onClick={() => setCursors((current) => [...current, search.nextCursor as string])}>
            Show more results
          </Button>
        ) : null}
        {cursors.length > 0 ? (
          <Button onClick={() => setCursors(() => [])}>Back to the closest matches</Button>
        ) : null}
      </div>
    </>
  )
}
