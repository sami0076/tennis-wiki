import { Link, useParams } from 'react-router-dom'
import { ApiError, type Event as EventData, type EventEdition } from '../api/client'
import { getEvent } from '../api/endpoints'
import { useResource } from '../api/useResource'
import { ButtonLink, EmptyState, Meta, Score, Skeleton, StatTable, SurfaceDot, type Column } from '../components'
import { formatScore } from '../lib/format'
import { breadcrumbs, useJsonLd } from '../lib/jsonld'
import styles from './Event.module.css'

/**
 * A tournament across seasons: every edition as a row, with who won it and
 * against whom, and how each row came to be on this page. ADR-0012 keys an
 * event by the tour's number where there is one and by name where there is
 * not, and the page says which, because the two are different claims.
 */
export function Event() {
  const { slug = '' } = useParams()
  const event = useResource((signal) => getEvent(slug, signal), [slug])
  useJsonLd(
    'breadcrumbs',
    event.state === 'ready'
      ? breadcrumbs([
          { name: 'Tournaments', path: '/tournaments' },
          { name: `${event.data.name}, ${event.data.tour.toUpperCase()}`, path: `/tournaments/${slug}` },
        ])
      : null,
  )

  if (event.state === 'loading') {
    return (
      <>
        <Skeleton lines={2} width="40%" />
        <div className={styles.block}>
          <Skeleton lines={10} />
        </div>
      </>
    )
  }

  if (event.state === 'error') {
    const notFound = event.error instanceof ApiError && event.error.status === 404
    return (
      <EmptyState
        heading={notFound ? 'No tournament has that address' : 'That tournament could not be loaded'}
        reason={event.error.message}
        action={<ButtonLink to="/tournaments">See every tournament</ButtonLink>}
      />
    )
  }

  const data = event.data
  const tour = data.tour.toUpperCase()
  const team = data.editions.some((edition) => edition.link === 'team')
  const span = data.first_season === data.last_season ? String(data.first_season) : `${data.first_season}-${data.last_season}`

  return (
    <>
      <p className={styles.path}>
        <Link to="/tournaments">Tournaments</Link>
      </p>
      <h1 className={styles.title}>{data.name}</h1>
      <Meta parts={[tour, span, `${data.editions.length} ${data.editions.length === 1 ? 'edition' : 'editions'}`]} />
      {data.names.length > 1 ? (
        <p className={styles.names}>
          Played as{' '}
          {data.names.map((run, i) => (
            <span key={`${run.name}-${run.first_season}`}>
              {i > 0 ? ', ' : ''}
              {run.name} {run.first_season === run.last_season ? run.first_season : `${run.first_season}-${run.last_season}`}
            </span>
          ))}
          .
        </p>
      ) : null}

      <div className={styles.block}>
        <StatTable
          caption={team ? 'Every season, as the ties the file carries.' : 'Every edition, most recent first; a season opens its sheet.'}
          columns={team ? teamColumns(data) : editionColumns(data)}
          rows={data.editions}
          rowKey={(row) => `${row.season}-${row.start_date}`}
          defaultSort={{ key: 'season', direction: 'desc' }}
        />
        <p className={styles.caption}>{provenance(data)}</p>
      </div>
    </>
  )
}

/** The counts ADR-0012 asks the page to print: how each edition got here. */
function provenance(event: EventData): string {
  const p = event.provenance
  const tour = event.tour.toUpperCase()
  const parts: string[] = []
  if (p.number > 0) parts.push(`${p.number} by the ${tour}'s number${event.number === null ? '' : ` ${event.number}`}`)
  if (p.override > 0) parts.push(`${p.override} by a checked renumbering`)
  if (p.bridged > 0) parts.push(`${p.bridged} by name, bridged to that number`)
  if (p.name > 0) parts.push(`${p.name} by name alone`)
  if (p.team > 0) parts.push(`${p.team} as ties of a team competition`)
  const total = event.editions.length
  return `Of ${total} ${total === 1 ? 'edition' : 'editions'}, ${parts.join('; ')}. An edition here by number and one here by name are two different claims.`
}

function editionColumns(event: EventData): Column<EventEdition>[] {
  return [
    {
      key: 'season',
      header: 'Season',
      value: (row) => row.season,
      render: (row) => (
        <Link className={styles.season} to={`/tournaments/${event.slug}/${row.season}`}>
          {row.season}
        </Link>
      ),
    },
    {
      key: 'surface',
      header: 'Surface',
      value: (row) => row.surface,
      render: (row) => <SurfaceDot surface={row.surface} />,
      wide: true,
    },
    { key: 'draw', header: 'Draw', align: 'right', value: (row) => row.draw_size, wide: true },
    {
      key: 'champion',
      header: 'Champion',
      wrap: true,
      value: (row) => row.champion?.name ?? null,
      render: (row) =>
        row.champion === null ? null : (
          <Link className={styles.player} to={`/players/${row.champion.slug}`}>
            {row.champion.name}
          </Link>
        ),
    },
    {
      key: 'finalist',
      header: 'Finalist',
      wrap: true,
      value: (row) => row.finalist?.name ?? null,
      render: (row) =>
        row.finalist === null ? null : (
          <Link className={styles.player} to={`/players/${row.finalist.slug}`}>
            {row.finalist.name}
          </Link>
        ),
    },
    {
      key: 'score',
      header: 'Score',
      wrap: true,
      minWidth: '12ch',
      value: (row) => formatScore(row.final_score),
      render: (row) => <Score score={row.final_score} />,
    },
  ]
}

function teamColumns(event: EventData): Column<EventEdition>[] {
  return [
    {
      key: 'season',
      header: 'Season',
      value: (row) => row.season,
      render: (row) => (
        <Link className={styles.season} to={`/tournaments/${event.slug}/${row.season}`}>
          {row.season}
        </Link>
      ),
    },
    { key: 'ties', header: 'Ties', align: 'right', value: (row) => row.ties },
    { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  ]
}
