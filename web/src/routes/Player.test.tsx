import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  AvailabilityNeverForTier,
  AvailabilityNeverInEra,
  AvailabilityRecorded,
  type PlayerMatch,
  type PlayerProfile,
} from '../api/client'
import { Player } from './Player'

const coverage = {
  current_through: { atp: '2026-01-17', wta: '2021-12-27' },
  tiers: [],
}

function profile(overrides: Partial<PlayerProfile> = {}): PlayerProfile {
  return {
    slug: 'itg-player',
    name: 'Itg Player',
    tour: 'atp',
    country: 'ESP',
    hand: 'R',
    height_cm: 183,
    birth_date: '2003-05-05',
    pro_since: 2018,
    career: {
      matches: 400,
      wins: 300,
      losses: 100,
      win_percentage: 75,
      titles: 20,
      majors: 4,
      incomplete_matches: 6,
      first_match: '2018-04-02',
      last_match: '2025-11-03',
      surfaces: [{ surface: 'clay', matches: 200, wins: 160, losses: 40 }],
      tiers: [{ tier: 'tour', matches: 400, matches_with_stats: 380 }],
    },
    serve: {
      availability: AvailabilityRecorded,
      matches_with_data: 380,
      rates: {
        aces_per_match: 6.4,
        double_faults_per_match: 2.1,
        first_serve_in_percentage: 63.2,
        first_serve_won_percentage: 74.8,
        second_serve_won_percentage: 55.1,
        break_points_saved_percentage: 66,
      },
    },
    ratings: [
      {
        surface: 'overall',
        matches: 400,
        current: { elo: 2168.4, as_of: '2025-11-03' },
        peak: { elo: 2201.9, as_of: '2024-06-10' },
      },
      {
        surface: 'clay',
        matches: 200,
        current: { elo: 2214.2, as_of: '2025-11-03' },
        peak: { elo: 2260.1, as_of: '2024-06-10' },
      },
    ],
    ...overrides,
  }
}

function match(overrides: Partial<PlayerMatch> = {}): PlayerMatch {
  return {
    date: '2025-11-03',
    tournament: 'Paris',
    tier: 'tour',
    level: 'M',
    season: 2025,
    round: 'F',
    qualifying: false,
    surface: 'hard',
    opponent: { slug: 'itg-rival', name: 'Itg Rival' },
    won: true,
    score: '6-4 7-6(3)',
    incomplete: false,
    minutes: 128,
    serve: {
      availability: AvailabilityRecorded,
      aces: 9,
      double_faults: 2,
      serve_points: 80,
      first_in: 50,
      first_won: 40,
      second_won: 18,
      serve_games: 12,
      break_points_saved: 3,
      break_points_faced: 4,
    },
    ...overrides,
  }
}

/** routes stubs fetch by path, so each endpoint can answer differently. */
function routes(handlers: Record<string, unknown>, notFound: string[] = []) {
  vi.stubGlobal('fetch', (input: RequestInfo | URL) => {
    const url = String(input)
    if (notFound.some((path) => url.includes(path))) {
      return Promise.resolve(
        new Response(
          JSON.stringify({ type: '/problems/not-found', title: 'Not found', status: 404 }),
          { status: 404, headers: { 'Content-Type': 'application/json' } },
        ),
      )
    }
    const key = Object.keys(handlers).find((path) => url.includes(path))
    if (key === undefined) return Promise.reject(new Error(`unstubbed ${url}`))
    return Promise.resolve(
      new Response(JSON.stringify(handlers[key]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
}

function show(slug = 'itg-player') {
  render(
    <MemoryRouter initialEntries={[`/players/${slug}`]}>
      <Routes>
        <Route path="/players/:slug" element={<Player />} />
      </Routes>
    </MemoryRouter>,
  )
}

const emptySeries = { surface: 'overall', from: '', to: '', points: [] }
const emptyRankings = { from: '', to: '', best: null, points: [] }

afterEach(() => vi.unstubAllGlobals())

describe('the player page', () => {
  it('renders a tour player with statistics completely', async () => {
    routes({
      '/coverage': coverage,
      '/ratings': emptySeries,
      '/rankings': { from: '2019-01-07', to: '2025-11-03', best: { date: '2023-09-11', rank: 1, points: 9000 }, points: [{ date: '2023-09-11', rank: 1, points: 9000 }] },
      '/matches': { data: [match()], next_cursor: '' },
      '/players/itg-player': profile(),
    })
    show()

    expect(await screen.findByRole('heading', { name: 'Itg Player' })).toBeInTheDocument()
    // Still playing, so the strip leads with the current rating rather than the peak.
    expect(screen.getByText('2168')).toBeInTheDocument()
    expect(screen.queryByText(/, peak/)).not.toBeInTheDocument()
    expect(screen.getByText('300–100')).toBeInTheDocument()
    expect(screen.getByText('4')).toBeInTheDocument()
    expect(screen.getByText('63.2%')).toBeInTheDocument()
    // The score is set with an en-dash, not the hyphen the source stores.
    expect(await screen.findByText(/6–4 7–6\(3\)/)).toBeInTheDocument()
  })

  // The majority case at 125,868 players, and the one normally done badly.
  it('explains a Futures career rather than showing zeroes', async () => {
    routes({
      '/coverage': coverage,
      '/ratings': emptySeries,
      '/rankings': emptyRankings,
      '/matches': { data: [match({ serve: { availability: AvailabilityNeverForTier, aces: null, double_faults: null, serve_points: null, first_in: null, first_won: null, second_won: null, serve_games: null, break_points_saved: null, break_points_faced: null } })], next_cursor: '' },
      '/players/itg-player': profile({
        serve: { availability: AvailabilityNeverForTier, matches_with_data: 0, rates: null },
      }),
    })
    show()

    expect(await screen.findByText(/No Futures or ITF match has ever recorded/)).toBeInTheDocument()
    expect(screen.queryByText('0.0%')).not.toBeInTheDocument()
    // The absent statistic is a dash in the match row, not a blank and not a nought.
    expect(await screen.findByLabelText('Aces: not recorded')).toBeInTheDocument()
  })

  it('explains a pre-1991 career by its era', async () => {
    routes({
      '/coverage': coverage,
      '/ratings': emptySeries,
      '/rankings': emptyRankings,
      '/matches': { data: [], next_cursor: '' },
      '/players/itg-player': profile({
        serve: { availability: AvailabilityNeverInEra, matches_with_data: 0, rates: null },
        career: { ...profile().career!, first_match: '1973-06-01', last_match: '1983-04-02' },
      }),
    })
    show()

    expect(await screen.findByText(/not kept before 1991/i)).toBeInTheDocument()
    // A finished career is described by its span and led by its peak.
    expect(await screen.findByText(/1973–1983/)).toBeInTheDocument()
    expect(screen.getAllByText(/, peak/).length).toBeGreaterThan(0)
    expect(screen.getByText('2202')).toBeInTheDocument()
  })

  it('renders a player with no matches at all without crashing', async () => {
    routes({
      '/coverage': coverage,
      '/ratings': emptySeries,
      '/rankings': emptyRankings,
      '/matches': { data: [], next_cursor: '' },
      '/players/itg-player': profile({
        career: null,
        ratings: null,
        serve: { availability: 'not_recorded', matches_with_data: 0, rates: null },
      }),
    })
    show()

    expect(await screen.findByRole('heading', { name: 'Itg Player' })).toBeInTheDocument()
    expect(await screen.findByText(/No matches in the database/)).toBeInTheDocument()
  })

  it('pages the match list without leaving the page', async () => {
    const first = match({ date: '2025-11-03', opponent: { slug: 'a', name: 'Recent Rival' } })
    const second = match({ date: '2019-05-02', opponent: { slug: 'b', name: 'Older Rival' } })
    let call = 0
    vi.stubGlobal('fetch', (input: RequestInfo | URL) => {
      const url = String(input)
      const body = url.includes('/matches')
        ? call++ === 0
          ? { data: [first], next_cursor: 'CURSOR' }
          : { data: [second], next_cursor: '' }
        : url.includes('/coverage')
          ? coverage
          : url.includes('/ratings')
            ? emptySeries
            : url.includes('/rankings')
              ? emptyRankings
              : profile()
      return Promise.resolve(
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })
    show()

    expect(await screen.findByText('Recent Rival')).toBeInTheDocument()
    await userEvent.click(await screen.findByRole('button', { name: 'Show earlier matches' }))
    expect(await screen.findByText('Older Rival')).toBeInTheDocument()
    // Still the same page: the identity header never went away.
    expect(screen.getByRole('heading', { name: 'Itg Player' })).toBeInTheDocument()
  })

  // An address that does not exist gets an explanation, not a blank screen.
  it('says so when there is no such player', async () => {
    routes({ '/coverage': coverage }, ['/players/'])
    show('itg-nobody')

    expect(await screen.findByText(/No player has that address/)).toBeInTheDocument()
    expect(screen.getByText(/itg-nobody/)).toBeInTheDocument()
  })

  it('shows a skeleton rather than a blank screen while loading', () => {
    vi.stubGlobal('fetch', () => new Promise(() => {}))
    const { container } = render(
      <MemoryRouter initialEntries={['/players/itg-player']}>
        <Routes>
          <Route path="/players/:slug" element={<Player />} />
        </Routes>
      </MemoryRouter>,
    )
    expect(within(container).getAllByRole('generic').length).toBeGreaterThan(0)
    expect(container.textContent).toBe('')
  })
})
