import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { Snapshot } from '../lib/playback'
import { Scoreboard } from './Scoreboard'

const midMatch: Snapshot = {
  sets: [{ a: 7, b: 6, tiebreakA: null, tiebreakB: 5, aWon: true }],
  current: { a: 3, b: 4 },
  setsWon: [1, 0],
  points: [61, 58],
  breakPointsWon: [1, 2],
  breakPointsFaced: [3, 4],
  holds: [9, 8],
  server: 1,
  flash: null,
  pop: 1,
}

describe('Scoreboard', () => {
  it('writes the tiebreak points as a superscript on the loser and marks the server', () => {
    render(<Scoreboard snapshot={midMatch} names={['Alcaraz', 'Sinner']} surface="clay" playing />)
    const rows = screen.getAllByRole('row')
    // Header, then one row per player.
    expect(rows).toHaveLength(3)
    expect(rows[2]).toHaveTextContent('Sinner serving')
    expect(rows[1]).not.toHaveTextContent('serving')
    const sup = rows[2]!.querySelector('sup')
    expect(sup).toHaveTextContent('5')
    expect(rows[1]!.querySelector('sup')).toBeNull()
    // One finished set and the set in progress.
    expect(screen.getAllByRole('columnheader')).toHaveLength(2)
  })

  it('shows no set in progress once the match is over', () => {
    render(
      <Scoreboard
        snapshot={{ ...midMatch, current: null, server: 0 }}
        names={['Alcaraz', 'Sinner']}
        surface="clay"
        playing={false}
      />,
    )
    expect(screen.getAllByRole('columnheader')).toHaveLength(1)
    expect(screen.queryByText('serving')).not.toBeInTheDocument()
  })
})
