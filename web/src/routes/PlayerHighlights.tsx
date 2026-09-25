import { Link } from 'react-router-dom'
import type { BestWin, FinalsRecord, PlayerHighlights, Rival, Streak } from '../api/client'
import type { Resource } from '../api/useResource'
import {
  Card,
  EmptyState,
  EventLink,
  Flag,
  Kicker,
  Odometer,
  RoundFunnel,
  Score,
  Skeleton,
  StatTable,
  SurfaceDot,
  type Column,
} from '../components'
import { formatElo, surname } from '../lib/format'
import { levelLabel } from '../lib/tier'
import styles from './Player.module.css'

/**
 * What each event category is called in a sentence. The API groups finals the
 * same way the season index does, so the words are the same words.
 */
const CATEGORY_WORDS: Record<string, string> = {
  slam: 'Grand Slams',
  finals: 'Tour finals',
  masters: 'Masters',
  olympics: 'Olympics',
  team: 'Team events',
  tour: 'Tour level',
  challenger: 'Challengers',
  futures: 'Futures',
  itf: 'ITF',
}

/**
 * The runs and the schedule: four figures that say what a table of rates
 * cannot, which is what this career actually did and who it did it against.
 */
export function RunsSection({ highlights }: { highlights: Resource<PlayerHighlights> }) {
  if (highlights.state === 'loading') {
    return (
      <Card title="Runs and the schedule">
        <Skeleton lines={4} />
      </Card>
    )
  }
  if (highlights.state === 'error') return null

  const { streaks, schedule } = highlights.data
  const best = streaks.find((s) => s.kind === 'best')
  const current = streaks.find((s) => s.kind === 'current')

  if (best === undefined && current === undefined && schedule.rated_matches === 0) return null

  return (
    <Card title="Runs and the schedule">
      <div className={styles.runs}>
        {best === undefined ? null : (
          <Figure
            kicker="Longest winning run"
            value={best.length}
            unit={best.length === 1 ? 'win' : 'wins'}
            foot={`${best.from} – ${best.to}`}
          />
        )}
        {current === undefined ? null : (
          <Figure
            kicker="Current run"
            value={current.length}
            unit={runWord(current)}
            foot={`since ${current.from}`}
          />
        )}
        {schedule.average_elo === null ? null : (
          <Figure
            kicker="Average opponent"
            value={Math.round(schedule.average_elo)}
            unit="Elo"
            foot={`over ${schedule.rated_matches.toLocaleString()} rated matches`}
          />
        )}
        {schedule.highest_elo === null ? null : (
          <Figure
            kicker="Toughest faced"
            value={Math.round(schedule.highest_elo)}
            unit="Elo"
            foot={
              schedule.elite_matches === 0
                ? `never met an opponent rated ${formatElo(schedule.elite_elo)}`
                : `${schedule.elite_wins}-${schedule.elite_matches - schedule.elite_wins} against ${formatElo(schedule.elite_elo)} and up`
            }
          />
        )}
      </div>
    </Card>
  )
}

function runWord(streak: Streak): string {
  if (streak.won) return streak.length === 1 ? 'win' : 'wins'
  return streak.length === 1 ? 'defeat' : 'defeats'
}

function Figure({
  kicker,
  value,
  unit,
  foot,
}: {
  kicker: string
  value: number
  unit: string
  foot: string
}) {
  return (
    <div className={styles.run}>
      <Kicker>{kicker}</Kicker>
      <div className={styles.runValue}>
        <Odometer value={value} />
        <span className={styles.runUnit}>{unit}</span>
      </div>
      <div className={styles.runFoot}>{foot}</div>
    </div>
  )
}

/**
 * The wins that cost the most, ranked by what the opponent was worth on the
 * day. This is the list every other career figure is an average of, and the
 * one a reader actually wants to see.
 */
export function BestWinsSection({ highlights }: { highlights: Resource<PlayerHighlights> }) {
  if (highlights.state === 'loading') {
    return (
      <Card title="Biggest wins">
        <Skeleton lines={5} />
      </Card>
    )
  }
  if (highlights.state === 'error') return null

  const wins = highlights.data.best_wins
  if (wins.length === 0) {
    return (
      <Card title="Biggest wins">
        <EmptyState
          heading="No rated win to rank"
          reason="Ranking a win needs a rating for the opponent in the week it was played. Either this career has no main-draw win in the database, or the model had not rated any of the opponents yet."
        />
      </Card>
    )
  }

  return (
    <Card title="Biggest wins">
      <StatTable
        caption="Main draw only, ranked on the opponent's Elo that week rather than their career best."
        columns={winColumns}
        rows={wins}
        rowKey={(row) => `${row.date}-${row.opponent.slug}-${row.round}`}
      />
    </Card>
  )
}

const winColumns: ReadonlyArray<Column<BestWin>> = [
  {
    key: 'elo',
    header: 'Elo then',
    align: 'right',
    value: (row) => row.opponent_elo,
    render: (row) => <span className={styles.bigElo}>{formatElo(row.opponent_elo)}</span>,
  },
  {
    key: 'opponent',
    header: 'Beat',
    wrap: true,
    value: (row) => row.opponent.name,
    render: (row) => (
      <>
        <Link to={`/players/${row.opponent.slug}`} className={styles.opponent}>
          {row.opponent.name}
        </Link>
        <div className={styles.event}>
          <span className={styles.eventSurface}>
            <SurfaceDot surface={row.surface} label={false} />{' '}
          </span>
          <EventLink name={row.tournament} slug={row.event_slug} season={row.season} /> {row.round}
        </div>
      </>
    ),
  },
  {
    key: 'level',
    header: 'Level',
    value: (row) => row.level,
    render: (row) => levelLabel(row.level, row.tier, ''),
    wide: true,
  },
  {
    key: 'score',
    header: 'Score',
    wrap: true,
    minWidth: '10ch',
    sortable: false,
    value: (row) => row.score,
    render: (row) => <Score score={row.score} incomplete={false} />,
  },
  { key: 'date', header: 'Date', align: 'right', value: (row) => row.date, wide: true },
]

/**
 * How far a career kept getting, and what it won when it got there. Two
 * answers to one question, which is why they share a card.
 */
export function RoundsSection({ highlights }: { highlights: Resource<PlayerHighlights> }) {
  if (highlights.state === 'loading') {
    return (
      <Card title="How far, and what was won">
        <Skeleton lines={6} />
      </Card>
    )
  }
  if (highlights.state === 'error') return null

  const { rounds, finals } = highlights.data
  if (rounds.length === 0) return null

  return (
    <Card title="How far, and what was won">
      <RoundFunnel
        rounds={rounds.map((row) => ({
          round: row.round,
          matches: Number(row.matches),
          wins: Number(row.wins),
        }))}
      />
      {finals.length === 0 ? null : <Shelf finals={finals} />}
    </Card>
  )
}

/** Titles and finals by what the event was, biggest first. */
function Shelf({ finals }: { finals: ReadonlyArray<FinalsRecord> }) {
  return (
    <>
      <Kicker className={styles.shelfLabel}>Finals reached</Kicker>
      <ul className={styles.shelf}>
        {finals.map((row) => (
          <li key={row.category} className={styles.shelfItem}>
            <span className={styles.shelfCount}>
              {row.titles}
              <span className={styles.shelfOf}>of {row.finals}</span>
            </span>
            <span className={styles.shelfName}>{CATEGORY_WORDS[row.category] ?? row.category}</span>
          </li>
        ))}
      </ul>
    </>
  )
}

/**
 * Who a career was spent against. Ordered on meetings rather than on the
 * record, because the question is who kept turning up.
 */
export function RivalsSection({
  highlights,
  slug,
}: {
  highlights: Resource<PlayerHighlights>
  slug: string
}) {
  if (highlights.state === 'loading') {
    return (
      <Card title="Most-played opponents">
        <Skeleton lines={5} />
      </Card>
    )
  }
  if (highlights.state === 'error') return null

  const rivals = highlights.data.rivals
  if (rivals.length === 0) return null

  return (
    <Card title="Most-played opponents">
      <ul className={styles.rivals}>
        {rivals.map((rival) => (
          <li key={rival.slug} className={styles.rival}>
            <Link className={styles.rivalName} to={`/players/${rival.slug}`}>
              <Flag country={rival.country} code={false} />
              {rival.name}
            </Link>
            <RivalBar rival={rival} />
            <span className={styles.rivalRecord}>
              {rival.wins}&#8209;{Number(rival.matches) - Number(rival.wins)}
            </span>
            <Link className={styles.rivalLink} to={`/h2h/${slug}/${rival.slug}`}>
              H2H →
            </Link>
          </li>
        ))}
      </ul>
    </Card>
  )
}

function RivalBar({ rival }: { rival: Rival }) {
  const matches = Number(rival.matches)
  const wins = Number(rival.wins)
  const losses = matches - wins
  return (
    <span
      className={styles.rivalBar}
      aria-label={`${wins} won, ${losses} lost against ${surname(rival.name)}`}
    >
      {wins > 0 ? <span className={styles.rivalWon} style={{ flexGrow: wins }} /> : null}
      {losses > 0 ? <span className={styles.rivalLost} style={{ flexGrow: losses }} /> : null}
    </span>
  )
}
