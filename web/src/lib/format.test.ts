import { describe, expect, it } from 'vitest'
import { ageOn, careerSpan, formatHand, formatScore } from './format'

describe('formatScore', () => {
  it('types scores as the sheet does, with hyphens', () => {
    expect(formatScore('6-4 7-6(3)')).toBe('6-4 7-6(3)')
  })

  it("types the sheet's marks for a retirement and a walkover", () => {
    expect(formatScore('6-4 2-1 RET')).toBe('6-4 2-1 ret.')
    expect(formatScore('W/O')).toBe('w/o')
  })

  it('passes a missing score through as missing', () => {
    expect(formatScore(null)).toBeNull()
  })
})

describe('careerSpan', () => {
  it('reads as a span', () => {
    expect(careerSpan('1973-06-01', '1983-04-02')).toBe('1973-1983')
  })

  it('collapses a career inside one season', () => {
    expect(careerSpan('2019-01-01', '2019-11-02')).toBe('2019')
  })
})

describe('ageOn', () => {
  it('counts whole years', () => {
    expect(ageOn('2003-05-05', '2026-05-04')).toBe(22)
    expect(ageOn('2003-05-05', '2026-05-05')).toBe(23)
  })

  it('returns null rather than a number for an unparseable date', () => {
    expect(ageOn('', '2026-01-01')).toBeNull()
  })
})

describe('formatHand', () => {
  it('spells the hand out', () => {
    expect(formatHand('L')).toBe('left-handed')
  })

  // U is the source saying it does not know, which is not a third way of
  // holding a racquet.
  it('says so when the source did not record one', () => {
    expect(formatHand('U')).toBe('hand not recorded')
    expect(formatHand(null)).toBeNull()
  })
})
