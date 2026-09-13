import { serverIsA } from './closedForms'

export type Side = 0 | 1

/** One finished set on the board. */
export interface SetScore {
  a: number
  b: number
  /** The loser's tiebreak points, on the side that lost it. */
  tiebreakA: number | null
  tiebreakB: number | null
  aWon: boolean
}

/** The board and the tracker at one moment of the match. */
export interface Snapshot {
  sets: ReadonlyArray<SetScore>
  /** The set in progress; null between the last set and the result. */
  current: { a: number; b: number } | null
  setsWon: [number, number]
  /** Tallies from the sampled points, never invented. */
  points: [number, number]
  breakPointsWon: [number, number]
  breakPointsFaced: [number, number]
  holds: [number, number]
  /** Who serves next. */
  server: Side
  /** The side that was just broken, for the row to flash. */
  flash: Side | null
  /** The side whose game count just moved, for the figure to pop. */
  pop: Side | null
}

export type PlaybackEvent =
  | { kind: 'breakPoint'; side: Side; snapshot: Snapshot; wait: number }
  | {
      kind: 'hold'
      side: Side
      how: 'love' | 'deuce' | 'front' | 'plain'
      snapshot: Snapshot
      wait: number
    }
  | { kind: 'break'; side: Side; snapshot: Snapshot; wait: number }
  | { kind: 'tiebreak'; snapshot: Snapshot; wait: number }
  | { kind: 'tiebreakWon'; side: Side; points: [number, number]; snapshot: Snapshot; wait: number }
  | { kind: 'set'; side: Side; number: number; snapshot: Snapshot; wait: number }
  | { kind: 'match'; side: Side; sets: [number, number]; snapshot: Snapshot; wait: number }

/** The prototype's beats: what each moment holds for before the next. */
export const WAIT = {
  hold: 480,
  holdAfterBreakPoint: 800,
  breakPoint: 850,
  break: 950,
  tiebreak: 1000,
  tiebreakWon: 1100,
  set: 1300,
  match: 0,
} as const

export interface SampleInput {
  /** Each player's chance of winning a point on their own serve. */
  point: [number, number]
  bestOf: number
}

interface Game {
  hold: boolean
  /** How many break points the receiver held during the game. */
  breakPoints: number
  serverPoints: number
  receiverPoints: number
}

/** One service game, point by point. */
export function sampleGame(p: number, rng: () => number): Game {
  let sp = 0
  let rp = 0
  let breakPoints = 0
  while (!(sp >= 4 && sp - rp >= 2) && !(rp >= 4 && rp - sp >= 2)) {
    if (rp >= 3 && rp > sp) breakPoints++
    if (rng() < p) sp++
    else rp++
  }
  return { hold: sp > rp, breakPoints, serverPoints: sp, receiverPoints: rp }
}

/**
 * A first-to-seven tiebreak, `first` serving the first point and the serve
 * then alternating in pairs. Returns the points as [A, B].
 */
export function sampleTiebreak(
  point: [number, number],
  first: Side,
  rng: () => number,
): [number, number] {
  let a = 0
  let b = 0
  while (!((a >= 7 || b >= 7) && Math.abs(a - b) >= 2) && a + b < 60) {
    const n = a + b
    const serverA = serverIsA(n) === (first === 0)
    const p = serverA ? point[0] : 1 - point[1]
    if (rng() < p) a++
    else b++
  }
  return [a, b]
}

/**
 * One match, game by game, as the prototype builds it: a break-point beat
 * before a game's result, a tiebreak as two beats, a set beat, and the result.
 * Serve alternates every game and carries across sets; A serves first.
 */
export function buildMatch(input: SampleInput, rng: () => number): PlaybackEvent[] {
  const need = input.bestOf === 5 ? 3 : 2
  const events: PlaybackEvent[] = []

  const sets: SetScore[] = []
  let current = { a: 0, b: 0 }
  const setsWon: [number, number] = [0, 0]
  const points: [number, number] = [0, 0]
  const breakPointsWon: [number, number] = [0, 0]
  const breakPointsFaced: [number, number] = [0, 0]
  const holds: [number, number] = [0, 0]
  let server: Side = 0

  const snap = (extra: Partial<Snapshot> = {}): Snapshot => ({
    sets: sets.map((s) => ({ ...s })),
    current: { ...current },
    setsWon: [...setsWon],
    points: [...points],
    breakPointsWon: [...breakPointsWon],
    breakPointsFaced: [...breakPointsFaced],
    holds: [...holds],
    server,
    flash: null,
    pop: null,
    ...extra,
  })

  const other = (side: Side): Side => (side === 0 ? 1 : 0)
  const games = (side: Side) => (side === 0 ? current.a : current.b)
  const setOver = () =>
    (current.a >= 6 && current.a - current.b >= 2) || (current.b >= 6 && current.b - current.a >= 2)

  while (setsWon[0] < need && setsWon[1] < need) {
    current = { a: 0, b: 0 }
    let done = false
    while (!done) {
      if (current.a === 6 && current.b === 6) {
        events.push({ kind: 'tiebreak', snapshot: snap(), wait: WAIT.tiebreak })
        const tb = sampleTiebreak(input.point, server, rng)
        const winner: Side = tb[0] > tb[1] ? 0 : 1
        points[0] += tb[0]
        points[1] += tb[1]
        let tiebreakA: number | null = null
        let tiebreakB: number | null = null
        if (winner === 0) {
          current.a = 7
          tiebreakB = tb[1]
        } else {
          current.b = 7
          tiebreakA = tb[0]
        }
        // The tiebreak counts as one game for the serve.
        server = other(server)
        sets.push({ a: current.a, b: current.b, tiebreakA, tiebreakB, aWon: winner === 0 })
        setsWon[winner]++
        events.push({
          kind: 'tiebreakWon',
          side: winner,
          points: tb,
          snapshot: snap({ pop: winner, current: null }),
          wait: WAIT.tiebreakWon,
        })
        done = true
        continue
      }

      const receiver = other(server)
      const before = games(server) - games(receiver)
      const g = sampleGame(input.point[server], rng)
      points[server] += g.serverPoints
      points[receiver] += g.receiverPoints
      let afterBreakPoint = false
      if (g.breakPoints > 0) {
        breakPointsFaced[receiver] += g.breakPoints
        afterBreakPoint = true
        events.push({ kind: 'breakPoint', side: receiver, snapshot: snap(), wait: WAIT.breakPoint })
      }

      if (g.hold) {
        if (server === 0) current.a++
        else current.b++
        holds[server]++
        // To love, from deuce, or to stay in front when the server already
        // led the set: the phrase follows the game, never a dice roll.
        const how =
          g.receiverPoints === 0
            ? 'love'
            : g.receiverPoints >= 3
              ? 'deuce'
              : before >= 1
                ? 'front'
                : 'plain'
        const held = server
        server = other(server)
        events.push({
          kind: 'hold',
          side: held,
          how,
          snapshot: snap({ pop: held }),
          wait: afterBreakPoint ? WAIT.holdAfterBreakPoint : WAIT.hold,
        })
      } else {
        if (receiver === 0) current.a++
        else current.b++
        breakPointsWon[receiver]++
        const broke = receiver
        server = other(server)
        events.push({
          kind: 'break',
          side: broke,
          snapshot: snap({ pop: broke, flash: other(broke) }),
          wait: WAIT.break,
        })
      }

      if (setOver()) {
        const winner: Side = current.a > current.b ? 0 : 1
        sets.push({ a: current.a, b: current.b, tiebreakA: null, tiebreakB: null, aWon: winner === 0 })
        setsWon[winner]++
        done = true
      }
    }

    const last = sets[sets.length - 1]!
    const setWinner: Side = last.aWon ? 0 : 1
    events.push({
      kind: 'set',
      side: setWinner,
      number: sets.length,
      snapshot: snap({ current: null, pop: setWinner }),
      wait: WAIT.set,
    })
  }

  const winner: Side = setsWon[0] > setsWon[1] ? 0 : 1
  events.push({
    kind: 'match',
    side: winner,
    sets: winner === 0 ? [setsWon[0], setsWon[1]] : [setsWon[1], setsWon[0]],
    snapshot: snap({ current: null, pop: winner }),
    wait: WAIT.match,
  })
  return events
}
