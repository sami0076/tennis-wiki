/**
 * The draw the home page showcases. There is no upcoming tournament in this
 * database and never will be, so it is a played one, and one whose answer a
 * reader can check: the whole advantage of simulating the past.
 *
 * The simulator no longer opens on it. That page takes the top of
 * /simulate/draws, so which draw it opens on follows the data rather than
 * pinning the page to one tournament forever; this is only its backstop for a
 * list that failed to load.
 */
export const FEATURED_DRAW = { event: 'wimbledon-atp', season: 2019 }

/**
 * A draw sheet's columns are the rounds a player is still in. The API reports
 * the chance of winning each round, which is the chance of reaching the next,
 * so the labels shift one round along and the last is the title.
 */
export function roundsReached(rounds: ReadonlyArray<string>): string[] {
  return [...rounds.slice(1), 'W']
}
