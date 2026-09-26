import { describe, expect, it } from 'vitest'
import { niceTicks, splitOnGaps, timeTicks } from './axis'

const DAY = 86_400_000
const at = (iso: string) => Date.parse(iso)

describe('niceTicks', () => {
  // The whole point: the grid reads as figures, not as the range over five.
  it('lands on round numbers inside the range', () => {
    expect(niceTicks(2292, 2632)).toEqual([2300, 2400, 2500, 2600])
  })

  it('scales the step to the range', () => {
    expect(niceTicks(0, 1)).toEqual([0, 0.25, 0.5, 0.75, 1])
    expect(niceTicks(1000, 3000)).toEqual([1000, 1500, 2000, 2500, 3000])
  })

  it('does not divide by a span of zero', () => {
    expect(niceTicks(1500, 1500)).toEqual([1500])
  })

  it('leaves no floating-point dust on a tick', () => {
    for (const tick of niceTicks(0.1, 0.9)) {
      expect(String(tick)).not.toMatch(/\d{6,}/)
    }
  })
})

describe('timeTicks', () => {
  it('ticks whole years over a long window', () => {
    const ticks = timeTicks(at('2019-03-01'), at('2026-09-01'))
    expect(ticks.map((t) => t.label)).toEqual(['2020', '2022', '2024', '2026'])
  })

  it('spends no tick on a January behind the window', () => {
    // from is March, so 2019's January is not in range and must not be counted
    // against the four ticks the axis has room for.
    expect(timeTicks(at('2019-03-01'), at('2026-09-01'))).toHaveLength(4)
  })

  // A window that steps over January still has to say which years it covers.
  it('carries the year on the first tick and wherever it turns', () => {
    const ticks = timeTicks(at('2024-09-01'), at('2025-06-01'))
    expect(ticks.map((t) => t.label)).toEqual(['Sep 2024', 'Dec', 'Mar 2025', 'Jun'])
  })

  it('ticks plain dates over a few weeks', () => {
    const ticks = timeTicks(at('2024-03-01'), at('2024-04-01'))
    expect(ticks[0]!.label).toBe('1 Mar')
    expect(ticks).toHaveLength(5)
  })

  it('never puts a tick outside the window it was given', () => {
    const [from, to] = [at('2019-03-01'), at('2026-09-01')]
    for (const tick of timeTicks(from, to)) {
      expect(tick.at).toBeGreaterThanOrEqual(from)
      expect(tick.at).toBeLessThanOrEqual(to)
    }
  })

  it('has nothing to tick on an empty span', () => {
    expect(timeTicks(at('2024-01-01'), at('2024-01-01'))).toEqual([])
  })
})

describe('splitOnGaps', () => {
  const run = (...iso: string[]) => splitOnGaps(iso, at).map((r) => r.length)

  it('keeps an ordinary schedule in one piece', () => {
    // Weeks apart, a few months between appearances, and the off-season: none
    // of it is an absence, and none of it should cut the line.
    expect(
      run('2024-01-01', '2024-01-08', '2024-03-01', '2024-06-01', '2024-11-01', '2025-01-06'),
    ).toEqual([6])
  })

  it('cuts where a player went unrated for half a year', () => {
    // Two weeks of a 2019 season, then the six-year hole the seed fixture has.
    expect(run('2019-01-07', '2019-03-26', '2025-12-29')).toEqual([2, 1])
  })

  it('cuts more than once', () => {
    expect(run('2015-01-05', '2018-12-31', '2019-01-14', '2025-12-29')).toEqual([1, 2, 1])
  })

  it('is exact at the boundary', () => {
    const base = at('2024-01-01')
    const days = (n: number) => new Date(base + n * DAY).toISOString()
    expect(splitOnGaps([days(0), days(180)], at)).toHaveLength(1)
    expect(splitOnGaps([days(0), days(181)], at)).toHaveLength(2)
  })

  // A series sampled once a year is not absent between its points, it is
  // annual. Cutting at every step shattered it into unconnected dots, which is
  // how the component gallery caught this.
  it('leaves a sparsely sampled series alone', () => {
    expect(run('2018-01-01', '2019-01-01', '2020-01-01', '2021-01-01', '2022-01-01')).toEqual([5])
  })

  // ...but the same series with a real hole in it still breaks there.
  it('still cuts a hole that dwarfs even a sparse cadence', () => {
    expect(run('2018-01-01', '2019-01-01', '2020-01-01', '2021-01-01', '2035-01-01')).toEqual([4, 1])
  })

  it('has nothing to split when there is nothing', () => {
    expect(splitOnGaps([], at)).toEqual([])
  })
})
