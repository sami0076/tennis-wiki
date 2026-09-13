import { describe, expect, it } from 'vitest'
import { mulberry32 } from './prng'
import { announced, commentary } from './narrate'
import { buildMatch, type PlaybackEvent, type Snapshot } from './sample'

const names = ['Alcaraz', 'Sinner'] as const
const snapshot = {} as Snapshot

describe('commentary', () => {
  it('says what happened, in the prototype words', () => {
    const cases: Array<[PlaybackEvent, string]> = [
      [{ kind: 'breakPoint', side: 1, snapshot, wait: 0 }, 'Break point, Sinner'],
      [{ kind: 'hold', side: 0, how: 'love', snapshot, wait: 0 }, 'Alcaraz holds comfortably'],
      [{ kind: 'hold', side: 0, how: 'deuce', snapshot, wait: 0 }, 'Alcaraz holds from deuce'],
      [{ kind: 'hold', side: 1, how: 'front', snapshot, wait: 0 }, 'Sinner holds to stay in front'],
      [{ kind: 'hold', side: 1, how: 'plain', snapshot, wait: 0 }, 'Sinner holds serve'],
      [{ kind: 'break', side: 1, snapshot, wait: 0 }, 'Sinner breaks'],
      [{ kind: 'tiebreak', snapshot, wait: 0 }, 'Tiebreak at 6-6'],
      [{ kind: 'tiebreakWon', side: 1, points: [5, 7], snapshot, wait: 0 }, 'Sinner takes the tiebreak 7-5'],
      [{ kind: 'set', side: 0, number: 2, snapshot, wait: 0 }, 'Set 2 to Alcaraz'],
      [{ kind: 'match', side: 0, sets: [3, 1], snapshot, wait: 0 }, 'Game, set, match: Alcaraz wins 3-1'],
    ]
    for (const [event, text] of cases) expect(commentary(event, names)).toBe(text)
  })

  // Typed like the rest of the sheet: hyphens, no dashes, no exclamation.
  it('keeps every line typed', () => {
    for (let seed = 1; seed <= 50; seed++) {
      for (const event of buildMatch({ point: [0.621, 0.606], bestOf: 3 }, mulberry32(seed))) {
        const line = commentary(event, names)
        expect(line).not.toMatch(/[\u2013\u2014!\u2192]/)
        expect(line).toMatch(/^[A-Z]/)
      }
    }
  })

  it('announces sets and the result, not every game', () => {
    expect(announced({ kind: 'set', side: 0, number: 1, snapshot, wait: 0 })).toBe(true)
    expect(announced({ kind: 'match', side: 0, sets: [2, 0], snapshot, wait: 0 })).toBe(true)
    expect(announced({ kind: 'hold', side: 0, how: 'plain', snapshot, wait: 0 })).toBe(false)
  })
})
