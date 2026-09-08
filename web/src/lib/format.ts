/**
 * The source writes scores with hyphens; the design sets them with en-dashes,
 * which is what a score is: a range between two numbers, not a hyphenation.
 * Only digit-hyphen-digit is touched, so a hyphenated name inside a retirement
 * note survives.
 */
export function formatScore(score: string | null): string | null {
  if (score === null) return null
  return score.replace(/(\d)-(\d)/g, '$1\u2013$2')
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
  return from === to ? from : `${from}\u2013${to}`
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
