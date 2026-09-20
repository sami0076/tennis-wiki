import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Event as EventData } from '../api/client'
import { Event } from './Event'

const dallas: EventData = {
  slug: 'dallas-atp',
  name: 'Dallas',
  tour: 'atp',
  keyed: 'number',
  number: '424',
  first_season: 2021,
  last_season: 2023,
  names: [
    { name: 'San Jose', first_season: 2021, last_season: 2021 },
    { name: 'Dallas', first_season: 2022, last_season: 2023 },
  ],
  provenance: { number: 2, override: 0, bridged: 1, name: 0, team: 0 },
  editions: [
    {
      season: 2021,
      name: 'San Jose',
      level: 'A',
      tier: 'tour',
      surface: 'hard',
      draw_size: 32,
      start_date: '2021-02-08',
      link: 'bridged',
      champion: { slug: 'ann', name: 'Ann Ace' },
      finalist: { slug: 'bea', name: 'Bea Base' },
      final_score: '6-4 6-4',
      matches: 31,
      ties: 0,
    },
    {
      season: 2022,
      name: 'Dallas',
      level: 'A',
      tier: 'tour',
      surface: 'hard',
      draw_size: 28,
      start_date: '2022-02-07',
      link: 'number',
      champion: { slug: 'cat', name: 'Cat Court' },
      finalist: { slug: 'ann', name: 'Ann Ace' },
      final_score: '7-6(4) 7-6(6)',
      matches: 27,
      ties: 0,
    },
    {
      season: 2023,
      name: 'Dallas',
      level: 'A',
      tier: 'tour',
      surface: 'hard',
      draw_size: 28,
      start_date: '2023-02-06',
      link: 'number',
      champion: null,
      finalist: null,
      final_score: null,
      matches: 26,
      ties: 0,
    },
  ],
}

function stub(body: EventData | null) {
  vi.stubGlobal('fetch', (input: string) => {
    const path = new URL(String(input), 'http://localhost').pathname
    const payload =
      body === null
        ? { type: '/problems/not-found', title: 'Not found', status: 404, detail: 'No tournament has that slug.', instance: path, request_id: 'x' }
        : body
    return Promise.resolve(new Response(JSON.stringify(payload), { status: body === null ? 404 : 200 }))
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/tournaments/:slug" element={<Event />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Event', () => {
  it('lists every edition, most recent first, each a link to its sheet', async () => {
    stub(dallas)
    renderAt('/tournaments/dallas-atp')
    expect(await screen.findByRole('heading', { level: 1, name: 'Dallas' })).toBeInTheDocument()
    const seasons = screen.getAllByRole('link', { name: /^20\d\d$/ })
    expect(seasons.map((link) => link.textContent)).toEqual(['2023', '2022', '2021'])
    expect(seasons[0]).toHaveAttribute('href', '/tournaments/dallas-atp/2023')
    expect(screen.getByRole('link', { name: 'Cat Court' })).toHaveAttribute('href', '/players/cat')
  })

  it('prints how each edition got onto the page', async () => {
    stub(dallas)
    renderAt('/tournaments/dallas-atp')
    expect(await screen.findByText(/2 by the ATP's number 424; 1 by name, bridged to that number/)).toBeInTheDocument()
    expect(screen.getByText(/Played as/)).toHaveTextContent('San Jose 2021, Dallas 2022-2023')
  })

  it('types a final the file does not carry as n/r', async () => {
    stub(dallas)
    renderAt('/tournaments/dallas-atp')
    await screen.findByRole('heading', { level: 1 })
    expect(screen.getAllByText('n/r').length).toBeGreaterThanOrEqual(3)
  })

  it('reads as an answer for an unknown slug', async () => {
    stub(null)
    renderAt('/tournaments/nowhere-atp')
    expect(await screen.findByText('No tournament has that address')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See every tournament' })).toHaveAttribute('href', '/tournaments')
  })
})
