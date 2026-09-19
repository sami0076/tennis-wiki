import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { EditionMatch, EditionSide } from '../api/client'
import { buildBracket, pageSheets } from '../lib/bracket'
import { DrawSheet } from './DrawSheet'

function side(slug: string, name: string, seed: number | null = null, entry: string | null = null): EditionSide {
  return { slug, name, country: 'ESP', seed, entry, rank: null }
}

function match(round: string, num: number, winner: EditionSide, loser: EditionSide, score = '6-4 6-4'): EditionMatch {
  return {
    round,
    match_num: num,
    qualifying: false,
    best_of: 3,
    tie: null,
    players: [winner, loser],
    score,
    incomplete: score.includes('RET'),
    minutes: null,
    serve: [undefined, undefined],
    charting_id: null,
  }
}

const ann = side('ann', 'Ann Ace', 1)
const bea = side('bea', 'Bea Base', 2)
const cat = side('cat', 'Cat Court', null, 'Q')
const dee = side('dee', 'Dee Drop')

// A four draw: Ann beat Dee, Bea beat Cat, Ann won the final.
const matches = [match('SF', 1, ann, dee), match('SF', 2, bea, cat, '6-7(3) 6-1 3-0 RET'), match('F', 3, ann, bea, '7-5 7-5')]
const rounds = ['SF', 'F']

function renderSheet(props: Partial<React.ComponentProps<typeof DrawSheet>> = {}) {
  const [page] = pageSheets(buildBracket(matches, rounds), rounds, 'main')
  return render(
    <MemoryRouter>
      <DrawSheet page={page!} kind="main" {...props} />
    </MemoryRouter>,
  )
}

describe('DrawSheet', () => {
  it('types the seed, the entry and the country on the entrants and the score where it was earned', () => {
    renderSheet()
    expect(screen.getAllByText('Ann Ace')).toHaveLength(3)
    expect(screen.getAllByText('[1]')[0]).toBeInTheDocument()
    expect(screen.getAllByText('(ESP)')).toHaveLength(4)
    // Cat entered as a qualifier, wrote no score, and appears once.
    expect(screen.getByText('Q')).toBeInTheDocument()
    expect(screen.getAllByText('Cat Court')).toHaveLength(1)
    // Bea's retirement is typed the way the log types it, under Bea in the final column.
    expect(screen.getByText('ret.')).toBeInTheDocument()
    // The final's score is written once, under the champion: two sets, one line.
    expect(screen.getAllByText('7-5')).toHaveLength(2)
  })

  it('heads the columns with the rounds and the champion', () => {
    renderSheet()
    expect(screen.getByText('Semifinals')).toBeInTheDocument()
    expect(screen.getByText('Final')).toBeInTheDocument()
    expect(screen.getByText('Champion')).toBeInTheDocument()
  })

  it('links every name to its player', () => {
    renderSheet()
    expect(screen.getAllByRole('link', { name: 'Dee Drop' })[0]).toHaveAttribute('href', '/players/dee')
  })

  it('reports the player under the pointer so the whole page can light the run', () => {
    const onLit = vi.fn()
    renderSheet({ onLit })
    fireEvent.mouseOver(screen.getAllByText('Bea Base')[0]!)
    expect(onLit).toHaveBeenCalledWith('bea')
  })

  it('types a bye where the file has no first-round match', () => {
    const withBye = matches.filter((m) => m.match_num !== 1)
    const [page] = pageSheets(buildBracket(withBye, rounds), rounds, 'main')
    render(
      <MemoryRouter>
        <DrawSheet page={page!} kind="main" />
      </MemoryRouter>,
    )
    expect(screen.getByText('bye')).toBeInTheDocument()
    expect(screen.queryByText('Dee Drop')).not.toBeInTheDocument()
  })
})
