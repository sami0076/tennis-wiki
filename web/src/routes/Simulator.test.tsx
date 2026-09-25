import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DrawSimulation, MatchSimulation, PlayerSearchResult, ReplayableDraw } from '../api/client'
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
    slug: 'wimbledon-atp',
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
  byes: 0,
  champion: 'novak-djokovic',
}

// The draws the picker offers. Two editions of two events, which is enough
// for the list to have groups and for a choice to have somewhere to go. Named
// individually because the tests reorder them to prove the page reads the
// list's order rather than happening to agree with it.
const wimbledon2019: ReplayableDraw = {
  slug: 'wimbledon-atp',
  name: 'Wimbledon',
  season: 2019,
  tour: 'atp',
  tier: 'tour',
  level: 'G',
  surface: 'grass',
  draw_size: 128,
  matches: 127,
  start_date: '2019-07-01',
}

const roland2019: ReplayableDraw = {
  slug: 'roland-garros-atp',
  name: 'Roland Garros',
  season: 2019,
  tour: 'atp',
  tier: 'tour',
  level: 'G',
  surface: 'clay',
  draw_size: 128,
  matches: 127,
  start_date: '2019-05-26',
}

const wimbledon2015: ReplayableDraw = { ...wimbledon2019, season: 2015, start_date: '2015-06-29' }

const replayable: ReplayableDraw[] = [wimbledon2019, roland2019, wimbledon2015]

let requests: string[] = []

function stub(
  match: MatchSimulation | null,
  options: {
    draw?: DrawSimulation | { status: number; body: unknown }
    /** A list, or 'error' for an API that does not serve one. */
    draws?: ReplayableDraw[] | 'error'
  } = {},
) {
  requests = []
  const drawSim = options.draw ?? draw
  const drawList = options.draws ?? replayable
  vi.stubGlobal('fetch', (input: string) => {
    requests.push(String(input))
    const path = new URL(String(input), 'http://localhost').pathname
    let body: unknown = match
    let status = 200
    // Checked before the single draw: the list is what the picker reads, and
    // the page asks for both on every render.
    if (path.endsWith('/simulate/draws')) {
      if (drawList === 'error') {
        // What a deployed frontend meets when the API is an older build.
        status = 404
        body = { type: '/problems/not-found', title: 'Not found', status: 404, detail: 'no route', instance: path, request_id: 'x' }
      } else {
        body = { data: drawList, filters: { tour: null, season: null, limit: 120 } }
      }
    } else if (path.endsWith('/simulate/draw')) {
      if ('status' in drawSim) {
        status = drawSim.status
        body = drawSim.body
      } else {
        body = drawSim
      }
    }
    if (path.endsWith('/players')) body = { data: [alcaraz], next_cursor: null }
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
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

    const caption = await screen.findByText(/edge on serve becomes/)
    expect(caption).toHaveTextContent('2-point edge on serve')
    expect(caption).toHaveTextContent('19-point edge on the match')
  })

  // ADR-0007: a simulation that will not say where its numbers came from is a
  // number pretending to be a fact.
  it('names its own inputs', async () => {
    stub(chain)
    renderAt('/simulator?a=a&b=b')

    const inputs = await screen.findByText(/Derived from the ratings/)
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

  it('offers one simulated match under the chain, and plays it out when asked', async () => {
    stub(chain)
    const user = userEvent.setup()
    renderAt('/simulator?a=carlos-alcaraz&b=jannik-sinner')

    const watch = await screen.findByRole('button', { name: 'Watch a simulated match' })
    expect(screen.getByText('How it ends')).toBeInTheDocument()
    // Reduced motion in these tests, so the whole match lands at once.
    await user.click(watch)
    expect(
      screen.getAllByText(
        (_, element) =>
          element?.tagName === 'P' &&
          /^Game, set, match: (Alcaraz|Sinner) wins 3-[012]$/.test(element.textContent ?? ''),
      ).length,
    ).toBeGreaterThan(0)
    expect(screen.getByText('Points won')).toBeInTheDocument()
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
    // The event, linking to its sheet, then the field size and the run count.
    const sheet = screen.getByRole('link', { name: 'Wimbledon 2019' })
    expect(sheet).toHaveAttribute('href', '/tournaments/wimbledon-atp/2019')
    const meta = sheet.closest('p')
    expect(meta).toHaveTextContent('128 draw')
    expect(meta).not.toHaveTextContent('byes')
    expect(meta).toHaveTextContent('10,000 runs')
  })

  // A 28 draw is a tree of 32 with four byes, and the line says so.
  it('counts the byes a draw had', async () => {
    stub(null, { draw: { ...draw, entered: 28, byes: 4 } })
    renderAt('/simulator')

    const sheet = await screen.findByRole('link', { name: 'Wimbledon 2019' })
    expect(sheet.closest('p')).toHaveTextContent('28 draw')
    expect(sheet.closest('p')).toHaveTextContent('4 byes')
  })

  // The edition page's "Replay this draw" lands here with the sheet's address.
  it('replays the draw the URL names', async () => {
    stub(null)
    renderAt('/simulator?event=testville-wta&season=2025')
    await screen.findByText('Draw simulator')
    const asked = requests.find((url) => url.includes('/simulate/draw?'))
    expect(asked).toContain('event=testville-wta')
    expect(asked).toContain('season=2025')
  })

  // A round robin is a draw the endpoint declines with a reason, and the
  // reason is the answer, with the sheet as the way on.
  it('reads a declined draw as an answer', async () => {
    stub(null, {
      draw: {
        status: 422,
        body: { type: '/problems/bad-request', title: 'Invalid request', status: 422, detail: 'That draw is a round robin.', instance: '/api/v1/simulate/draw', request_id: 'x' },
      },
    })
    renderAt('/simulator?event=monte-carlo-masters-atp&season=2023')
    expect(await screen.findByText('This draw cannot be replayed')).toBeInTheDocument()
    expect(screen.getByText('That draw is a round robin.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See the sheet instead' })).toHaveAttribute(
      'href',
      '/tournaments/monte-carlo-masters-atp/2023',
    )
  })

  // A draw simulation is always as of the week it began, and the page says so
  // rather than letting a reader assume today's ratings.
  it('says which week the draw ratings are from', async () => {
    stub(null)
    renderAt('/simulator')

    const caption = await screen.findByText(/This draw was played/)
    expect(caption).toHaveTextContent('2019-07-01')
  })

  // The page used to replay the featured draw and nothing else: the only way
  // to another one was typing a URL nothing on screen mentioned.
  it('replays another draw when one is picked', async () => {
    const user = userEvent.setup()
    stub(null)
    renderAt('/simulator')
    await screen.findByText('Draw simulator')

    await user.selectOptions(
      screen.getByRole('combobox', { name: 'Draw' }),
      'roland-garros-atp\u00002019',
    )

    const asked = requests.filter((url) => url.includes('/simulate/draw?')).at(-1)
    expect(asked).toContain('event=roland-garros-atp')
    // Both halves of the address, or the page asks for a season of nothing.
    expect(asked).toContain('season=2019')
  })

  // The page opened on a hardcoded Wimbledon 2019 whatever the database held.
  // The top of the list is newest and biggest first, so it follows the data.
  it('opens on the first draw of the list, not a constant', async () => {
    stub(null)
    renderAt('/simulator')
    await screen.findByText('Draw simulator')

    const asked = requests.find((url) => url.includes('/simulate/draw?'))
    // replayable[0] is Wimbledon 2019 here, so the fixture is reordered to
    // prove the page reads the list rather than happening to agree with it.
    expect(asked).toContain('event=wimbledon-atp')
  })

  it('opens on whatever the list puts first', async () => {
    stub(null, { draws: [roland2019, wimbledon2019] })
    renderAt('/simulator')
    await screen.findByText('Draw simulator')

    const asked = requests.find((url) => url.includes('/simulate/draw?'))
    expect(asked).toContain('event=roland-garros-atp')
  })

  // Waiting for the list costs milliseconds against a simulation that costs
  // hundreds; simulating a guess first would spend a real request on a draw
  // nobody asked for and then replace it on screen.
  it('does not simulate a guess before the list arrives', async () => {
    stub(null, { draws: [roland2019] })
    renderAt('/simulator')
    await screen.findByText('Draw simulator')

    const drawCalls = requests.filter((url) => url.includes('/simulate/draw?'))
    expect(drawCalls).toHaveLength(1)
    expect(drawCalls[0]).toContain('event=roland-garros-atp')
  })

  // A URL that names a draw is the reader's choice and does not wait on, or
  // get overridden by, the list.
  it('lets the URL outrank the list', async () => {
    stub(null)
    renderAt('/simulator?event=testville-wta&season=2025')
    await screen.findByText('Draw simulator')

    const drawCalls = requests.filter((url) => url.includes('/simulate/draw?'))
    expect(drawCalls).toHaveLength(1)
    expect(drawCalls[0]).toContain('event=testville-wta')
  })

  // The list is what names the default draw, so when it fails the page still
  // has to show one -- and has to admit the picker is not usable.
  it('still shows a draw, and says why it cannot be changed, when the list fails', async () => {
    stub(null, { draws: 'error' })
    renderAt('/simulator')
    await screen.findByText('Draw simulator')

    expect(await screen.findByRole('link', { name: 'Wimbledon 2019' })).toBeInTheDocument()
    expect(screen.getByText(/The list of draws could not be loaded/)).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Draw' })).toBeDisabled()
  })

  // Being unable to replay this draw is the moment a reader most wants a
  // different one, so the picker outlives the panel's answer.
  it('still offers the other draws when this one is declined', async () => {
    stub(null, {
      draw: {
        status: 422,
        body: { type: '/problems/bad-request', title: 'Invalid request', status: 422, detail: 'That draw is a round robin.', instance: '/api/v1/simulate/draw', request_id: 'x' },
      },
    })
    renderAt('/simulator?event=monte-carlo-masters-atp&season=2023')

    await screen.findByText('This draw cannot be replayed')
    expect(screen.getByRole('combobox', { name: 'Draw' })).toBeInTheDocument()
  })
})
