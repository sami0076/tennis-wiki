import type { PlayerSeason, PlayerSeasons, PlayerSplits, RankBand } from '../api/client'
import type { Resource } from '../api/useResource'
import { EmptyState, Skeleton, StatRow, StatTable, type Column } from '../components'
import { formatPercent } from '../lib/format'
import styles from './Player.module.css'

const BAND_WORDS: Record<string, string> = {
  '1': 'vs No. 1',
  '5': 'vs top 5',
  '10': 'vs top 10',
  '20': 'vs top 20',
  '50': 'vs top 50',
  '100': 'vs top 100',
  outside: 'vs outside the top 100',
  unranked: 'vs unranked',
}

/**
 * The record by who was beaten and how close it was: the bands of the
 * opponent's ranking on the day, the record against higher- and lower-ranked
 * opponents, and the matches a final-set tiebreak decided. Each is captioned
 * with the matches it is a record over, because the ranking is known for
 * some matches and not others.
 */
export function OpponentsSection({ splits }: { splits: PlayerSplits }) {
  const bands = splits.by_rank.filter((band) => band.matches > 0)
  const unranked = splits.by_rank.find((band) => band.band === 'unranked')?.matches ?? 0
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>By opponent&apos;s ranking</h2>
      {splits.ranked === 0 ? (
        <EmptyState
          heading="No opponent's ranking is recorded"
          reason="The files carry the ranking each side held on the day for most tour-level matches and few below it. Nothing here was ranked."
        />
      ) : (
        <>
          <StatTable
            caption={`Over the ${splits.ranked} matches where the opponent's ranking on the day is known${unranked > 0 ? `; ${unranked} more were against an unranked opponent` : ''}. The bands nest: a win over No. 1 is in every band down to the top 100.`}
            columns={bandColumns}
            rows={bands}
            rowKey={(row) => row.band}
          />
          <StatRow label="Against higher-ranked">{record(splits.higher)}</StatRow>
          <StatRow label="Against lower-ranked">{record(splits.lower)}</StatRow>
        </>
      )}
      <StatRow label="Decided by a final-set tiebreak">{record(splits.final_set_tiebreaks)}</StatRow>
      <p className={styles.caption}>
        Higher and lower over the matches where both rankings are known. The final-set tiebreak
        record is over the {splits.scored} matches whose score could be read; tiebreaks and
        deciding sets on their own are under pressure, above.
      </p>
    </section>
  )
}

function record(r: { matches: number; wins: number }): string {
  return `${r.wins}-${r.matches - r.wins}`
}

const bandColumns: ReadonlyArray<Column<RankBand>> = [
  { key: 'band', header: 'Opponent', wrap: true, sortable: false, value: (row) => BAND_WORDS[row.band] ?? row.band },
  { key: 'record', header: 'W-L', align: 'right', sortable: false, value: (row) => row.matches, render: (row) => record(row) },
  {
    key: 'pct',
    header: 'Won',
    align: 'right',
    sortable: false,
    value: (row) => (row.matches === 0 ? null : (100 * row.wins) / row.matches),
    render: (row) => formatPercent((100 * row.wins) / row.matches),
  },
]

/**
 * A career a year at a time. Every rate column stands on its own count of
 * matches: a season with four recorded matches and sixty played is the common
 * case in the 1990s, and the four must not read as the sixty.
 */
export function SeasonsSection({ seasons }: { seasons: Resource<PlayerSeasons> }) {
  if (seasons.state === 'loading') {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Year by year</h2>
        <Skeleton lines={6} />
      </section>
    )
  }
  if (seasons.state === 'error') {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Year by year</h2>
        <p className={styles.error}>The seasons could not be loaded: {seasons.error.message}</p>
      </section>
    )
  }
  const rows = seasons.data.seasons
  if (rows.length === 0) return null
  const withServe = rows.reduce((n, row) => n + row.with_serve, 0)
  const played = rows.reduce((n, row) => n + row.matches, 0)
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Year by year</h2>
      <StatTable
        caption={`Sets, games and tiebreaks are over the matches whose score could be read; hold, break, ace, double-fault and dominance over the matches carrying serve lines, which is the Lines column: ${withServe} of ${played} matches. A rate with nothing to divide by is n/r.`}
        columns={seasonColumns}
        rows={rows}
        rowKey={(row) => String(row.season)}
        defaultSort={{ key: 'season', direction: 'desc' }}
      />
      <p className={styles.caption}>
        The dominance ratio is return points won over serve points lost; above 1 is winning
        more points than losing. Defined on the methodology page.
      </p>
    </section>
  )
}

const pct = (v: number | null) => (v === null ? null : formatPercent(v))

const seasonColumns: ReadonlyArray<Column<PlayerSeason>> = [
  { key: 'season', header: 'Season', value: (row) => row.season },
  { key: 'matches', header: 'Matches', align: 'right', value: (row) => row.matches },
  {
    key: 'record',
    header: 'W-L',
    align: 'right',
    value: (row) => row.wins,
    render: (row) => `${row.wins}-${row.losses}`,
  },
  { key: 'titles', header: 'Titles', align: 'right', value: (row) => row.titles },
  { key: 'sets', header: 'Sets', align: 'right', value: (row) => row.sets_pct, render: (row) => pct(row.sets_pct) },
  { key: 'games', header: 'Games', align: 'right', wide: true, value: (row) => row.games_pct, render: (row) => pct(row.games_pct) },
  { key: 'tiebreaks', header: 'Tiebreaks', align: 'right', wide: true, value: (row) => row.tiebreaks_pct, render: (row) => pct(row.tiebreaks_pct) },
  { key: 'lines', header: 'Lines', align: 'right', value: (row) => row.with_serve },
  { key: 'hold', header: 'Hold', align: 'right', value: (row) => row.hold_pct, render: (row) => pct(row.hold_pct) },
  { key: 'break', header: 'Break', align: 'right', wide: true, value: (row) => row.break_pct, render: (row) => pct(row.break_pct) },
  { key: 'ace', header: 'Ace', align: 'right', wide: true, value: (row) => row.ace_pct, render: (row) => pct(row.ace_pct) },
  { key: 'df', header: 'DF', align: 'right', wide: true, value: (row) => row.df_pct, render: (row) => pct(row.df_pct) },
  {
    key: 'dominance',
    header: 'Dominance',
    align: 'right',
    value: (row) => row.dominance,
    render: (row) => (row.dominance === null ? null : row.dominance.toFixed(2)),
  },
]
