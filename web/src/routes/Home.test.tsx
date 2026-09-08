import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CoverageResponse } from '../api/client'
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

function respondWith(body: unknown) {
  vi.stubGlobal('fetch', () =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    ),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Home', () => {
  it('renders the coverage the API reports', async () => {
    respondWith(coverage)
    render(<Home />)

    expect(await screen.findByText('226694')).toBeInTheDocument()
    expect(screen.getByText('2026-01-17')).toBeInTheDocument()
    expect(screen.getByText('52.3%')).toBeInTheDocument()
  })

  // A tier that never recorded serve statistics has no percentage, and 0.0%
  // would be a claim that it recorded them and they came to nothing.
  it('shows a tier with no statistics as absent, not as 0%', async () => {
    respondWith(coverage)
    render(<Home />)

    await screen.findByText('447000')
    expect(screen.getByLabelText('With serve stats: not recorded')).toBeInTheDocument()
    expect(screen.queryByText('0.0%')).not.toBeInTheDocument()
  })

  it('says what failed and what to do when the API is not there', async () => {
    vi.stubGlobal('fetch', () => Promise.reject(new Error('Failed to fetch')))
    render(<Home />)

    const message = await screen.findByText(/could not be loaded/)
    expect(message).toHaveTextContent('Failed to fetch')
    expect(message).toHaveTextContent('make api')
  })
})
