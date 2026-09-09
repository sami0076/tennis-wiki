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
