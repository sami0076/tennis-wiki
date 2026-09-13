import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DrawSimulation, MatchSimulation, PlayerSearchResult } from '../api/client'
import { Simulator } from './Simulator'

const alcaraz: PlayerSearchResult = {
  slug: 'carlos-alcaraz',
  name: 'Carlos Alcaraz',
  tour: 'atp',
  country: 'ESP',
  matches: 400,
  best_tier: 'tour',
  score: 0.9,
}

const chain: MatchSimulation = {
  players: [
    { slug: 'a', name: 'Carlos Alcaraz', tour: 'atp', country: 'ESP', elo: 2571, surface_weight: 0.75, surface_matches: 193 },
    { slug: 'b', name: 'Jannik Sinner', tour: 'atp', country: 'ITA', elo: 2503, surface_weight: 0.75, surface_matches: 129 },
  ],
  best_of: 5,
  surface: 'clay',
  chain: {
    point: [0.621, 0.606],
    hold: [0.777, 0.748],
    set: [0.552, 0.448],
    match: [0.596, 0.404],
  },
  inputs: {
    source: 'elo_derived',
    anchor: 0.613,
    anchor_scope: 'tier_surface_decade',
    anchor_points: 932159,
    surface: 'clay',
    tier: 'tour',
    decade: 2020,
    expected: 0.596,
    achieved: 0.596,
  },
  availability: 'elo_derived',
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
    { slug: 'novak-djokovic', name: 'Novak Djokovic', seed: 1, title: 0.401, title_interval: 0.01, reached: [], rating: 2386 },
    { slug: 'roger-federer', name: 'Roger Federer', seed: 2, title: 0.277, title_interval: 0.009, reached: [], rating: 2337 },
  ],
  runs: 10000,
  seed: 1,
  inputs: {
    source: 'elo_derived',
    anchor: 0.656,
    anchor_scope: 'tier_surface_decade',
    anchor_points: 733212,
    surface: 'grass',
    tier: 'tour',
    decade: 2010,
    expected: null,
    achieved: null,
  },
  entered: 128,
  champion: 'novak-djokovic',
}

function stub(match: MatchSimulation | null, drawSim: DrawSimulation = draw) {
  vi.stubGlobal('fetch', (input: string) => {
    const path = new URL(String(input), 'http://localhost').pathname
    let body: unknown = match
    if (path.endsWith('/simulate/draw')) body = drawSim
    if (path.endsWith('/players')) body = { data: [alcaraz], next_cursor: null }
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/simulator" element={<Simulator />} />
      </Routes>
    </MemoryRouter>,
  )
}

// The result's count-up and the played-out match answer the reader's own
// motion preference; these tests read the finished figures, so they ask for less.
beforeEach(() => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: true,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Simulator', () => {
  it('shows every rung of the chain, not just the answer', async () => {
    stub(chain)
    renderAt('/simulator?a=carlos-alcaraz&b=jannik-sinner&surface=clay&best_of=5')

    expect(await screen.findByText('How the edge compounds')).toBeInTheDocument()
    for (const rung of ['Point, on serve', 'Hold', 'Set', 'Match']) {
      expect(screen.getByText(rung)).toBeInTheDocument()
    }
    // The headline is the last rung, so it appears twice: once as the answer
    // and once as the top of the ladder that produced it.
    expect(screen.getAllByText(/59\.6%/)).toHaveLength(2)
    expect(screen.getByText(/77\.7%/)).toBeInTheDocument()
  })

  // The sentence the page exists for, carrying its own numbers rather than the
  // design's.
  it('says how far the edge was amplified', async () => {
    stub(chain)
    renderAt('/simulator?a=a&b=b')

    const caption = await screen.findByText(/Tennis scoring is an amplifier/)
    expect(caption).toHaveTextContent('2-point edge on serve')
    expect(caption).toHaveTextContent('19-point edge on the match')
  })

  // ADR-0007: a simulation that will not say where its numbers came from is a
  // number pretending to be a fact.
  it('names its own inputs', async () => {
    stub(chain)
    renderAt('/simulator?a=a&b=b')

    const inputs = await screen.findByText(/derived from the ratings rather than measured/)
    expect(inputs).toHaveTextContent('2571')
    expect(inputs).toHaveTextContent('2503')
    expect(inputs).toHaveTextContent('61.3%')
    expect(inputs).toHaveTextContent('tour level')
  })

  it('reads as an answer when the pair cannot be simulated', async () => {
    stub({
      ...chain,
      players: [chain.players[0], { ...chain.players[1], elo: null }],
      chain: null,
      availability: 'unrated',
    })
    renderAt('/simulator?a=a&b=b')

    expect(await screen.findByText('This pair cannot be simulated')).toBeInTheDocument()
    expect(screen.getByText(/Jannik Sinner has no rating/)).toBeInTheDocument()
    expect(screen.queryByText('How the edge compounds')).not.toBeInTheDocument()
  })

  // One player picked is half a simulation, and the box has to keep the name
  // in the meantime: the response names neither side until both are chosen.
  it('keeps showing a player picked before the other one is', async () => {
    stub(null)
    const user = userEvent.setup()
    renderAt('/simulator')

    const box = screen.getByRole('combobox', { name: 'First player' })
    await user.type(box, 'alcaraz')
    await user.click(await screen.findByText('Carlos Alcaraz'))

    expect(box).toHaveValue('Carlos Alcaraz')
  })

  // A cleared box has to stay clear. Falling back to the name the moment the
  // box is empty makes it impossible to delete one.
  it('lets a picked name be deleted', async () => {
    stub(null)
    const user = userEvent.setup()
    renderAt('/simulator')

    const box = screen.getByRole('combobox', { name: 'First player' })
    await user.type(box, 'alcaraz')
    await user.click(await screen.findByText('Carlos Alcaraz'))
    expect(box).toHaveValue('Carlos Alcaraz')

    await user.clear(box)
    expect(box).toHaveValue('')
    await user.type(box, 'sin')
    expect(box).toHaveValue('sin')
  })

  it('asks for two players before simulating anything', async () => {
    stub(null)
    renderAt('/simulator')

    expect(await screen.findByText('Pick two players')).toBeInTheDocument()
    expect(screen.queryByText('How the edge compounds')).not.toBeInTheDocument()
  })

  // Every draw figure is one sample of ten thousand, so none of them appears
  // without the interval it earned.
  it('shows the draw odds with their intervals', async () => {
    stub(null)
    renderAt('/simulator')

    expect(await screen.findByText('Draw simulator')).toBeInTheDocument()
    expect(screen.getByLabelText(/Novak Djokovic: 40\.1%, give or take 1\.0/)).toBeInTheDocument()
    expect(screen.getByText(/±0\.9/)).toBeInTheDocument()
    // The event, the field size and the run count are named.
    const meta = screen.getByText(/Wimbledon 2019/)
    expect(meta).toHaveTextContent('128 draw')
    expect(meta).toHaveTextContent('10,000 runs')
  })

  // A draw simulation is always as of the week it began, and the page says so
  // rather than letting a reader assume today's ratings.
  it('says which week the draw ratings are from', async () => {
    stub(null)
    renderAt('/simulator')

    const caption = await screen.findByText(/This draw was played/)
    expect(caption).toHaveTextContent('2019-07-01')
  })
})
