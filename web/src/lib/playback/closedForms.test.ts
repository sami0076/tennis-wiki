import { describe, expect, it } from 'vitest'
import { game, matchFromSets, scorelines, set, tiebreak, twoPointRace } from './closedForms'
import { mulberry32 } from './prng'

// The fixture the simulator tests use: what the API's own chain says for it.
const pA = 0.621
const pB = 0.606

describe('closed forms', () => {
  // The API rounds the chain to three places, so the port is checked to that.
  it('reproduce the API chain for the fixture', () => {
    expect(Math.abs(game(pA) - 0.777)).toBeLessThan(1e-3)
    expect(Math.abs(game(pB) - 0.748)).toBeLessThan(1e-3)
    expect(Math.abs(set(pA, pB) - 0.552)).toBeLessThan(1e-3)
    expect(Math.abs(matchFromSets(set(pA, pB), 5) - 0.596)).toBeLessThan(1e-3)
  })

  // The Go tests pin this exact fraction for a 60% server.
  it('hold the exact deuce arithmetic', () => {
    expect(game(0.6)).toBeCloseTo(149445 / 203125, 9)
  })

  it('give a coin toss to a race nobody can win', () => {
    expect(twoPointRace(0, 0)).toBe(0.5)
    expect(tiebreak(0.65, 0.65)).toBeCloseTo(0.5, 9)
  })

  // Serving first is worth nothing over a set, so the set is symmetric.
  it('is symmetric in who serves first', () => {
    expect(set(pA, pB)).toBeCloseTo(1 - set(pB, pA), 9)
  })
})

describe('scorelines', () => {
  it('sum to one and read most likely first', () => {
    for (const bestOf of [3, 5]) {
      const rows = scorelines(set(pA, pB), bestOf)
      expect(rows.reduce((sum, r) => sum + r.p, 0)).toBeCloseTo(1, 9)
      for (let i = 1; i < rows.length; i++) expect(rows[i]!.p).toBeLessThanOrEqual(rows[i - 1]!.p)
    }
  })

  it("write A's sets first and mark whose win each is", () => {
    const rows = scorelines(0.6, 3)
    expect(rows.map((r) => r.score).sort()).toEqual(['0-2', '1-2', '2-0', '2-1'])
    expect(rows.find((r) => r.score === '1-2')?.aWins).toBe(false)
    expect(rows.find((r) => r.score === '2-1')?.aWins).toBe(true)
  })
})

describe('mulberry32', () => {
  it('repeats itself for a seed and stays inside [0, 1)', () => {
    const a = mulberry32(1847)
    const b = mulberry32(1847)
    for (let i = 0; i < 1000; i++) {
      const x = a()
      expect(x).toBe(b())
      expect(x).toBeGreaterThanOrEqual(0)
      expect(x).toBeLessThan(1)
    }
  })
})
