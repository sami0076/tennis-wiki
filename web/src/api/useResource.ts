import { useEffect, useState } from 'react'

/**
 * Resource is the three states every screen has to render: loading, failed, and
 * loaded. Making them a union rather than three loose booleans is what stops a
 * component rendering a blank screen for a case it forgot about.
 */
export type Resource<T> =
  | { state: 'loading'; data: null; error: null }
  | { state: 'error'; data: null; error: Error }
  | { state: 'ready'; data: T; error: null }

const loading = { state: 'loading', data: null, error: null } as const

/**
 * useResource runs a fetch and cancels it if the inputs change or the component
 * goes away, so a slow first request cannot overwrite a fast second one.
 *
 * `deps` is what identifies the request. It is spread into the effect's
 * dependency list, so pass the slug and filters, not the function.
 */
export function useResource<T>(
  load: (signal: AbortSignal) => Promise<T>,
  deps: ReadonlyArray<unknown>,
): Resource<T> {
  const [resource, setResource] = useState<Resource<T>>(loading)

  useEffect(() => {
    const controller = new AbortController()
    setResource(loading)

    load(controller.signal)
      .then((data) => setResource({ state: 'ready', data, error: null }))
      .catch((err: unknown) => {
        // An aborted request is not a failure, and the component that would
        // have shown the error is on its way out anyway.
        if (controller.signal.aborted) return
        setResource({
          state: 'error',
          data: null,
          error: err instanceof Error ? err : new Error(String(err)),
        })
      })

    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  return resource
}
