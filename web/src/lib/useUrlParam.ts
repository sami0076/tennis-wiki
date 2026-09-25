import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'

/**
 * useUrlParam keeps a filter in the query string, so a filtered view can be
 * reloaded, bookmarked and shared. A surface toggle whose state lives only in
 * React is a view nobody can link to.
 *
 * Setting it to null removes the parameter rather than writing an empty one,
 * which keeps the URL honest about what is actually filtered.
 */
export function useUrlParam(name: string): [string | null, (value: string | null) => void] {
  const [params, setParams] = useSearchParams()

  const set = useCallback(
    (value: string | null) => {
      setParams(
        (current) => {
          const next = new URLSearchParams(current)
          if (value === null || value === '') {
            next.delete(name)
          } else {
            next.set(name, value)
          }
          return next
        },
        { replace: true },
      )
    },
    [name, setParams],
  )

  return [params.get(name), set]
}

/**
 * Set several parameters in one navigation.
 *
 * Two useUrlParam setters called in the same tick clobber each other: each
 * resolves against the URL as it was before either ran, and the last
 * navigation is the one that sticks. That is invisible until something needs
 * two parameters to agree.
 *
 * The draw on the simulator is the case that found it. A draw is addressed by
 * an event slug *and* a season, so writing them one at a time leaves the page
 * pointing at an event that never played that year -- or, as it actually did,
 * at a season with no event at all.
 */
export function useUrlParams(): (values: Record<string, string | null>) => void {
  const [, setParams] = useSearchParams()

  return useCallback(
    (values: Record<string, string | null>) => {
      setParams(
        (current) => {
          const next = new URLSearchParams(current)
          for (const [name, value] of Object.entries(values)) {
            if (value === null || value === '') next.delete(name)
            else next.set(name, value)
          }
          return next
        },
        { replace: true },
      )
    },
    [setParams],
  )
}
