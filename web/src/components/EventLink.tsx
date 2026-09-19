import { Link } from 'react-router-dom'
import styles from './EventLink.module.css'

interface EventLinkProps {
  name: string
  /** The event's slug, or null for a row the events stage has not keyed. */
  slug: string | null
  season: number
}

/**
 * EventLink is a tournament name as the sheet it is on: a link to the
 * edition where the file keyed the row to an event, and the name alone where
 * it did not, so nothing links to a page that would be a 404.
 */
export function EventLink({ name, slug, season }: EventLinkProps) {
  if (slug === null) return <>{name}</>
  return (
    <Link className={styles.link} to={`/tournaments/${slug}/${season}`}>
      {name}
    </Link>
  )
}
