/**
 * The four tiers, as the database spells them, turned into something a reader
 * can put next to a name. "itf" in a meta string is a database value that
 * escaped; "ITF" is the level a player competes at.
 */
export function tierLabel(tier: string | null): string | null {
  switch (tier) {
    case 'tour':
      return 'Tour level'
    case 'challenger':
      return 'Challenger'
    case 'futures':
      return 'Futures'
    case 'itf':
      return 'ITF'
    default:
      return null
  }
}

const levels: Record<string, string> = {
  G: 'Grand Slam',
  M: 'Masters',
  PM: 'Premier Mandatory',
  P: 'Premier',
  I: 'International',
  T1: 'Tier I',
  T2: 'Tier II',
  T3: 'Tier III',
  T4: 'Tier IV',
  T5: 'Tier V',
  F: 'Tour finals',
  O: 'Olympics',
  D: 'Team competition',
  C: 'Challenger',
  S: 'Futures',
}

/**
 * The level a tournament was sanctioned at, in words: the files carry a code
 * per era (G, M, PM, T1, 1000) and the page writes what it meant. A code with
 * no name of its own falls back to the tier, which is always known.
 */
export function levelLabel(level: string, tier: string, tour: string): string {
  const named = levels[level]
  if (named !== undefined) return named
  if (/^(1000|500|250)$/.test(level)) return `${tour.toUpperCase()} ${level}`
  return tierLabel(tier) ?? tier
}
