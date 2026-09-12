import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CoverageResponse, DrawSimulation, RankingPage, Trajectories } from '../api/client'
import { Home } from './Home'

const coverage: CoverageResponse = {
  current_through: { atp: '2026-01-17', wta: '2021-12-27' },
  tiers: [
    {
      tour: 'atp',
      tier: 'tour',
      matches: 226694,
      first_match: '1967-12-28',
      last_match: '2026-01-11',
      matches_with_stats: 118557,
      stats_percentage: 52.3,
    },
    {
      tour: 'atp',
      tier: 'futures',
      matches: 447000,
      first_match: '1991-01-07',
      last_match: '2025-12-29',
      // No Futures match has ever recorded serve statistics.
      matches_with_stats: 0,
      stats_percentage: 0,
    },
  ],
}

const trajectories: Trajectories = {
  surface: 'overall',
  tour: null,
  from: '2024-01-08',
  to: '2026-01-12',
  lines: [
    {
      slug: 'jannik-sinner',
      name: 'Jannik Sinner',
      position: 1,
      points: [
        { elo: 2600, as_of: '2024-01-08' },
        { elo: 2751.9, as_of: '2026-01-12' },
      ],
    },
    {
      slug: 'carlos-alcaraz',
      name: 'Carlos Alcaraz',
      position: 2,
      points: [
        { elo: 2570, as_of: '2024-01-08' },
        { elo: 2623.3, as_of: '2026-01-12' },
      ],
    },
  ],
}

const rankings: RankingPage = {
  type: 'elo',
  surface: 'overall',
  tour: null,
  as_of: '2026-01-12',
  requested: null,
  data: [
    {
      position: 1,
      slug: 'jannik-sinner',
      name: 'Jannik Sinner',
      tour: 'atp',
      country: 'ITA',
      elo: 2751.9,
      peak_elo: 2751.9,
      official_rank: 1,
      points: 11830,
      matches: 474,
      age: 24,
      delta: 0,
    },
    {
      position: 2,
      slug: 'novak-djokovic',
      name: 'Novak Djokovic',
      tour: 'atp',
      country: 'SRB',
      elo: 2592.3,
      peak_elo: 2803.7,
      official_rank: 7,
      points: 3910,
      matches: 1416,
      age: 38,
      delta: 5,
    },
  ],
  next_cursor: null,
}

const draw: DrawSimulation = {
  event: {
    name: 'Wimbledon',
    season: 2019,
    tour: 'atp',
    tier: 'tour',
    surface: 'grass',
    ratings_as_of: '2019-07-01',
  },
  rounds: ['R128', 'R64', 'R32', 'R16', 'QF', 'SF', 'F'],
  odds: [
    {
      slug: 'novak-djokovic',
      name: 'Novak Djokovic',
      seed: 1,
      title: 0.401,
      title_interval: 0.01,
      reached: [0.942, 0.917, 0.83, 0.708, 0.64, 0.562, 0.401],
      rating: 2385.9,
    },
    {
      slug: 'roger-federer',
      name: 'Roger Federer',
      seed: 2,
      title: 0.277,
      title_interval: 0.009,
      reached: [0.938, 0.895, 0.821, 0.68, 0.577, 0.457, 0.277],
      rating: 2337.3,
    },
  ],
  runs: 10000,
  seed: 1,
  inputs: {
    source: 'elo',
    anchor: 0.63,
    anchor_scope: 'tour',
    anchor_points: 100000,
    surface: 'grass',
    tier: 'tour',
    decade: 2010,
  } as DrawSimulation['inputs'],
  entered: 128,
  champion: 'novak-djokovic',
}

function stub(lines: Trajectories = trajectories) {
  vi.stubGlobal('fetch', (input: string) => {
    const path = new URL(String(input), 'http://localhost').pathname
    let body: unknown = coverage
    if (path.endsWith('/trajectory')) body = lines
    else if (path.endsWith('/rankings')) body = rankings
    else if (path.endsWith('/simulate/draw')) body = draw
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
}

function renderHome() {
  return render(
    <MemoryRouter>
      <Home />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Home', () => {
  it('renders the coverage the API reports', async () => {
    stub()
    renderHome()

    expect(await screen.findByText('226694')).toBeInTheDocument()
    expect(screen.getByText('2026-01-17')).toBeInTheDocument()
    expect(screen.getByText('52.3%')).toBeInTheDocument()
  })

  // A tier that never recorded serve statistics has no percentage, and 0.0%
  // would be a claim that it recorded them and they came to nothing.
  it('shows a tier with no statistics as absent, not as 0%', async () => {
    stub()
    renderHome()

    await screen.findByText('447000')
    expect(screen.getByLabelText('With serve stats: not recorded')).toBeInTheDocument()
    expect(screen.queryByText('0.0%')).not.toBeInTheDocument()
  })

  it('draws the leaders on one shared scale', async () => {
    stub()
    renderHome()

    const chart = await screen.findByRole('img')
    expect(chart).toHaveAttribute('aria-label', expect.stringContaining('Jannik Sinner'))
  })

  // The strip is a way into the rankings, not an ornament, so every row is a
  // link and the list itself leads somewhere.
  it('leads from the leader strip into the full rankings', async () => {
    stub()
    renderHome()

    // Djokovic is on the sheet twice, as a seed and as the 2019 champion, and
    // both lead to the same page.
    for (const link of await screen.findAllByRole('link', { name: 'Novak Djokovic' })) {
      expect(link).toHaveAttribute('href', '/players/novak-djokovic')
    }
    expect(screen.getByRole('link', { name: 'See the full rankings' })).toHaveAttribute(
      'href',
      '/rankings',
    )
    // Elo is as of a week that happened, and the page says which.
    expect(screen.getAllByText(/as of 2026-01-12/).length).toBeGreaterThan(0)
  })

  // The replayed draw is read against what happened: the champion is marked.
  it('replays a played draw and marks who actually won it', async () => {
    stub()
    renderHome()

    expect(await screen.findByText(/Wimbledon 2019, replayed/)).toBeInTheDocument()
    expect(screen.getByText('won')).toBeInTheDocument()
    expect(screen.getByText('40.1')).toBeInTheDocument()
  })

  // One point is not a line. The rest of the page still has to work.
  it('says so rather than drawing an empty frame when there is nothing to plot', async () => {
    stub({ ...trajectories, lines: [] })
    renderHome()

    expect(await screen.findByText(/enough rated weeks in this window to draw/)).toBeInTheDocument()
    expect(await screen.findByText('226694')).toBeInTheDocument()
  })

  it('says what failed and what to do when the API is not there', async () => {
    vi.stubGlobal('fetch', () => Promise.reject(new Error('Failed to fetch')))
    renderHome()

    const message = await screen.findByText(/The coverage figures could not be loaded/)
    expect(message).toHaveTextContent('Failed to fetch')
    expect(message).toHaveTextContent('make api')
  })
})
