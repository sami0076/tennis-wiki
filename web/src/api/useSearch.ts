import { useEffect, useState } from 'react'
import { searchPlayers } from './endpoints'
import type { PlayerSearchResult } from './types.gen'

/**
 * Mirrors the API's own floor. Below two characters a trigram query matches
 * most of the database and ranks nothing usefully, so the API answers 400 --
 * asking anyway would turn every first keystroke into an error.
 */
export const MIN_QUERY = 2

/** Long enough that a typed name is one request, short enough to feel live. */
const DEBOUNCE_MS = 180

export interface PlayerSearchState {
  results: PlayerSearchResult[]
  nextCursor: string | null
  /** A request is in flight. */
  busy: boolean
  error: Error | null
  /**
   * The query the results actually answer. It lags the argument while typing,
   * which is what lets a caller tell fresh results from the ones still on
   * screen from the previous keystroke.
   */
  query: string
  /** Nothing was requested, because the query is below MIN_QUERY. */
  tooShort: boolean
}

const idle: PlayerSearchState = {
  results: [],
  nextCursor: null,
  busy: false,
  error: null,
  query: '',
  tooShort: true,
}

/**
 * usePlayerSearch runs the search endpoint as a query is typed.
 *
 * Two things a naive fetch-per-keystroke gets wrong, and both are the point of
 * this hook: the request is debounced, so typing a name is one query rather
 * than one per letter, and the previous request is aborted when a new one
 * starts, so a slow "nov" cannot land after a fast "novak" and overwrite it.
 *
 * Results from the previous query stay on screen while the next one is in
 * flight. Blanking the list on every keystroke would make the thing flicker
 * through empty states that were never true.
 */
export function usePlayerSearch(
  query: string,
  options: { tour?: string | null; limit?: number; cursor?: string | null } = {},
): PlayerSearchState {
  const { tour = null, limit = 8, cursor = null } = options
  const trimmed = query.trim()
  const tooShort = trimmed.length < MIN_QUERY

  const [state, setState] = useState<PlayerSearchState>(idle)

  useEffect(() => {
    if (tooShort) {
      setState(idle)
      return
    }

    const controller = new AbortController()
    setState((current) => ({ ...current, busy: true, error: null, tooShort: false }))

    const timer = setTimeout(() => {
      searchPlayers(trimmed, { tour, limit, cursor }, controller.signal)
        .then((page) => {
          // fetch rejects an aborted request, but only once it notices. The
          // guard is what makes the newer query the winner rather than the
          // faster one.
          if (controller.signal.aborted) return
          setState({
            results: page.data,
            nextCursor: page.next_cursor === '' ? null : page.next_cursor,
            busy: false,
            error: null,
            query: trimmed,
            tooShort: false,
          })
        })
        .catch((err: unknown) => {
          if (controller.signal.aborted) return
          setState((current) => ({
            ...current,
            busy: false,
            error: err instanceof Error ? err : new Error(String(err)),
          }))
        })
    }, DEBOUNCE_MS)

    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [trimmed, tour, limit, cursor, tooShort])

  return state
}
