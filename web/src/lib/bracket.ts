import type { EditionMatch, EditionSide } from '../api/client'

/**
 * The bracket, built back from the results. The file records who beat whom
 * and nothing about which line of the draw anyone was on, so the tree is
 * grown from the final: each name's previous match is looked up in the round
 * before, and its two players become the name's kids. A name with no match in
 * the round before is a bye if that round was the first, and a match the file
 * does not carry otherwise; both keep their place so the columns stay even.
 */

export const MAIN_ROUNDS = ['R128', 'R64', 'R32', 'R16', 'QF', 'SF', 'F'] as const
export const QUALIFYING_ROUNDS = ['Q1', 'Q2', 'Q3', 'Q4'] as const

export const ROUND_WORDS: Record<string, string> = {
  R128: 'Round of 128',
  R64: 'Round of 64',
  R32: 'Round of 32',
  R16: 'Round of 16',
  QF: 'Quarterfinals',
  SF: 'Semifinals',
  F: 'Final',
  RR: 'Round robin',
  BR: 'Bronze medal match',
  ER: 'Early rounds',
  Q1: 'Qualifying, first round',
  Q2: 'Qualifying, second round',
  Q3: 'Qualifying, third round',
  Q4: 'Qualifying, fourth round',
}

const QUALIFYING_HEADS: Record<string, string> = {
  Q1: 'First round',
  Q2: 'Second round',
  Q3: 'Third round',
  Q4: 'Fourth round',
}

export type Void = 'bye' | 'n/r'

/** The two sides of a match, winner first: a Go [2]T arrives as a plain array. */
export function sides(match: EditionMatch): [EditionSide, EditionSide] {
  return match.players as [EditionSide, EditionSide]
}

export interface Slot {
  /** 0 is the entrants' column; k the field of rounds[k]; rounds.length the winner's. */
  col: number
  /** The name on the line, or null for an empty line under a bye or a gap. */
  side: EditionSide | null
  /** The match won to reach this slot. Null in the entrants' column and behind a void. */
  match: EditionMatch | null
  /** Why there is no match behind a name that should have one. */
  void: Void | null
  kids: [Slot, Slot] | null
  parent: Slot | null
}

export interface Rounds {
  /** The rounds the main draw's bracket runs through, first to last. */
  main: string[]
  /** The qualifying rounds, first to last; empty where the file has none. */
  qualifying: string[]
  /** Matches that are not a bracket's: a round robin, a bronze medal match. */
  other: EditionMatch[]
}

/** Which of an edition's rounds make a bracket, and which do not. */
export function splitRounds(matches: ReadonlyArray<EditionMatch>): Rounds {
  const present = new Set(matches.map((m) => m.round))
  const main = MAIN_ROUNDS.filter((r) => present.has(r))
  const qualifying = QUALIFYING_ROUNDS.filter((r) => present.has(r))
  const bracket = new Set<string>([...main, ...qualifying])
  return { main, qualifying, other: matches.filter((m) => !bracket.has(m.round)) }
}

/**
 * The trees behind the winners of the last round: one for a main draw, one
 * per qualifier for a qualifying draw. Every tree is full to the entrants'
 * column, empty lines included.
 */
export function buildBracket(matches: ReadonlyArray<EditionMatch>, rounds: ReadonlyArray<string>): Slot[] {
  if (rounds.length === 0) return []
  const wonIn = rounds.map((r) => {
    const byWinner = new Map<string, EditionMatch>()
    for (const m of matches) if (m.round === r) byWinner.set(sides(m)[0].slug, m)
    return byWinner
  })
  const depth = rounds.length

  function blank(col: number, parent: Slot | null, reason: Void): Slot {
    const slot: Slot = { col, side: null, match: null, void: reason, kids: null, parent }
    if (col > 0) slot.kids = [blank(col - 1, slot, reason), blank(col - 1, slot, reason)]
    return slot
  }

  function slot(col: number, side: EditionSide, parent: Slot | null): Slot {
    const node: Slot = { col, side, match: null, void: null, kids: null, parent }
    if (col === 0) return node
    const match = wonIn[col - 1]?.get(side.slug)
    if (match === undefined) {
      // The file has no first-round match for a player: a bye, which no
      // source records as a row. Deeper than that, a row the file lacks.
      node.void = col === 1 ? 'bye' : 'n/r'
      node.kids = [slot(col - 1, side, node), blank(col - 1, node, node.void)]
      return node
    }
    node.match = match
    const [winner, loser] = sides(match)
    node.kids = [slot(col - 1, winner, node), slot(col - 1, loser, node)]
    // Draw order: the earlier match number is the higher line. A first-round
    // pair has no number to go by, so its winner stays first.
    const [a, b] = node.kids
    if (a.match && b.match && a.match.match_num > b.match.match_num) node.kids = [b, a]
    return node
  }

  const last = rounds[depth - 1] as string
  return matches
    .filter((m) => m.round === last)
    .sort((a, b) => a.match_num - b.match_num)
    .map((m) => slot(depth, sides(m)[0], null))
}

/** Every slot of one column under a root, top line first. */
export function column(root: Slot, col: number): Slot[] {
  if (root.col === col) return [root]
  if (root.kids === null) return []
  return [...column(root.kids[0], col), ...column(root.kids[1], col)]
}

/** Whether the name on this line won the match played from it. */
export function advanced(slot: Slot): boolean {
  if (slot.side === null) return false
  if (slot.parent === null) return true
  return slot.parent.side?.slug === slot.side.slug
}

export interface SheetPage {
  /** Null when the draw is one sheet; otherwise which part of it this is. */
  title: string | null
  roots: Slot[]
  /** The columns this page shows, inclusive. */
  c0: number
  c1: number
  heads: string[]
}

const PAGE_ENTRANTS = 32

/**
 * A sheet page holds 32 entrants and five columns, the way the printed draw is
 * paged: a 128 draw is four quarters and then the last eight; a 64 or 56 draw
 * two halves and then the last four; 32 and under is one sheet. Sections
 * follow the file's match numbers, not a claim about the printed draw's top.
 */
export function pageSheets(
  roots: ReadonlyArray<Slot>,
  rounds: ReadonlyArray<string>,
  kind: 'main' | 'qualifying',
): SheetPage[] {
  const depth = rounds.length
  if (depth === 0 || roots.length === 0) return []
  const head = (c: number): string => {
    if (c === depth) return kind === 'main' ? 'Champion' : 'Qualified'
    const round = rounds[c] as string
    return kind === 'main' ? (ROUND_WORDS[round] ?? round) : (QUALIFYING_HEADS[round] ?? round)
  }
  const heads = (c0: number, c1: number) => Array.from({ length: c1 - c0 + 1 }, (_, i) => head(c0 + i))

  if (depth <= 5) {
    const per = Math.max(1, Math.floor(PAGE_ENTRANTS / 2 ** depth))
    const pages: SheetPage[] = []
    for (let i = 0; i < roots.length; i += per) {
      pages.push({
        title: roots.length > per ? `Section ${i / per + 1}` : null,
        roots: roots.slice(i, i + per),
        c0: 0,
        c1: depth,
        heads: heads(0, depth),
      })
    }
    return pages
  }

  const groups = roots.flatMap((root) => column(root, 5).map((slot) => slot.kids as [Slot, Slot]))
  const names =
    groups.length === 4
      ? ['First quarter', 'Second quarter', 'Third quarter', 'Fourth quarter']
      : groups.length === 2
        ? ['First half', 'Second half']
        : groups.map((_, i) => `Section ${i + 1}`)
  const pages: SheetPage[] = groups.map((group, i) => ({
    title: names[i] as string,
    roots: group,
    c0: 0,
    c1: 4,
    heads: heads(0, 4),
  }))
  pages.push({
    title: groups.length === 4 ? 'Last eight' : groups.length === 2 ? 'Last four' : 'Final rounds',
    roots: [...roots],
    c0: 4,
    c1: depth,
    heads: heads(4, depth),
  })
  return pages
}

export interface RoundGroup {
  title: string | null
  slots: Slot[]
}

/**
 * The slots that played a round, in the sheet's order, grouped by the page
 * they are on so a list can carry the same headings the sheet does.
 */
export function roundGroups(pages: ReadonlyArray<SheetPage>, col: number): RoundGroup[] {
  const parts = pages.filter((page) => page.c0 === 0 && col <= page.c1)
  if (parts.length > 1) {
    return parts.map((page) => ({
      title: page.title,
      slots: page.roots.flatMap((root) => column(root, col)),
    }))
  }
  const top = pages[pages.length - 1]
  if (top === undefined) return []
  return [{ title: null, slots: top.roots.flatMap((root) => column(root, col)) }]
}
