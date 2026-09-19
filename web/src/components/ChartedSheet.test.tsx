import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ChartedMatch } from '../api/client'
import { ChartedMark, ChartedSheet } from './ChartedSheet'

const figures = (serve: number, aces: number, bpFaced = 4) => ({
  serve_points: serve,
  aces,
  double_faults: 1,
  first_in: Math.round(serve * 0.6),
  first_won: Math.round(serve * 0.45),
  second_in: Math.round(serve * 0.3),
  second_won: Math.round(serve * 0.15),
  bp_faced: bpFaced,
  bp_saved: Math.min(3, bpFaced),
  return_points: serve,
  return_points_won: Math.round(serve * 0.4),
  winners: 12,
  winners_fh: 8,
  winners_bh: 4,
  unforced: 15,
  unforced_fh: 9,
  unforced_bh: 6,
})

const charted: ChartedMatch = {
  charting_id: '20250505-M-Open-F-Cha_Aaa-Cha_Bbb',
  played_on: '2025-05-05',
  charted_by: 'stard54',
  tournament: 'Open',
  event_slug: 'open-atp',
  season: 2025,
  round: 'F',
  score: '6-4 6-3',
  players: [
    { slug: 'cha-aaa', name: 'Cha Aaa' },
    { slug: 'cha-bbb', name: 'Cha Bbb' },
  ],
  sets: [
    { set: 0, lines: [figures(80, 6), figures(70, 2)] },
    { set: 1, lines: [figures(40, 4, 0), figures(35, 1)] },
    { set: 2, lines: [figures(40, 2), figures(35, 1)] },
  ],
}

function stub(body: unknown, status = 200) {
  vi.stubGlobal('fetch', () =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
    ),
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('ChartedSheet', () => {
  it('lays the match out a column per set with both players in every cell', async () => {
    stub(charted)
    render(
      <MemoryRouter>
        <ChartedSheet chartingId={charted.charting_id} />
      </MemoryRouter>,
    )
    const table = await screen.findByRole('table')
    const heads = within(table)
      .getAllByRole('columnheader')
      .map((th) => th.textContent)
    expect(heads).toEqual(['Per set', 'Set 1', 'Set 2', 'Match'])

    const aces = within(table).getByRole('row', { name: /^Aces/ })
    const cells = within(aces)
      .getAllByRole('cell')
      .map((td) => td.textContent?.replace(/\s+/g, ' ').trim())
    expect(cells).toEqual(['4 1', '2 1', '6 2'])

    // A rate with no denominator in the set shows its counts, not a rate and
    // not an absence: nobody failed to record it, there was nothing to record.
    const bp = within(table).getByRole('row', { name: /^Break points saved/ })
    expect(within(bp).getAllByRole('cell')[0]?.textContent).toMatch(/0\/0/)
    expect(within(table).getByRole('row', { name: /^First serves in/ }).textContent).toMatch(/60%/)
  })

  it('names both players in the head, the asked-for player first, and links them', async () => {
    stub(charted)
    render(
      <MemoryRouter>
        <ChartedSheet chartingId={charted.charting_id} first="cha-bbb" />
      </MemoryRouter>,
    )
    await screen.findByRole('table')
    const links = screen.getAllByRole('link')
    expect(links.map((a) => a.textContent)).toEqual(['Cha Bbb', 'Cha Aaa'])
    expect(links[0]).toHaveAttribute('href', '/players/cha-bbb')
    // And the figures follow the swap: B's 70 serve points come first.
    const serve = screen.getByRole('row', { name: /^Serve points/ })
    expect(within(serve).getAllByRole('cell')[2]?.textContent?.replace(/\s+/g, ' ').trim()).toBe(
      '70 80',
    )
  })

  it('says what charting is and who did it, once, under the sheet', async () => {
    stub(charted)
    render(
      <MemoryRouter>
        <ChartedSheet chartingId={charted.charting_id} />
      </MemoryRouter>,
    )
    await screen.findByRole('table')
    const note = screen.getByText(/Charted point by point by a volunteer/)
    expect(note.textContent).toContain('Match Charting Project')
    expect(note.textContent).toContain('stard54')
    expect(note.textContent).toContain('in no career total')
    expect(screen.getAllByText(/Match Charting Project/)).toHaveLength(1)
  })

  it('reports a sheet that could not be loaded rather than an empty one', async () => {
    stub({ type: '/problems/not-found', title: 'Not found', status: 404 }, 404)
    render(
      <MemoryRouter>
        <ChartedSheet chartingId="nothing" />
      </MemoryRouter>,
    )
    expect(await screen.findByText(/could not be loaded/)).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
  })
})

describe('ChartedMark', () => {
  it('is a typed word that reports whether its sheet is open', async () => {
    const onToggle = vi.fn()
    render(<ChartedMark open={false} onToggle={onToggle} />)
    const mark = screen.getByRole('button', { name: 'charted' })
    expect(mark).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(mark)
    expect(onToggle).toHaveBeenCalledOnce()
  })
})
