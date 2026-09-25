import { describe, expect, it } from 'vitest'
import type { Edition, PlayerProfile } from '../api/client'
import { breadcrumbs, person, sportsEvent, website } from './jsonld'

const profile: PlayerProfile = {
  slug: 'rafael-nadal',
  name: 'Rafael Nadal',
  tour: 'atp',
  country: 'ESP',
  hand: 'L',
  height_cm: 185,
  birth_date: '1986-06-03',
  pro_since: 2001,
  career: null,
  serve: { availability: 'recorded', matches_with_data: 0, rates: null },
  return: { availability: 'recorded', matches_with_data: 0, rates: null },
  points: null,
  splits: null,
  ratings: null,
}

describe('person', () => {
  // The rule the issue asks for a test of: nothing in the JSON-LD that the
  // API response did not carry, and nothing the response left null.
  it('carries no field the API response did not', () => {
    const ld = person(profile)
    const sources: Record<string, unknown> = {
      '@context': 'https://schema.org',
      '@type': 'Person',
      name: profile.name,
      url: `https://deucepoint.net/players/${profile.slug}`,
      nationality: profile.country,
      birthDate: profile.birth_date,
    }
    for (const [key, value] of Object.entries(ld)) {
      expect(sources, `unexpected field ${key}`).toHaveProperty(key)
      expect(value).toEqual(sources[key])
    }
  })

  it('leaves out what the source did not record rather than writing null', () => {
    const ld = person({ ...profile, country: null, birth_date: null })
    expect(ld).not.toHaveProperty('nationality')
    expect(ld).not.toHaveProperty('birthDate')
    expect(Object.values(ld)).not.toContain(null)
  })
})

describe('sportsEvent', () => {
  const edition: Edition = {
    event: { slug: 'wimbledon-atp', name: 'Wimbledon', tour: 'atp' },
    season: 2019,
    name: 'Wimbledon',
    level: 'G',
    tier: 'tour',
    surface: 'grass',
    draw_size: 128,
    start_date: '2019-07-01',
    link: 'number',
    champion: { slug: 'novak-djokovic', name: 'Novak Djokovic' },
    finalist: { slug: 'roger-federer', name: 'Roger Federer' },
    final_score: '7-6(5) 1-6 7-6(4) 4-6 13-12(3)',
    serve: { availability: 'recorded', matches_with: 239, matches: 239 },
    seeds: [],
    matches: [],
  }

  it('names the edition, its start and its finalists, and invents no location', () => {
    const ld = sportsEvent(edition)
    expect(ld).toMatchObject({
      '@type': 'SportsEvent',
      name: 'Wimbledon 2019',
      startDate: '2019-07-01',
      description: 'ATP, grass, 128 draw',
    })
    expect(ld).not.toHaveProperty('location')
    expect(ld.competitor).toEqual([
      { '@type': 'Person', name: 'Novak Djokovic', url: 'https://deucepoint.net/players/novak-djokovic' },
      { '@type': 'Person', name: 'Roger Federer', url: 'https://deucepoint.net/players/roger-federer' },
    ])
  })

  it('has no competitors when the file has no final', () => {
    const ld = sportsEvent({ ...edition, champion: null, finalist: null })
    expect(ld).not.toHaveProperty('competitor')
  })
})

describe('breadcrumbs and website', () => {
  it('leads every trail from the site', () => {
    const ld = breadcrumbs([{ name: 'Tournaments', path: '/tournaments' }, { name: 'Wimbledon', path: '/tournaments/wimbledon-atp' }])
    const items = ld.itemListElement as { position: number; name: string; item: string }[]
    expect(items.map((i) => i.name)).toEqual(['Deucepoint', 'Tournaments', 'Wimbledon'])
    expect(items[2]).toMatchObject({ position: 3, item: 'https://deucepoint.net/tournaments/wimbledon-atp' })
  })

  it('points the search action at the player search', () => {
    const ld = website() as { potentialAction: { target: { urlTemplate: string } } }
    expect(ld.potentialAction.target.urlTemplate).toBe('https://deucepoint.net/players?q={search_term_string}')
  })
})
