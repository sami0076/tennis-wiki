import { Link } from 'react-router-dom'
import type { SeasonRow, SeasonTour } from '../api/client'
import { getSeasons } from '../api/endpoints'
import { useResource } from '../api/useResource'
import { Note, PageHeader, Skeleton } from '../components'
import { surname } from '../lib/format'
import { SURFACES, surfaceLabel, surfaceVar, surfaceWash } from '../lib/surface'
import styles from './Seasons.module.css'
import { breadcrumbs, useJsonLd } from '../lib/jsonld'

/**
 * The calendar: one row per year, both tours on it, since a season is the
 * one place the two tours share a calendar. Each half is the year's shape --
 * how many events on each surface -- and its four names, the Slam champions.
 * The women's rows reach back to 1923; the men's files start in 1968, and a
 * year before that is a row with one half rather than a year the page skips.
 */
export function Seasons() {
  const seasons = useResource((signal) => getSeasons(signal), [])
  useJsonLd('breadcrumbs', breadcrumbs([{ name: 'Seasons', path: '/seasons' }]))

  return (
    <>
      <PageHeader
        kicker={
          <Link className={styles.path} to="/tournaments">
            Tournaments
          </Link>
        }
        title="Seasons"
        lede="The calendar a year at a time, both tours on every row: events on each surface, then the year's Slam champions."
      />

      {seasons.state === 'loading' ? (
        <Skeleton lines={14} />
      ) : seasons.state === 'error' ? (
        <p className={styles.error}>
          The calendar could not be loaded: {seasons.error.message} The API may not be running;
          start it with <code>make api</code> and reload.
        </p>
      ) : (
        <>
          <div className={styles.head} aria-hidden="true">
            <span className={styles.year}>Year</span>
            <span className={styles.half}>ATP</span>
            <span className={styles.half}>WTA</span>
          </div>
          <ol className={styles.rows}>
            {seasons.data.data.map((row) => (
              <Year key={row.season} row={row} through={seasons.data.current_through} />
            ))}
          </ol>
          <Note>
            Events on each surface, then the Slam champions of the year in calendar order. A
            row marked in progress is complete to the tour&apos;s last match, not to the end
            of the year. A surface the file did not record is counted in pencil.
          </Note>
        </>
      )}
    </>
  )
}

function Year({ row, through }: { row: SeasonRow; through: Record<string, string> }) {
  return (
    <li className={styles.row}>
      <Link className={styles.year} to={`/seasons/${row.season}`}>
        {row.season}
      </Link>
      <Half tour="atp" season={row.season} data={row.atp} through={through.atp} />
      <Half tour="wta" season={row.season} data={row.wta} through={through.wta} />
    </li>
  )
}

interface HalfProps {
  tour: string
  season: number
  data: SeasonTour | null
  through: string | undefined
}

function Half({ tour, season, data, through }: HalfProps) {
  if (data === null) {
    return (
      <div className={`${styles.half} ${styles.none}`}>
        <span className={styles.tourTag}>{tour.toUpperCase()}</span>
        <span className={styles.absent}>no {tour.toUpperCase()} file for {season}</span>
      </div>
    )
  }
  return (
    <div className={styles.half}>
      <span className={styles.tourTag}>{tour.toUpperCase()}</span>
      <div className={styles.surfaces}>
        {SURFACES.map((surface) =>
          data.surfaces[surface] > 0 ? (
            <span
              key={surface}
              className={styles.count}
              title={surfaceLabel(surface)}
              style={{ color: surfaceVar(surface), background: surfaceWash(surface) }}
            >
              <span className={styles.square} style={{ background: surfaceVar(surface) }} aria-hidden="true" />
              <span className="sr-only">{surfaceLabel(surface)} </span>
              {data.surfaces[surface]}
            </span>
          ) : null,
        )}
        {data.surfaces.unknown > 0 ? (
          <span
            className={styles.count}
            title="Surface not recorded"
            style={{ color: surfaceVar(null), background: surfaceWash(null) }}
          >
            <span className={styles.square} style={{ background: surfaceVar(null) }} aria-hidden="true" />
            <span className="sr-only">Surface not recorded </span>
            {data.surfaces.unknown}
          </span>
        ) : null}
        {data.ties > 0 ? <span className={styles.ties}>{data.ties} ties</span> : null}
        {data.partial && through !== undefined ? (
          <span className={styles.partial}>in progress, through {through}</span>
        ) : null}
      </div>
      {data.slams.length > 0 ? (
        <div className={styles.slams}>
          {data.slams.map((slam) => (
            <span key={slam.slug} className={styles.slam}>
              <Link className={styles.event} to={`/tournaments/${slam.slug}/${season}`} title={slam.name}>
                {abbreviate(slam.name)}
              </Link>{' '}
              {slam.champion === null ? (
                <span className={styles.absent}>n/r</span>
              ) : (
                <Link className={styles.champion} to={`/players/${slam.champion.slug}`} title={slam.champion.name}>
                  {surname(slam.champion.name)}
                </Link>
              )}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  )
}

/** The Slams as a season row has room for: a short name, the full one on hover. */
function abbreviate(name: string): string {
  const known: Record<string, string> = {
    'Australian Open': 'AO',
    'Australian Championships': 'AC',
    'Roland Garros': 'RG',
    'French Championships': 'FC',
    Wimbledon: 'W',
    'US Open': 'USO',
    'US National Championships': 'USN',
  }
  return known[name] ?? name.split(' ').map((word) => word[0]).join('')
}
