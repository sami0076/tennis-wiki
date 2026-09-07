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
