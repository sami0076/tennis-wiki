import { Link } from 'react-router-dom'
import type { RecentFinals as RecentFinalsData } from '../api/client'
import type { Resource } from '../api/useResource'
import { Score } from './Score'
import { Skeleton } from './Skeleton'
import { SurfaceDot } from './SurfaceDot'
import styles from './RecentFinals.module.css'

interface RecentFinalsProps {
  recent: Resource<RecentFinalsData>
}

/**
 * RecentFinals is the week that ended last Sunday, as the finals played in
 * it: one ruled line per final, both tours, each a link to the sheet and to
 * both players, and the date the data is current to written on it. An
 * off-season week says which week it is showing instead of vanishing. Not a
 * feed: a final is the row on a sheet that says who won the thing.
 */
export function RecentFinals({ recent }: RecentFinalsProps) {
  if (recent.state === 'loading') return <Skeleton lines={4} />
  if (recent.state === 'error') {
    return <p className={styles.error}>The week&apos;s finals could not be loaded: {recent.error.message}</p>
  }
  const data = recent.data
  const through = Object.entries(data.through)
    .sort()
    .map(([tour, date]) => `${tour.toUpperCase()} through ${date}`)
    .join(', ')

  return (
    <div>
      <p className={styles.week}>
        {data.requested !== null ? (
          <>
            Nothing began in the week of {data.requested.from}; the last week with a final was{' '}
            {data.week.from} to {data.week.to}.
          </>
        ) : (
          <>
            The week of {data.week.from} to {data.week.to}, as the finals of the events that began
            it.
          </>
        )}{' '}
        Current {through}.
      </p>
      {data.finals.length > 0 ? (
        <ol className={styles.finals}>
          {data.finals.map((final) => (
            <li key={`${final.tour}-${final.name}-${final.start_date}`} className={styles.final}>
              <span className={styles.tour}>{final.tour.toUpperCase()}</span>
              <span className={styles.event}>
                {final.slug === null ? (
                  final.name
                ) : (
                  <Link className={styles.sheet} to={`/tournaments/${final.slug}/${final.season}`}>
                    {final.name}
                  </Link>
                )}
                {final.surface === null ? null : (
                  <span className={styles.surface}>
                    {' '}
                    <SurfaceDot surface={final.surface} label={false} />
                  </span>
                )}
              </span>
              <span className={styles.result}>
                <Link className={styles.champion} to={`/players/${final.champion.slug}`}>
                  {final.champion.name}
                </Link>{' '}
                d.{' '}
                <Link className={styles.finalist} to={`/players/${final.finalist.slug}`}>
                  {final.finalist.name}
                </Link>
              </span>
              <span className={styles.score}>
                <Score score={final.final_score} />
              </span>
            </li>
          ))}
        </ol>
      ) : null}
      {data.without.map((tour) => (
        <p key={tour} className={styles.without}>
          No {tour.toUpperCase()} final that week.
        </p>
      ))}
    </div>
  )
}
