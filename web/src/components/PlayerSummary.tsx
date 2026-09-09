import type { PlayerSearchResult } from '../api/types.gen'
import { tierLabel } from '../lib/tier'
import { Meta } from './Meta'
import styles from './PlayerSummary.module.css'

interface PlayerSummaryProps {
  player: PlayerSearchResult
}

/**
 * PlayerSummary is one search result: the name, and enough beside it to tell
 * two people apart.
 *
 * At 115,000 players a name is not an identifier, and the ranking will not
 * always settle it -- within a tier it is raw trigram similarity, which favours
 * shorter names. Career match count and best tier let a reader resolve that
 * themselves, which is worth more than pretending the ranking is already right.
 */
export function PlayerSummary({ player }: PlayerSummaryProps) {
  return (
    <>
      <span className={styles.name}>{player.name}</span>
      <Meta
        className={styles.meta}
        parts={[
          player.tour.toUpperCase(),
          player.country,
          // Zero matches is a player the tour lists and this database has no
          // match for. Saying "0 matches" would read as a career that failed.
          player.matches === 0 ? 'no matches in the database' : `${player.matches} matches`,
          tierLabel(player.best_tier),
        ]}
      />
    </>
  )
}
