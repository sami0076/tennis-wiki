/**
 * The draw the site opens on. There is no upcoming tournament in this database
 * and never will be, so it is a played one, and one whose answer a reader can
 * check: the whole advantage of simulating the past.
 */
export const FEATURED_DRAW = { tour: 'atp', season: 2019, event: 'Wimbledon' }

/**
 * A draw sheet's columns are the rounds a player is still in. The API reports
 * the chance of winning each round, which is the chance of reaching the next,
 * so the labels shift one round along and the last is the title.
 */
export function roundsReached(rounds: ReadonlyArray<string>): string[] {
  return [...rounds.slice(1), 'W']
}
