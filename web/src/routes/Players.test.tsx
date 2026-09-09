import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PlayerSearchResult } from '../api/client'
import { Players } from './Players'

const erler: PlayerSearchResult = {
  slug: 'alexander-erler',
  name: 'Alexander Erler',
  tour: 'atp',
  country: 'Austria',
  matches: 31,
  best_tier: 'challenger',
  score: 0.7,
}

const bublik: PlayerSearchResult = {
  slug: 'alexander-bublik',
  name: 'Alexander Bublik',
  tour: 'atp',
  country: 'Kazakhstan',
  matches: 412,
  best_tier: 'tour',
  score: 0.6,
}

/** Answers every search with `rows`, and records the URLs it was asked for. */
function respondWith(rows: PlayerSearchResult[]) {
  const urls: string[] = []
  vi.stubGlobal('fetch', (input: string) => {
    urls.push(String(input))
    return Promise.resolve(
      new Response(JSON.stringify({ data: rows, next_cursor: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
  return urls
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Players />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Players', () => {
  it('searches for the query in the URL', async () => {
    const urls = respondWith([erler, bublik])
    renderAt('/players?q=alexander')

    expect(await screen.findByText('Alexander Erler')).toBeInTheDocument()
    expect(screen.getByText('Alexander Bublik')).toBeInTheDocument()
    expect(urls[0]).toContain('q=alexander')
  })

  // The known limitation the issue names: within a tier the ranking is raw
  // similarity, so the shorter name wins. The match count is what lets a reader
  // see past that, and it has to be on the row.
  it('shows the match count that settles a namesake', async () => {
    respondWith([erler, bublik])
    renderAt('/players?q=alexander')

    await screen.findByText('Alexander Erler')
    expect(screen.getByText(/31 matches/)).toBeInTheDocument()
    expect(screen.getByText(/412 matches/)).toBeInTheDocument()
  })

  it('asks nothing until the query is long enough', async () => {
    const urls = respondWith([])
    renderAt('/players?q=a')

    expect(await screen.findByText(/at least 2 characters/)).toBeInTheDocument()
    expect(urls).toHaveLength(0)
  })

  it('reads as an answer, not an error, when nothing matches', async () => {
    respondWith([])
    renderAt('/players?q=zzzz')

    expect(await screen.findByText('No player matches "zzzz"')).toBeInTheDocument()
    expect(screen.getByText(/surname on its own/)).toBeInTheDocument()
  })

  it('puts the tour filter in the URL so a search can be shared', async () => {
    const urls = respondWith([bublik])
    const user = userEvent.setup()
    renderAt('/players?q=alexander')

    await screen.findByText('Alexander Bublik')
    await user.click(screen.getByRole('button', { name: 'WTA' }))

    await waitFor(() => expect(urls.at(-1)).toContain('tour=wta'))
  })

  it('names what failed and what to do about it', async () => {
    vi.stubGlobal('fetch', () => Promise.reject(new Error('Failed to fetch')))
    renderAt('/players?q=alexander')

    const message = await screen.findByText(/could not run/)
    expect(message).toHaveTextContent('Failed to fetch')
    expect(message).toHaveTextContent('make api')
  })
})
