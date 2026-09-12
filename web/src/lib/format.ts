/**
 * A score is typed the way the source writes it and the way a draw sheet
 * writes it: 6-4 7-6(3). The marks are the sheet's: ret. for a retirement,
 * w/o for a walkover, def. for a default.
 */
export function formatScore(score: string | null): string | null {
  if (score === null) return null
  return score
    .replace(/\bRET\b/g, 'ret.')
    .replace(/\bW\/O\b/g, 'w/o')
    .replace(/\bDEF\b/g, 'def.')
}

/**
 * Whether a score already carries the sheet's mark for not being played out, so
 * an incomplete match is not marked twice.
 */
export function scoreCarriesMark(score: string | null): boolean {
  return score !== null && /\b(ret\.|w\/o|def\.)/.test(formatScore(score) ?? '')
}

/** elo is shown whole: the hundredths in the database are not a real precision. */
export function formatElo(elo: number): string {
  return String(Math.round(elo))
}

export function formatPercent(value: number, places = 1): string {
  return `${value.toFixed(places)}%`
}

/**
 * A career is described by an age while it is running and by its span once it
 * is over. "23" and "1973-1983" answer the same question about two players.
 */
export function careerSpan(firstMatch: string, lastMatch: string): string {
  const from = firstMatch.slice(0, 4)
  const to = lastMatch.slice(0, 4)
  return from === to ? from : `${from}-${to}`
}

/** surname is what a sheet writes where a whole name will not fit. */
export function surname(name: string): string {
  const parts = name.trim().split(' ')
  return parts[parts.length - 1] ?? name
}

/** ageOn is whole years between two dates, which is how an age is quoted. */
export function ageOn(birthDate: string, on: string): number | null {
  const born = new Date(birthDate)
  const at = new Date(on)
  if (Number.isNaN(born.getTime()) || Number.isNaN(at.getTime())) return null

  let age = at.getUTCFullYear() - born.getUTCFullYear()
  const monthDiff = at.getUTCMonth() - born.getUTCMonth()
  if (monthDiff < 0 || (monthDiff === 0 && at.getUTCDate() < born.getUTCDate())) age -= 1
  return age
}

const hands: Record<string, string> = {
  R: 'right-handed',
  L: 'left-handed',
  U: 'hand not recorded',
}

export function formatHand(hand: string | null): string | null {
  if (hand === null) return null
  return hands[hand] ?? null
}
