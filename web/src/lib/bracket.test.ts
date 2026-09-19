import { describe, expect, it } from 'vitest'
import type { EditionMatch, EditionSide } from '../api/client'
import {
  advanced,
  buildBracket,
  column,
  pageSheets,
  roundGroups,
  sides,
  splitRounds,
  MAIN_ROUNDS,
} from './bracket'

function side(n: number): EditionSide {
  return { slug: `p${n}`, name: `Player ${n}`, country: 'SRB', seed: null, entry: null, rank: null }
}

function match(round: string, num: number, winner: number, loser: number): EditionMatch {
  return {
    round,
    match_num: num,
    qualifying: round.startsWith('Q'),
    best_of: 3,
    tie: null,
    players: [side(winner), side(loser)],
    score: '6-4 6-4',
    incomplete: false,
    minutes: null,
    serve: [undefined, undefined],
    charting_id: null,
  }
}

/**
 * A complete draw of `size` players in which the lower number always wins, so
 * p1 is the champion and every line is predictable. Match numbers run through
 * the draw in order, the way the files number them.
 */
function draw(size: number, rounds: ReadonlyArray<string>): EditionMatch[] {
  const out: EditionMatch[] = []
  let players = Array.from({ length: size }, (_, i) => i + 1)
  let num = 1
  for (const round of rounds) {
    const next: number[] = []
    for (let i = 0; i < players.length; i += 2) {
      const a = players[i] as number
      const b = players[i + 1] as number
      out.push(match(round, num++, Math.min(a, b), Math.max(a, b)))
      next.push(Math.min(a, b))
    }
    players = next
  }
  return out
}

describe('buildBracket', () => {
  it('grows the tree back from the final, in match-number order', () => {
    const matches = draw(4, ['SF', 'F'])
    const [root] = buildBracket(matches, ['SF', 'F'])
    expect(root?.side?.slug).toBe('p1')
    expect(root?.match?.round).toBe('F')
    // The two semifinal winners, the earlier match on the higher line.
    expect(root?.kids?.map((k) => k.side?.slug)).toEqual(['p1', 'p3'])
    // The entrants, winner first within a pair: the file has no draw lines.
    expect(column(root!, 0).map((s) => s.side?.slug)).toEqual(['p1', 'p2', 'p3', 'p4'])
  })

  it('reverses a pair whose match numbers run the other way', () => {
    const matches = draw(4, ['SF', 'F'])
    // p3 beat p4 in match 1 and p1 beat p2 in match 2: p3's half is the top.
    matches[0]!.match_num = 2
    matches[1]!.match_num = 1
    const [root] = buildBracket(matches, ['SF', 'F'])
    expect(column(root!, 0).map((s) => s.side?.slug)).toEqual(['p3', 'p4', 'p1', 'p2'])
  })

  it('marks who advanced from each line', () => {
    const [root] = buildBracket(draw(4, ['SF', 'F']), ['SF', 'F'])
    const entrants = column(root!, 0)
    expect(entrants.map(advanced)).toEqual([true, false, true, false])
    expect(advanced(root!)).toBe(true)
  })

  it('types a bye under a seed with no first-round match', () => {
    // An 8 draw where p1 sat out the first round: three first-round matches.
    const matches = draw(8, ['QF', 'SF', 'F']).filter((m) => !(m.round === 'QF' && sides(m)[0].slug === 'p1'))
    const [root] = buildBracket(matches, ['QF', 'SF', 'F'])
    const entrants = column(root!, 0)
    expect(entrants).toHaveLength(8)
    expect(entrants[0]?.side?.slug).toBe('p1')
    expect(entrants[1]?.side).toBeNull()
    expect(entrants[1]?.void).toBe('bye')
    // The seed's own line in the next column has no score behind it.
    expect(column(root!, 1)[0]?.match).toBeNull()
    expect(column(root!, 1)[0]?.void).toBe('bye')
  })

  it('types n/r under a match the file does not carry, keeping the columns even', () => {
    const matches = draw(8, ['QF', 'SF', 'F']).filter((m) => !(m.round === 'SF' && sides(m)[0].slug === 'p1'))
    const [root] = buildBracket(matches, ['QF', 'SF', 'F'])
    expect(column(root!, 2)[0]?.void).toBe('n/r')
    expect(column(root!, 0)).toHaveLength(8)
    expect(column(root!, 0).filter((s) => s.void === 'n/r')).toHaveLength(2)
  })

  it('builds one tree per qualifier', () => {
    const matches = [...draw(4, ['Q1', 'Q2']), ...draw(4, ['Q1', 'Q2']).map((m) => ({ ...m, match_num: m.match_num + 10 }))]
    const roots = buildBracket(matches, ['Q1', 'Q2'])
    expect(roots).toHaveLength(2)
  })
})

describe('pageSheets', () => {
  it('pages a 128 draw as four quarters and the last eight', () => {
    const roots = buildBracket(draw(128, MAIN_ROUNDS), MAIN_ROUNDS)
    const pages = pageSheets(roots, MAIN_ROUNDS, 'main')
    expect(pages.map((p) => p.title)).toEqual([
      'First quarter',
      'Second quarter',
      'Third quarter',
      'Fourth quarter',
      'Last eight',
    ])
    expect(pages[0]?.heads).toEqual(['Round of 128', 'Round of 64', 'Round of 32', 'Round of 16', 'Quarterfinals'])
    expect(pages[0]?.roots.flatMap((r) => column(r, 0))).toHaveLength(32)
    expect(pages[4]?.heads).toEqual(['Quarterfinals', 'Semifinals', 'Final', 'Champion'])
    expect(pages[4]?.c0).toBe(4)
  })

  it('pages a 64 draw as two halves and the last four', () => {
    const rounds = MAIN_ROUNDS.slice(1)
    const pages = pageSheets(buildBracket(draw(64, rounds), rounds), rounds, 'main')
    expect(pages.map((p) => p.title)).toEqual(['First half', 'Second half', 'Last four'])
  })

  it('keeps a 32 draw on one sheet with six columns', () => {
    const rounds = MAIN_ROUNDS.slice(2)
    const pages = pageSheets(buildBracket(draw(32, rounds), rounds), rounds, 'main')
    expect(pages).toHaveLength(1)
    expect(pages[0]?.title).toBeNull()
    expect(pages[0]?.heads).toEqual(['Round of 32', 'Round of 16', 'Quarterfinals', 'Semifinals', 'Final', 'Champion'])
  })

  it('groups qualifiers 32 entrants to a sheet', () => {
    const rounds = ['Q1', 'Q2', 'Q3']
    const matches: EditionMatch[] = []
    for (let q = 0; q < 16; q++) {
      for (const m of draw(8, rounds)) {
        const [winner, loser] = sides(m)
        matches.push({
          ...m,
          match_num: m.match_num + q * 10,
          players: [
            { ...winner, slug: `q${q}-${winner.slug}` },
            { ...loser, slug: `q${q}-${loser.slug}` },
          ],
        })
      }
    }
    const pages = pageSheets(buildBracket(matches, rounds), rounds, 'qualifying')
    expect(pages.map((p) => p.title)).toEqual(['Section 1', 'Section 2', 'Section 3', 'Section 4'])
    expect(pages[0]?.heads).toEqual(['First round', 'Second round', 'Third round', 'Qualified'])
  })
})

describe('roundGroups', () => {
  it('carries the page headings into the early rounds and drops them later', () => {
    const roots = buildBracket(draw(128, MAIN_ROUNDS), MAIN_ROUNDS)
    const pages = pageSheets(roots, MAIN_ROUNDS, 'main')
    const first = roundGroups(pages, 1)
    expect(first.map((g) => g.title)).toEqual(['First quarter', 'Second quarter', 'Third quarter', 'Fourth quarter'])
    expect(first[0]?.slots).toHaveLength(16)
    const semis = roundGroups(pages, 6)
    expect(semis).toHaveLength(1)
    expect(semis[0]?.slots.map((s) => s.match?.round)).toEqual(['SF', 'SF'])
  })
})

describe('splitRounds', () => {
  it('keeps a round robin out of the bracket', () => {
    const matches = [...draw(4, ['SF', 'F']), match('RR', 50, 1, 2), match('BR', 60, 3, 4)]
    const rounds = splitRounds(matches)
    expect(rounds.main).toEqual(['SF', 'F'])
    expect(rounds.qualifying).toEqual([])
    expect(rounds.other.map((m) => m.round)).toEqual(['RR', 'BR'])
  })
})
