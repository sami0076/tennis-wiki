import { waitFor } from '@testing-library/react'

/**
 * The JSON-LD of a given @type, once it is in the document head.
 *
 * useJsonLd appends its script from an effect, so waiting for the heading that
 * carries the same data is waiting for the wrong thing: the render can be
 * committed and painted with the effect still pending. It holds on a fast
 * machine and fails on a loaded one, which is how this first showed up -- green
 * locally through repeated full runs, red once on CI.
 */
export async function findJsonLd(type: string): Promise<Record<string, unknown>> {
  return waitFor(() => {
    const scripts = Array.from(document.head.querySelectorAll('script[type="application/ld+json"]'))
    const found = scripts
      .map((script) => JSON.parse(script.textContent ?? '{}') as Record<string, unknown>)
      .find((ld) => ld['@type'] === type)
    if (found === undefined) throw new Error(`no ${type} in the document head`)
    return found
  })
}
