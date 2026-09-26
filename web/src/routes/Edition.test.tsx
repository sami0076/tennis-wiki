import { fireEvent, render, screen, within } from '@testing-library/react'
import { findJsonLd } from '../test/jsonld'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Edition as EditionData, EditionMatch, EditionSide, Event } from '../api/client'
import { Edition } from './Edition'

function side(slug: string, name: string, seed: number | null = null): EditionSide {
  return { slug, name, country: 'SRB', seed, entry: null, rank: null }
}

function match(round: string, num: number, winner: EditionSide, loser: EditionSide, extra: Partial<EditionMatch> = {}): EditionMatch {
  return {
    round,
    match_num: num,
    qualifying: round.startsWith('Q'),
    best_of: 3,
    tie: null,
    players: [winner, loser],
    score: '6-4 6-4',
    incomplete: false,
    minutes: null,
    serve: [undefined, undefined],
    charting_id: null,
    ...extra,
  }
}

const ann = side('ann', 'Ann Ace', 1)
const bea = side('bea', 'Bea Base', 2)
const cat = side('cat', 'Cat Court')
const dee = side('dee', 'Dee Drop')

const edition: EditionData = {
  event: { slug: 'testville-wta', name: 'Testville', tour: 'wta' },
  season: 2025,
  name: 'Testville',
  level: 'P',
  tier: 'tour',
  surface: 'clay',
  draw_size: 4,
  start_date: '2025-05-01',
  link: 'number',
  champion: { slug: 'ann', name: 'Ann Ace' },
  finalist: { slug: 'bea', name: 'Bea Base' },
  final_score: '7-5 7-5',
  serve: { availability: 'partial', matches_with: 2, matches: 3 },
  seeds: [
    { seed: 1, player: { slug: 'ann', name: 'Ann Ace' }, exit: 'W' },
    { seed: 2, player: { slug: 'bea', name: 'Bea Base' }, exit: 'F' },
  ],
  matches: [
    match('SF', 1, ann, dee),
    match('SF', 2, bea, cat),
    match('F', 3, ann, bea, { score: '7-5 7-5' }),
    match('Q1', 4, cat, side('eve', 'Eve East')),
  ],
}

const event: Event = {
  slug: 'testville-wta',
  name: 'Testville',
  tour: 'wta',
  keyed: 'number',
  number: '777',
  first_season: 2015,
  last_season: 2025,
  names: [{ name: 'Testville', first_season: 2015, last_season: 2025 }],
  provenance: { number: 2, override: 0, bridged: 1, name: 0, team: 0 },
  editions: [2015, 2024, 2025].map((season) => ({
    season,
    name: 'Testville',
    level: 'P',
    tier: 'tour',
    surface: 'clay',
    draw_size: 4,
    start_date: `${season}-05-01`,
    link: season === 2015 ? 'bridged' : 'number',
    champion: { slug: 'ann', name: 'Ann Ace' },
    finalist: { slug: 'bea', name: 'Bea Base' },
    final_score: '7-5 7-5',
    matches: 3,
    ties: 0,
  })),
}

function stub(body: EditionData | null, status = 200) {
  vi.stubGlobal('fetch', (input: string) => {
    const path = new URL(String(input), 'http://localhost').pathname
    if (path === '/api/v1/tournaments/testville-wta') {
      return Promise.resolve(new Response(JSON.stringify(event), { status: 200 }))
    }
    const payload =
      body === null
        ? { type: '/problems/not-found', title: 'Not found', status: 404, detail: 'That tournament was not played that season.', instance: path, request_id: 'x' }
        : body
    return Promise.resolve(new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } }))
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/tournaments/:slug/:season" element={<Edition />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Edition', () => {
  it('is the title, the sheet, the seeds and one caption', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025')

    expect(await screen.findByRole('heading', { level: 1, name: 'Testville 2025' })).toBeInTheDocument()
    expect(screen.getByText(/Premier/)).toBeInTheDocument()
    expect(screen.getByText(/4 draw/)).toBeInTheDocument()
    // The sheet's columns and the champion, in 700 in the last column.
    expect(screen.getByText('Champion')).toBeInTheDocument()
    // The seeds and where each went out.
    const seeds = screen.getByRole('list')
    expect(within(seeds).getByText('W')).toBeInTheDocument()
    expect(within(seeds).getByText('F')).toBeInTheDocument()
    // One caption, with the denominator and how the edition is filed.
    expect(screen.getByText(/recorded for 2 of 3 matches/)).toBeInTheDocument()
    expect(screen.getByText(/Filed under the WTA's number 777/)).toBeInTheDocument()
  })

  it('links the replay to the simulator on the same edition', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025')
    expect(await screen.findByRole('link', { name: 'Replay this draw' })).toHaveAttribute(
      'href',
      '/simulator?event=testville-wta&season=2025',
    )
  })

  it('writes the neighbouring seasons with this one in brackets', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025')
    expect(await screen.findByText('[2025]')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '2024' })).toHaveAttribute('href', '/tournaments/testville-wta/2024')
  })

  it('steps through the rounds on the phone, keeping the round in the URL', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025?round=Q1')
    expect(await screen.findByRole('tab', { name: 'Q1' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('heading', { level: 2, name: 'Qualifying, first round' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'F' }))
    expect(screen.getByRole('heading', { level: 2, name: 'Final' })).toBeInTheDocument()
  })

  it('keeps qualifying behind a button on the sheet', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025')
    // The sheet is display: none below 880px, and jsdom has no viewport, so
    // the desktop side is queried by text rather than by role.
    const button = await screen.findByText('Show the qualifying draw')
    expect(screen.queryByText('Qualified')).not.toBeInTheDocument()
    fireEvent.click(button)
    expect(screen.getByText('Qualified')).toBeInTheDocument()
  })

  it('reads as an answer when the season was not played', async () => {
    stub(null, 404)
    renderAt('/tournaments/testville-wta/1999')
    expect(await screen.findByText('No sheet for that season')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See every edition' })).toHaveAttribute('href', '/tournaments/testville-wta')
  })

  it('lists a team competition by tie', async () => {
    stub({
      ...edition,
      link: 'team',
      seeds: [],
      matches: [
        match('RR', 1, ann, bea, { tie: 'Serbia v Spain' }),
        match('RR', 2, cat, dee, { tie: 'Serbia v Spain' }),
      ],
    })
    renderAt('/tournaments/testville-wta/2025')
    expect(await screen.findByRole('heading', { level: 3, name: 'Serbia v Spain' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Replay this draw' })).not.toBeInTheDocument()
  })

  it('writes the edition as a SportsEvent into the document head, without a location', async () => {
    stub(edition)
    renderAt('/tournaments/testville-wta/2025')
    await screen.findByRole('heading', { level: 1, name: 'Testville 2025' })
    const event = (await findJsonLd('SportsEvent')) as Record<string, unknown> & {
      competitor: { name: string }[]
    }
    expect(event).toMatchObject({ name: 'Testville 2025', startDate: '2025-05-01', description: 'WTA, clay, 4 draw' })
    expect(event).not.toHaveProperty('location')
    expect(event.competitor.map((c) => c.name)).toEqual(['Ann Ace', 'Bea Base'])
  })

})
