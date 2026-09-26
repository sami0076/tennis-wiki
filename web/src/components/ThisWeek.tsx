import type { CSSProperties } from 'react'
import { Link } from 'react-router-dom'
import type { ThisWeek as ThisWeekData, WeekEvent, WeekPlayer } from '../api/client'
import type { Resource } from '../api/useResource'
import { ROUND_WORDS } from '../lib/bracket'
import { formatAgo, formatScore } from '../lib/format'
import { levelLabel } from '../lib/tier'
import { Skeleton } from './Skeleton'
import { SurfaceBadge } from './SurfaceBadge'
import styles from './ThisWeek.module.css'

function Name({ player }: { player: WeekPlayer }) {
  const seed = player.seed === null ? null : <span className={styles.seed}>({player.seed})</span>
  return player.slug === null ? (
    <span>
      {player.name} {seed}
    </span>
  ) : (
    <span>
      <Link className={styles.player} to={`/players/${player.slug}`}>
        {player.name}
      </Link>{' '}
      {seed}
    </span>
  )
}

function Stage({ event }: { event: WeekEvent }) {
  if (event.champion !== null) {
    return (
      <span className={styles.stage}>
        Champion <Name player={event.champion} />
      </span>
    )
  }
  return <span className={styles.stage}>{ROUND_WORDS[event.round] ?? event.round} under way</span>
}

/**
 * ThisWeek is the events being played now, as the results so far: one tile
 * per event with how far it has got and its latest matches. Provisional and
 * refreshed hourly, which the heading says rather than implying it is live.
 */
export function ThisWeek({ week, now = new Date() }: { week: Resource<ThisWeekData>; now?: Date }) {
  if (week.state === 'loading') return <Skeleton lines={4} />
  if (week.state === 'error') {
    return <p className={styles.quiet}>This week&apos;s results could not be loaded: {week.error.message}</p>
  }
  const { events, changed_at: changed } = week.data
  if (events.length === 0) {
    return <p className={styles.quiet}>No tour-level event has a result in yet this week.</p>
  }

  return (
    <>
      <p className={styles.quiet}>
        Results so far, not live scores
        {changed === null ? '' : `: updated ${formatAgo(changed, now)}`}. Ratings take them in once
        each event is over.
      </p>
      <ol className={styles.events}>
        {events.map((event, index) => (
          <li
            key={`${event.tour}-${event.name}`}
            className={styles.event}
            style={{ '--i': index } as CSSProperties}
          >
            <div className={styles.head}>
              <span className={styles.name}>{event.name}</span>
              <SurfaceBadge surface={event.surface} />
            </div>
            <span className={styles.level}>{levelLabel(event.level, 'tour', event.tour)}</span>
            <Stage event={event} />
            <ul className={styles.results}>
              {event.latest.slice(0, 3).map((result) => (
                <li
                  key={`${result.round}-${result.winner.name}-${result.loser.name}`}
                  className={styles.result}
                >
                  <span className={styles.round}>{result.round}</span>
                  <span className={styles.match}>
                    <Name player={result.winner} /> d. <Name player={result.loser} />
                  </span>
                  <span className={styles.score}>{formatScore(result.score)}</span>
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ol>
    </>
  )
}
