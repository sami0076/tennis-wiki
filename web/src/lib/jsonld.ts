import { useEffect } from 'react'
import type { Edition, PlayerProfile } from '../api/client'

/**
 * Structured data for the crawlers that execute scripts. Every value here is
 * copied from an API response, never inferred: a field the API left null is
 * left out, not written as null, and nothing is invented to fill a slot the
 * schema offers (SportsEvent has a location; the files do not).
 */

export const SITE = 'https://deucepoint.net'

type JsonLd = Record<string, unknown>

/** Person for a player page: name, nationality and birth date where known. */
export function person(profile: PlayerProfile): JsonLd {
  const out: JsonLd = {
    '@context': 'https://schema.org',
    '@type': 'Person',
    name: profile.name,
    url: `${SITE}/players/${profile.slug}`,
  }
  if (profile.country !== null) out.nationality = profile.country
  if (profile.birth_date !== null) out.birthDate = profile.birth_date
  return out
}

/**
 * SportsEvent for an edition: the name, when it began, the surface and draw
 * in the description, and the finalists as competitors. No location: the
 * files carry none, and a city read off the name would be a guess.
 */
export function sportsEvent(edition: Edition): JsonLd {
  const out: JsonLd = {
    '@context': 'https://schema.org',
    '@type': 'SportsEvent',
    name: `${edition.name} ${edition.season}`,
    sport: 'Tennis',
    startDate: edition.start_date,
    url: `${SITE}/tournaments/${edition.event.slug}/${edition.season}`,
  }
  const parts = [edition.event.tour.toUpperCase()]
  if (edition.surface !== null) parts.push(edition.surface)
  if (edition.draw_size !== null) parts.push(`${edition.draw_size} draw`)
  out.description = parts.join(', ')
  const competitors = [edition.champion, edition.finalist]
    .filter((side): side is NonNullable<typeof side> => side !== null)
    .map((side) => ({ '@type': 'Person', name: side.name, url: `${SITE}/players/${side.slug}` }))
  if (competitors.length > 0) out.competitor = competitors
  return out
}

export interface Crumb {
  name: string
  path: string
}

/** BreadcrumbList for every route below the root, the site as the first crumb. */
export function breadcrumbs(crumbs: ReadonlyArray<Crumb>): JsonLd {
  const items = [{ name: 'Deucepoint', path: '/' }, ...crumbs]
  return {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: items.map((crumb, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: crumb.name,
      item: `${SITE}${crumb.path}`,
    })),
  }
}

/** WebSite on the root, with the player search as its search action. */
export function website(): JsonLd {
  return {
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    name: 'Deucepoint',
    url: SITE,
    potentialAction: {
      '@type': 'SearchAction',
      target: {
        '@type': 'EntryPoint',
        urlTemplate: `${SITE}/players?q={search_term_string}`,
      },
      'query-input': 'required name=search_term_string',
    },
  }
}

/**
 * useJsonLd writes one JSON-LD script into the document head for as long as
 * the page is mounted, keyed so a page can carry more than one type and a
 * route change replaces rather than accumulates. Null writes nothing, for a
 * page whose data has not arrived.
 */
export function useJsonLd(key: string, data: JsonLd | null) {
  const json = data === null ? null : JSON.stringify(data)
  useEffect(() => {
    if (json === null) return
    const script = document.createElement('script')
    script.type = 'application/ld+json'
    script.dataset.jsonld = key
    script.textContent = json
    document.head.appendChild(script)
    return () => {
      script.remove()
    }
  }, [key, json])
}
