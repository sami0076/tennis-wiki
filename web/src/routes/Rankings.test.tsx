import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { RankingPage, Trajectories } from '../api/client'
import { Rankings } from './Rankings'

const elo: RankingPage = {
  type: 'elo',
  surface: 'overall',
  tour: 'atp',
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
      position: 3,
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
      delta: 4,
    },
  ],
  next_cursor: null,
}

const official: RankingPage = {
  ...elo,
  type: 'official',
  surface: null,
  data: elo.data.map((row) => ({ ...row, delta: null, matches: null })),
}

const trajectories: Trajectories = {
  surface: 'overall',
  tour: 'atp',
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

function stub(page: RankingPage) {
  vi.stubGlobal('fetch', (input: string) => {
    const path = new URL(String(input), 'http://localhost').pathname
    const body = path.endsWith('/trajectory') ? trajectories : page
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
        <Route path="/rankings" element={<Rankings />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Rankings', () => {
  // A ranking is as of the last week that exists, and the page has to say which
  // week that was: the alternative is a reader assuming it is today.
  it('states the week it is a ranking as of', async () => {
    stub(elo)
    renderAt('/rankings')

    expect(await screen.findByText(/as of 2026-01-12/)).toBeInTheDocument()
    // By role: the leader is in the chart legend as well as the table, and the
    // row is the one that has to be a link to the player.
    expect(screen.getByRole('link', { name: 'Jannik Sinner' })).toHaveAttribute(
      'href',
      '/players/jannik-sinner',
    )
  })

  // The filter is deliberate and invisible in the data, so it is on the page
  // rather than only in the handler that applies it.
  it('explains why the retired are missing', async () => {
    stub(elo)
    renderAt('/rankings')

    expect(await screen.findByText(/more than a year old drops out/)).toBeInTheDocument()
  })

  it('puts the model against the published rank', async () => {
    stub(elo)
    renderAt('/rankings')

    expect(await screen.findByText('Against rank')).toBeInTheDocument()
    expect(screen.getByText('+4')).toBeInTheDocument()
  })

  // The tours publish one list each. The API answers 400 for a surface over it,
  // so the page does not offer the combination in the first place.
  it('offers no surface filter over the published list, and says why', async () => {
    stub(official)
    renderAt('/rankings?type=official')

    expect(await screen.findByText(/tours publish one list/)).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Filter by surface' })).not.toBeInTheDocument()
    expect(screen.queryByText('Against rank')).not.toBeInTheDocument()
  })

  // A date outside coverage is an answer with a reason, not a 404 and not an
  // error banner.
  it('reads as an answer when the date is outside coverage', async () => {
    stub({ ...elo, as_of: '1900-01-01', requested: '1900-01-01', data: [], next_cursor: null })
    renderAt('/rankings?date=1900-01-01')

    expect(await screen.findByText('Nobody was ranked that week')).toBeInTheDocument()
    expect(screen.getByText(/does not reach 1900-01-01/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show the latest list' })).toBeInTheDocument()
  })

  it('says when the week it used is not the week that was asked for', async () => {
    stub({ ...elo, as_of: '1969-12-29', requested: '1970-01-01' })
    renderAt('/rankings?date=1970-01-01')

    expect(await screen.findByText(/You asked for 1970-01-01/)).toBeInTheDocument()
    expect(screen.getByText(/as of 1969-12-29/)).toBeInTheDocument()
  })
})
