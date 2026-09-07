import { getCoverage } from '../api/endpoints'
import { useResource } from '../api/useResource'
import { AbsentCell, Skeleton, StatTable, type Column } from '../components'
import type { CoverageEntry } from '../api/client'
import styles from './Home.module.css'

const columns: ReadonlyArray<Column<CoverageEntry>> = [
  { key: 'tour', header: 'Tour', value: (row) => row.tour.toUpperCase() },
  { key: 'tier', header: 'Tier', value: (row) => row.tier },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  { key: 'first', header: 'From', align: 'right', value: (row) => row.first_match },
  { key: 'last', header: 'To', align: 'right', value: (row) => row.last_match },
  {
    key: 'stats',
    header: 'With serve stats',
    align: 'right',
    // Zero matches with statistics is a real, recorded zero for a tier that
    // never had them -- the percentage is what is genuinely absent.
    value: (row) => (row.matches_with_stats === 0 ? null : row.stats_percentage),
    render: (row) =>
      row.matches_with_stats === 0 ? (
        <AbsentCell label="With serve stats" />
      ) : (
        `${row.stats_percentage.toFixed(1)}%`
      ),
  },
]

/**
 * Home is the coverage of the database, queried rather than claimed.
 *
 * The hero trajectory chart in the design needs the ratings endpoints, which do
 * not exist yet, so this page ships the part that is real and says nothing about
 * the part that is not.
 */
export function Home() {
  const coverage = useResource((signal) => getCoverage(signal), [])

  return (
    <>
      <section className={styles.hero}>
        <h1 className={styles.title}>Every match, and every gap between them.</h1>
        <p className={styles.standfirst}>
          1.6 million matches across both tours, back to 1922, rated on one Elo scale. Where
          a statistic was never recorded, this site explains which kind of never.
        </p>
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>What is actually in the database</h2>

        {coverage.state === 'loading' ? <Skeleton lines={6} /> : null}

        {coverage.state === 'error' ? (
          <p className={styles.error}>
            The coverage figures could not be loaded: {coverage.error.message} The API may not
            be running; start it with <code>make api</code> and reload.
          </p>
        ) : null}

        {coverage.state === 'ready' ? (
          <>
            <div className={styles.through}>
              {Object.entries(coverage.data.current_through).map(([tour, date]) => (
                <span key={tour}>
                  <span className={styles.throughLabel}>{tour.toUpperCase()} through </span>
                  {date}
                </span>
              ))}
            </div>
            <StatTable
              caption="Queried from the database on every request, so the claim cannot drift from the data."
              columns={columns}
              rows={coverage.data.tiers}
              rowKey={(row) => `${row.tour}-${row.tier}`}
              defaultSort={{ key: 'matches', direction: 'desc' }}
            />
          </>
        ) : null}
      </section>
    </>
  )
}
