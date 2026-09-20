import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Leaderboard, PlayerSearchResult } from '../api/client'
import { Leaders } from './Leaders'

const stats: Leaderboard['stats'] = [
  { key: 'aces', label: 'Aces', family: 'serve', kind: 'rate', sample: 'service points', ascending: false },
  { key: 'serve_points_won', label: 'Service points won', family: 'serve', kind: 'rate', sample: 'service points', ascending: false },
  { key: 'dominance', label: 'Dominance ratio', family: 'points', kind: 'ratio', sample: 'matches with both serve lines', ascending: false },
  { key: 'matches_won', label: 'Matches won', family: 'score', kind: 'rate', sample: 'matches', ascending: false },
]

const board: Leaderboard = {
  stat: stats[1]!,
  filters: { tour: 'atp', tier: null, surface: 'clay', season: 2019, min_matches: 10, limit: 100 },
  population: { matches: 2140, with_stats: 1760, players: 412, qualified: 58 },
  data: [
    { position: 1, slug: 'rafael-nadal', name: 'Rafael Nadal', tour: 'atp', country: 'ESP', sample: 41, matches: 41, numerator: 1912, denominator: 2680, value: 0.7134 },
    { position: 2, slug: 'dominic-thiem', name: 'Dominic Thiem', tour: 'atp', country: 'AUT', sample: 33, matches: 33, numerator: 1500, denominator: 2200, value: 0.6818 },
  ],
  stats,
  player: null,
}

const nadal: PlayerSearchResult = {
  slug: 'rafael-nadal',
  name: 'Rafael Nadal',
  tour: 'atp',
  country: 'ESP',
  matches: 1300,
  best_tier: 'tour',
  score: 1,
}
const nobody: PlayerSearchResult = { ...nadal, slug: 'joe-nobody', name: 'Joe Nobody', matches: 12 }

let requested: string[] = []

function stub(body: Leaderboard, own: Leaderboard['player'] = null) {
  requested = []
  vi.stubGlobal('fetch', (input: string) => {
    const url = new URL(String(input), 'http://localhost')
    requested.push(url.toString())
    if (url.pathname === '/api/v1/players') {
      return Promise.resolve(new Response(JSON.stringify({ data: [nadal, nobody], next_cursor: null }), { status: 200 }))
    }
    const payload = url.searchParams.get('player') ? { ...body, player: own } : body
    return Promise.resolve(new Response(JSON.stringify(payload), { status: 200 }))
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/leaders" element={<Leaders />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Leaders', () => {
  it('sends every filter from the URL and states the population', async () => {
    stub(board)
    renderAt('/leaders?stat=serve_points_won&tour=atp&surface=clay&season=2019&min=10')
    expect(await screen.findByText(/Of 2,140 matches on the ATP on clay in 2019, 1,760 recorded serve statistics/)).toBeInTheDocument()
    expect(screen.getByText(/58 players clear 10 matches/)).toBeInTheDocument()
    const url = new URL(requested[0] as string)
    expect(url.pathname).toBe('/api/v1/leaders/serve_points_won')
    expect(url.searchParams.get('tour')).toBe('atp')
    expect(url.searchParams.get('surface')).toBe('clay')
    expect(url.searchParams.get('season')).toBe('2019')
    expect(url.searchParams.get('min_matches')).toBe('10')
  })

  it('shows the value in emphasis with its counts and the matches it stands on', async () => {
    stub(board)
    renderAt('/leaders')
    expect(await screen.findByRole('link', { name: 'Rafael Nadal' })).toHaveAttribute('href', '/players/rafael-nadal')
    expect(screen.getByText('71.3%')).toBeInTheDocument()
    expect(screen.getByText('1,912 / 2,680')).toBeInTheDocument()
    expect(screen.getByText('41')).toBeInTheDocument()
    expect(screen.getByText(/Floor of 10/)).toBeInTheDocument()
  })

  it('offers every stat the API lists, grouped by family', async () => {
    stub(board)
    renderAt('/leaders')
    await screen.findByText('71.3%')
    const picker = screen.getByLabelText('Stat') as HTMLSelectElement
    expect(picker.value).toBe('serve_points_won')
    expect(screen.getByRole('option', { name: 'Dominance ratio' })).toBeInTheDocument()
    fireEvent.change(picker, { target: { value: 'aces' } })
    const url = new URL(requested[requested.length - 1] as string)
    expect(url.pathname).toBe('/api/v1/leaders/aces')
  })

  it('says why a sought name is absent: below the floor', async () => {
    stub(board, { position: 0, slug: 'joe-nobody', name: 'Joe Nobody', tour: 'atp', country: 'ESP', sample: 3, matches: 12, numerator: 90, denominator: 150, value: 0.6 })
    renderAt('/leaders')
    await screen.findByText('71.3%')
    const find = screen.getByRole('combobox', { name: /Find a player on this board/ })
    fireEvent.change(find, { target: { value: 'nobody' } })
    fireEvent.mouseDown(await screen.findByRole('option', { name: /Joe Nobody/ }))
    expect(await screen.findByText('Joe Nobody is below the floor')).toBeInTheDocument()
    expect(screen.getByText(/3 matches carrying this figure .* against a floor of 10\. Over those, 60\.0%/)).toBeInTheDocument()
  })

  it('says why a sought name is absent: no figure at all', async () => {
    stub(board, null)
    renderAt('/leaders')
    await screen.findByText('71.3%')
    const find = screen.getByRole('combobox', { name: /Find a player on this board/ })
    fireEvent.change(find, { target: { value: 'nobody' } })
    fireEvent.mouseDown(await screen.findByRole('option', { name: /Joe Nobody/ }))
    expect(await screen.findByText('Joe Nobody is not on this board')).toBeInTheDocument()
    expect(screen.getByText(/That is an absence, not a low number/)).toBeInTheDocument()
  })

  it('reads as an answer when nobody clears the floor', async () => {
    stub({ ...board, data: [], population: { ...board.population, qualified: 0 } })
    renderAt('/leaders?min=500')
    expect(await screen.findByText('Nobody clears 500 matches here')).toBeInTheDocument()
  })
})
