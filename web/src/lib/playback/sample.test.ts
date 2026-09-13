import { describe, expect, it } from 'vitest'
import { mulberry32 } from './prng'
import { buildMatch, sampleGame, sampleTiebreak, WAIT, type PlaybackEvent } from './sample'

const input = { point: [0.621, 0.606] as [number, number], bestOf: 5 }

/** An rng that hands the point to whoever the script says. */
function scripted(winners: ReadonlyArray<'server' | 'receiver'>): () => number {
  let i = 0
  return () => (winners[i++] === 'server' ? 0 : 0.999)
}

describe('sampleGame', () => {
  it('counts a break point each time the receiver is a point from the game', () => {
    // 0-40 is three break points; the server saves two, then is broken.
    const g = sampleGame(0.6, scripted(['receiver', 'receiver', 'receiver', 'server', 'server', 'receiver']))
    expect(g.hold).toBe(false)
    expect(g.breakPoints).toBe(3)
    expect(g.serverPoints).toBe(2)
    expect(g.receiverPoints).toBe(4)
  })

  it('holds to love without a break point', () => {
    const g = sampleGame(0.6, scripted(['server', 'server', 'server', 'server']))
    expect(g).toEqual({ hold: true, breakPoints: 0, serverPoints: 4, receiverPoints: 0 })
  })
})

describe('sampleTiebreak', () => {
  it('ends at seven with two clear, or two clear beyond it', () => {
    const rng = mulberry32(3)
    for (let i = 0; i < 200; i++) {
      const [a, b] = sampleTiebreak(input.point, i % 2 === 0 ? 0 : 1, rng)
      expect(Math.max(a, b)).toBeGreaterThanOrEqual(7)
      expect(Math.abs(a - b)).toBeGreaterThanOrEqual(2)
      if (Math.max(a, b) > 7) expect(Math.abs(a - b)).toBe(2)
    }
  })
})

describe('buildMatch', () => {
  it('is the same match for the same seed', () => {
    const one = buildMatch(input, mulberry32(1847))
    const two = buildMatch(input, mulberry32(1847))
    expect(one).toEqual(two)
    expect(one).not.toEqual(buildMatch(input, mulberry32(1848)))
  })

  it('always ends with a legal result, in both formats', () => {
    for (const bestOf of [3, 5]) {
      const need = bestOf === 5 ? 3 : 2
      for (let seed = 1; seed <= 200; seed++) {
        const events = buildMatch({ ...input, bestOf }, mulberry32(seed))
        const last = events[events.length - 1] as PlaybackEvent
        expect(last.kind).toBe('match')
        if (last.kind !== 'match') continue
        expect(last.sets[0]).toBe(need)
        expect(last.sets[1]).toBeLessThan(need)
        expect(last.snapshot.current).toBeNull()
        for (const s of last.snapshot.sets) {
          const hi = Math.max(s.a, s.b)
          const lo = Math.min(s.a, s.b)
          expect([6, 7]).toContain(hi)
          if (hi === 6) expect(lo).toBeLessThanOrEqual(4)
          if (hi === 7) expect([5, 6]).toContain(lo)
          // Tiebreak points only where there was one, and only on the loser.
          const played = lo === 6
          expect(s.tiebreakA !== null || s.tiebreakB !== null).toBe(played)
          if (played) expect(s.aWon ? s.tiebreakA : s.tiebreakB).toBeNull()
        }
        expect(last.snapshot.setsWon).toEqual(
          last.side === 0 ? [need, last.sets[1]] : [last.sets[1], need],
        )
      }
    }
  })

  it('tallies holds and break points from the games that were played', () => {
    const events = buildMatch(input, mulberry32(7))
    const last = events[events.length - 1]!
    const holds = events.filter((e) => e.kind === 'hold').length
    const breaks = events.filter((e) => e.kind === 'break').length
    expect(last.snapshot.holds[0] + last.snapshot.holds[1]).toBe(holds)
    expect(last.snapshot.breakPointsWon[0] + last.snapshot.breakPointsWon[1]).toBe(breaks)
    expect(last.snapshot.points[0] + last.snapshot.points[1]).toBeGreaterThan(100)
  })

  it('beats as the prototype does: a break point before its game, a set after its last game', () => {
    const events = buildMatch(input, mulberry32(11))
    events.forEach((event, i) => {
      if (event.kind === 'breakPoint') {
        expect(event.wait).toBe(WAIT.breakPoint)
        expect(['hold', 'break']).toContain(events[i + 1]!.kind)
        if (events[i + 1]!.kind === 'hold') expect(events[i + 1]!.wait).toBe(WAIT.holdAfterBreakPoint)
      }
      if (event.kind === 'set') expect(['hold', 'break', 'tiebreakWon']).toContain(events[i - 1]!.kind)
      if (event.kind === 'tiebreak') expect(events[i + 1]!.kind).toBe('tiebreakWon')
    })
    expect(events.filter((e) => e.kind === 'set').length).toBe(events[events.length - 1]!.snapshot.sets.length)
  })

  it('alternates the serve every game, across sets and through a tiebreak', () => {
    const events = buildMatch(input, mulberry32(5))
    let expected = 0
    for (const event of events) {
      if (event.kind === 'hold' || event.kind === 'break' || event.kind === 'tiebreakWon') {
        expected = expected === 0 ? 1 : 0
        expect(event.snapshot.server).toBe(expected)
      }
    }
  })
})
