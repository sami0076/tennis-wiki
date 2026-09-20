import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { SeasonEventsResponse, SeasonsResponse } from '../api/client'
import { Season } from './Season'
import { Seasons } from './Seasons'

const calendar: SeasonsResponse = {
  current_through: { atp: '2026-09-07', wta: '2026-08-30' },
  data: [
    {
      season: 2026,
      atp: {
        events: 229,
        ties: 26,
        surfaces: { hard: 102, clay: 114, grass: 13, carpet: 0, unknown: 0 },
        slams: [
          { slug: 'australian-open-atp', name: 'Australian Open', start_date: '2026-01-12', champion: { slug: 'jannik-sinner', name: 'Jannik Sinner' }, finalist: null, final_score: '6-4 6-4 6-4' },
          { slug: 'wimbledon-atp', name: 'Wimbledon', start_date: '2026-06-29', champion: null, finalist: null, final_score: null },
        ],
        partial: true,
      },
      wta: {
        events: 39,
        ties: 7,
        surfaces: { hard: 22, clay: 12, grass: 5, carpet: 0, unknown: 0 },
        slams: [],
        partial: true,
      },
    },
    {
      season: 1950,
      atp: null,
      wta: {
        events: 40,
        ties: 0,
        surfaces: { hard: 0, clay: 10, grass: 25, carpet: 0, unknown: 5 },
        slams: [{ slug: 'wimbledon-wta', name: 'Wimbledon', start_date: '1950-06-26', champion: { slug: 'louise-brough', name: 'Louise Brough' }, finalist: null, final_score: '6-1 3-6 6-1' }],
        partial: false,
      },
    },
  ],
}

const year: SeasonEventsResponse = {
  season: 2026,
  tour: null,
  tier: 'tour',
  partial: { atp: '2026-09-07', wta: '2026-08-30' },
  events: [
    { slug: 'australian-open-atp', name: 'Australian Open', tour: 'atp', category: 'slam', level: 'G', tier: 'tour', surface: 'hard', draw_size: 128, start_date: '2026-01-12', champion: { slug: 'jannik-sinner', name: 'Jannik Sinner' }, finalist: { slug: 'carlos-alcaraz', name: 'Carlos Alcaraz' }, final_score: '6-4 6-4 6-4', matches: 127, ties: 0 },
    { slug: 'davis-cup', name: 'Davis Cup', tour: 'atp', category: 'team', level: 'D', tier: 'tour', surface: null, draw_size: null, start_date: '2026-02-01', champion: null, finalist: null, final_score: null, matches: 80, ties: 26 },
    { slug: 'doha-atp', name: 'Doha', tour: 'atp', category: 'tour', level: 'A', tier: 'tour', surface: 'hard', draw_size: 32, start_date: '2026-02-16', champion: null, finalist: null, final_score: null, matches: 31, ties: 0 },
  ],
}

let requested: string[] = []

function stub(body: unknown) {
  requested = []
  vi.stubGlobal('fetch', (input: string) => {
    requested.push(String(input))
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/seasons" element={<Seasons />} />
        <Route path="/seasons/:year" element={<Season />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Seasons', () => {
  it('is a row per year with both tours, the women before 1968 included', async () => {
    stub(calendar)
    renderAt('/seasons')
    expect(await screen.findByRole('link', { name: '2026' })).toHaveAttribute('href', '/seasons/2026')
    expect(screen.getByRole('link', { name: '1950' })).toBeInTheDocument()
    expect(screen.getByText(/no ATP file for 1950/)).toBeInTheDocument()
    // The surface counts, and the Slam champion linked by surname with the full name on hover.
    expect(screen.getByText('114')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Sinner' })).toHaveAttribute('href', '/players/jannik-sinner')
    expect(screen.getByRole('link', { name: 'Sinner' })).toHaveAttribute('title', 'Jannik Sinner')
    expect(screen.getByRole('link', { name: 'AO' })).toHaveAttribute('href', '/tournaments/australian-open-atp/2026')
    // A Slam without a final in the file is n/r, not a missing name.
    expect(screen.getByText('n/r')).toBeInTheDocument()
  })

  it('marks the season in progress with the date each tour is complete to', async () => {
    stub(calendar)
    renderAt('/seasons')
    expect(await screen.findByText(/in progress, through 2026-09-07/)).toBeInTheDocument()
    expect(screen.getByText(/in progress, through 2026-08-30/)).toBeInTheDocument()
  })
})

describe('Season', () => {
  it('groups the year by level, each event a link to its sheet, and says it is in progress', async () => {
    stub(year)
    renderAt('/seasons/2026')
    expect(await screen.findByRole('heading', { level: 1, name: '2026' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Grand Slams' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Team competitions' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Australian Open' })).toHaveAttribute('href', '/tournaments/australian-open-atp/2026')
    expect(screen.getByText(/complete the ATP's through 2026-09-07 and the WTA's through 2026-08-30/)).toBeInTheDocument()
    // A final the file does not carry is n/r in the table.
    expect(screen.getAllByText('n/r').length).toBeGreaterThan(0)
  })

  it('sends the tour and tier from the URL', async () => {
    stub({ ...year, tour: 'wta', tier: 'itf', events: [] })
    renderAt('/seasons/2026?tour=wta&tier=itf')
    expect(await screen.findByText(/Nothing at ITF level in 2026/)).toBeInTheDocument()
    const url = new URL(requested[requested.length - 1] as string, 'http://localhost')
    expect(url.searchParams.get('tour')).toBe('wta')
    expect(url.searchParams.get('tier')).toBe('itf')
    expect(screen.getByRole('button', { name: 'ITF' })).toHaveAttribute('aria-pressed', 'true')
  })
})
