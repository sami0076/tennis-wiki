import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import type { ThisWeek as ThisWeekData } from '../api/client'
import { ThisWeek } from './ThisWeek'

const week: ThisWeekData = {
  checked_at: '2026-09-26T16:00:00Z',
  changed_at: '2026-09-26T15:20:00Z',
  events: [
    {
      tour: 'atp',
      name: 'Chengdu',
      level: '250',
      surface: 'hard',
      round: 'QF',
      matches: 24,
      last_played: '2026-09-26',
      champion: null,
      latest: [
        {
          date: '2026-09-26',
          round: 'QF',
          winner: { name: 'Lloyd Harris', slug: 'lloyd-harris', seed: null },
          loser: { name: 'Unknown Qualifier', slug: null, seed: 3 },
          score: '6-4 7-6(5)',
        },
      ],
    },
  ],
}

const now = new Date('2026-09-26T16:00:00Z')

function renderWeek(data: ThisWeekData) {
  return render(
    <MemoryRouter>
      <ThisWeek week={{ state: 'ready', data, error: null }} now={now} />
    </MemoryRouter>,
  )
}

describe('ThisWeek', () => {
  it('says it is results so far, and how old', () => {
    renderWeek(week)
    expect(screen.getByText(/Results so far, not live scores: updated 40 minutes ago/)).toBeInTheDocument()
  })

  it('writes how far each event has got and links the players it knows', () => {
    renderWeek(week)
    expect(screen.getByText('Quarterfinals under way')).toBeInTheDocument()
    expect(screen.getByText('ATP 250')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Lloyd Harris' })).toHaveAttribute('href', '/players/lloyd-harris')
    expect(screen.queryByRole('link', { name: 'Unknown Qualifier' })).toBeNull()
  })

  it('names the champion once the final is in', () => {
    const decided = week.events[0]!
    renderWeek({
      ...week,
      events: [{ ...decided, round: 'F', champion: { name: 'Lloyd Harris', slug: 'lloyd-harris', seed: null } }],
    })
    expect(screen.getByText(/Champion/)).toBeInTheDocument()
  })

  it('says so when nothing is in yet', () => {
    renderWeek({ ...week, events: [] })
    expect(screen.getByText(/No tour-level event has a result in yet/)).toBeInTheDocument()
  })
})

