import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { EditionMatch, EditionSide } from '../api/client'
import { buildBracket, pageSheets, roundGroups } from '../lib/bracket'
import { MatchList, RoundList, RoundStepper } from './RoundList'

function side(slug: string, name: string, seed: number | null = null): EditionSide {
  return { slug, name, country: 'FRA', seed, entry: null, rank: null }
}

function match(round: string, num: number, winner: EditionSide, loser: EditionSide, tie: string | null = null): EditionMatch {
  return {
    round,
    match_num: num,
    qualifying: false,
    best_of: 3,
    tie,
    players: [winner, loser],
    score: '6-3 6-3',
    incomplete: false,
    minutes: null,
    serve: [undefined, undefined],
    charting_id: null,
  }
}

const ann = side('ann', 'Ann Ace', 1)
const bea = side('bea', 'Bea Base')
const cat = side('cat', 'Cat Court')
const dee = side('dee', 'Dee Drop')

describe('RoundStepper', () => {
  it('boxes the current round and steps qualifying apart', () => {
    const onChange = vi.fn()
    render(<RoundStepper rounds={['SF', 'F']} qualifying={['Q1']} value="SF" onChange={onChange} />)
    expect(screen.getByRole('tab', { name: 'SF' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Q1' }))
    expect(onChange).toHaveBeenCalledWith('Q1')
  })
})

describe('RoundList', () => {
  it('writes each match as the winner, the score and the loser, and a bye as a line', () => {
    // Ann had a bye; Cat beat Dee; Ann won the final.
    const matches = [match('SF', 2, cat, dee), match('F', 3, ann, cat)]
    const rounds = ['SF', 'F']
    const pages = pageSheets(buildBracket(matches, rounds), rounds, 'main')
    render(
      <MemoryRouter>
        <RoundList groups={roundGroups(pages, 1)} />
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: 'Cat Court' })).toHaveAttribute('href', '/players/cat')
    expect(screen.getByText('Dee Drop')).toBeInTheDocument()
    expect(screen.getByText('bye')).toBeInTheDocument()
    expect(screen.queryByText('Bea Base')).not.toBeInTheDocument()
  })
})

describe('MatchList', () => {
  it('heads each tie once', () => {
    const matches = [
      match('RR', 1, ann, bea, 'France v Spain'),
      match('RR', 2, cat, dee, 'France v Spain'),
      match('RR', 3, ann, cat, 'Italy v Serbia'),
    ]
    render(
      <MemoryRouter>
        <MatchList matches={matches} groupBy={(m) => m.tie} />
      </MemoryRouter>,
    )
    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual([
      'France v Spain',
      'Italy v Serbia',
    ])
  })
})
