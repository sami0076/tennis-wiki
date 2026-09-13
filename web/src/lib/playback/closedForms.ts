/**
 * The scoring system in closed form, ported from internal/simulate/match.go so
 * the played-out match and the scorelines under the chain come from the same
 * arithmetic the API used. A serves the first game and the first tiebreak point.
 */

/**
 * The probability the first player takes a two-point lead, given the chance
 * each of them wins one of the paired points. The shape behind deuce, behind
 * 6-6 in a tiebreak and behind an advantage set.
 */
export function twoPointRace(winBoth: number, loseBoth: number): number {
  const total = winBoth * winBoth + loseBoth * loseBoth
  // Neither side can ever win two in a row, so the race does not end. Half is
  // the only answer symmetry allows.
  if (total === 0) return 0.5
  return (winBoth * winBoth) / total
}

/** The probability a server who wins p of their service points holds. */
export function game(p: number): number {
  const q = 1 - p
  const straight = p * p * p * p * (1 + 4 * q + 10 * q * q)
  const toDeuce = 20 * p * p * p * q * q * q
  return straight + toDeuce * twoPointRace(p, q)
}

/** Who serves point n of a tiebreak, counting from zero: A serves one, then B two, then A two. */
export function serverIsA(n: number): boolean {
  const r = n % 4
  return r === 0 || r === 3
}

/** The probability A wins a first-to-seven tiebreak, A serving the first point. */
export function tiebreak(pA: number, pB: number): number {
  const win: number[][] = Array.from({ length: 7 }, () => new Array<number>(7).fill(0))
  const value = (a: number, b: number): number => {
    if (a === 7) return 1
    if (b === 7) return 0
    return win[a]![b]!
  }
  for (let n = 12; n >= 0; n--) {
    for (let a = 0; a <= 6; a++) {
      const b = n - a
      if (b < 0 || b > 6) continue
      if (a === 6 && b === 6) {
        // Beyond 6-6 the points pair up one serve each, so a two-point race settles it.
        win[a]![b] = twoPointRace(pA * (1 - pB), (1 - pA) * pB)
        continue
      }
      const p = serverIsA(a + b) ? pA : 1 - pB
      win[a]![b] = p * value(a + 1, b) + (1 - p) * value(a, b + 1)
    }
  }
  return win[0]![0]!
}

/** The probability A wins a set, serving the first game, with a tiebreak at 6-6. */
export function set(pA: number, pB: number): number {
  const holdA = game(pA)
  const holdB = game(pB)
  const atSixAll = tiebreak(pA, pB)

  const win: number[][] = Array.from({ length: 7 }, () => new Array<number>(7).fill(0))
  const value = (a: number, b: number): number => {
    // Six games with two clear, or 7-5 after the set went past it.
    if (a === 6 && b <= 4) return 1
    if (b === 6 && a <= 4) return 0
    if (a === 7) return 1
    if (b === 7) return 0
    return win[a]![b]!
  }
  for (let n = 12; n >= 0; n--) {
    for (let a = 0; a <= 6; a++) {
      const b = n - a
      if (b < 0 || b > 6) continue
      if ((a === 6 && b <= 4) || (b === 6 && a <= 4)) continue
      if (a === 6 && b === 6) {
        win[a]![b] = atSixAll
        continue
      }
      // A serves the even-numbered games, having served the first.
      const p = (a + b) % 2 === 1 ? 1 - holdB : holdA
      win[a]![b] = p * value(a + 1, b) + (1 - p) * value(a, b + 1)
    }
  }
  return value(0, 0)
}

/** The probability A wins the match from their chance in a set, sets independent. */
export function matchFromSets(setA: number, bestOf: number): number {
  const lose = 1 - setA
  if (bestOf === 5) {
    return setA ** 3 + 3 * setA ** 3 * lose + 6 * setA ** 3 * lose * lose
  }
  return setA * setA + 2 * setA * setA * lose
}

export interface Scoreline {
  /** Sets, A's first: "3-1" is A winning, "1-3" is B. */
  score: string
  /** The chance of exactly this score. */
  p: number
  aWins: boolean
}

/**
 * The chance of each set score, most likely first. A's sets are written first
 * throughout, as the split above reads, so "2-3" is A losing in five.
 */
export function scorelines(setA: number, bestOf: number): Scoreline[] {
  const s = setA
  const q = 1 - s
  const rows: Scoreline[] =
    bestOf === 5
      ? [
          { score: '3-0', p: s * s * s, aWins: true },
          { score: '3-1', p: 3 * s * s * s * q, aWins: true },
          { score: '3-2', p: 6 * s * s * s * q * q, aWins: true },
          { score: '2-3', p: 6 * q * q * q * s * s, aWins: false },
          { score: '1-3', p: 3 * q * q * q * s, aWins: false },
          { score: '0-3', p: q * q * q, aWins: false },
        ]
      : [
          { score: '2-0', p: s * s, aWins: true },
          { score: '2-1', p: 2 * s * s * q, aWins: true },
          { score: '1-2', p: 2 * q * q * s, aWins: false },
          { score: '0-2', p: q * q, aWins: false },
        ]
  return rows.sort((x, y) => y.p - x.p)
}
