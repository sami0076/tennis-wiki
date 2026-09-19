import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { EventSummary, Page } from '../api/client'
import { Tournaments } from './Tournaments'

function event(slug: string, name: string, tour: string, category: string, editions = 3): EventSummary {
  return {
    slug,
    name,
    tour,
    category,
    level: category === 'slam' ? 'G' : 'A',
    tier: 'tour',
    first_season: 1968,
    last_season: 2026,
    editions,
  }
}

const index: Page<EventSummary> = {
  data: [
    event('wimbledon-atp', 'Wimbledon', 'atp', 'slam', 58),
    event('wimbledon-wta', 'Wimbledon', 'wta', 'slam', 97),
    event('dallas-atp', 'Dallas', 'atp', 'tour'),
  ],
  next_cursor: 'abc',
}

let requested: string[] = []

function stub(page: Page<EventSummary>) {
  requested = []
  vi.stubGlobal('fetch', (input: string) => {
    requested.push(String(input))
    return Promise.resolve(new Response(JSON.stringify(page), { status: 200 }))
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/tournaments" element={<Tournaments />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Tournaments', () => {
  it('groups the index by level, each name a link to its event', async () => {
    stub(index)
    renderAt('/tournaments')
    expect(await screen.findByText('Grand Slams')).toBeInTheDocument()
    expect(screen.getByText('Tour level')).toBeInTheDocument()
    const links = screen.getAllByRole('link', { name: 'Wimbledon' })
    expect(links[0]).toHaveAttribute('href', '/tournaments/wimbledon-atp')
    expect(links[1]).toHaveAttribute('href', '/tournaments/wimbledon-wta')
    expect(screen.getByText(/97 editions/)).toBeInTheDocument()
  })

  it('sends the search, the tour and the level from the URL', async () => {
    stub(index)
    renderAt('/tournaments?q=wim&tour=wta&level=slam')
    await screen.findByText('Grand Slams')
    const url = new URL(requested[requested.length - 1] as string, 'http://localhost')
    expect(url.searchParams.get('q')).toBe('wim')
    expect(url.searchParams.get('tour')).toBe('wta')
    expect(url.searchParams.get('level')).toBe('slam')
    expect(screen.getByRole('button', { name: 'Grand Slams' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('offers the next page', async () => {
    stub(index)
    renderAt('/tournaments')
    const more = await screen.findByRole('button', { name: 'Show the next 100' })
    fireEvent.click(more)
    expect(await screen.findByRole('button', { name: 'Back to the top of the list' })).toBeInTheDocument()
    const url = new URL(requested[requested.length - 1] as string, 'http://localhost')
    expect(url.searchParams.get('cursor')).toBe('abc')
  })

  it('reads as an answer when nothing matches', async () => {
    stub({ data: [], next_cursor: null })
    renderAt('/tournaments?q=zzz')
    expect(await screen.findByText('No tournament matches "zzz"')).toBeInTheDocument()
  })
})
