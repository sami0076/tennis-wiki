import {
  AvailabilityNeverForTier,
  AvailabilityNeverInEra,
  AvailabilityNotRecorded,
} from '../api/client'
import { absenceReason } from '../lib/absence'
import { useUrlParam } from '../lib/useUrlParam'
import {
  AbsentCell,
  Button,
  ButtonLink,
  EmptyState,
  PartialAggregate,
  RankDelta,
  RivalryStrip,
  Skeleton,
  Sparkline,
  SplitBar,
  StatTable,
  SurfaceDot,
  SurfaceToggle,
  WinLossMark,
  type Column,
} from '../components'
import styles from './Gallery.module.css'

interface Row {
  date: string
  opponent: string
  surface: string | null
  won: boolean
  aces: number | null
}

const rows: ReadonlyArray<Row> = [
  { date: '2019-07-14', opponent: 'Roger Federer', surface: 'grass', won: true, aces: 23 },
  { date: '2019-06-08', opponent: 'Dominic Thiem', surface: 'clay', won: false, aces: 4 },
  { date: '1979-06-09', opponent: 'Victor Pecci', surface: 'clay', won: true, aces: null },
  { date: '1978-07-08', opponent: 'Jimmy Connors', surface: 'grass', won: true, aces: null },
]

const columns: ReadonlyArray<Column<Row>> = [
  { key: 'date', header: 'Date', value: (row) => row.date },
  { key: 'opponent', header: 'Opponent', value: (row) => row.opponent },
  {
    key: 'surface',
    header: 'Surface',
    value: (row) => row.surface,
    render: (row) => <SurfaceDot surface={row.surface} />,
  },
  {
    key: 'result',
    header: 'Result',
    value: (row) => (row.won ? 1 : 0),
    render: (row) => <WinLossMark won={row.won} />,
    sortable: false,
  },
  { key: 'aces', header: 'Aces', align: 'right', value: (row) => row.aces },
]

const trajectory = [
  { date: '2018-01-01', elo: 2050 },
  { date: '2018-07-01', elo: 2110 },
  { date: '2019-01-01', elo: 2080 },
  { date: '2019-07-01', elo: 2190 },
  { date: '2020-01-01', elo: 2240 },
  { date: '2020-07-01', elo: 2205 },
]

const rivalry = [
  { wonByA: true, surface: 'grass', description: 'Wimbledon 1980 final' },
  { wonByA: false, surface: 'hard', description: 'US Open 1980 final' },
  { wonByA: true, surface: 'clay', description: 'Roland Garros 1981 final' },
  { wonByA: false, surface: 'grass', description: 'Wimbledon 1981 final' },
]

/**
 * The gallery renders every component in every state it has, on one page.
 *
 * It is the fastest way to check the design system against the mockups without
 * waiting for the pages that use it, and it is the only place the three
 * absences appear side by side, which is the comparison that matters most.
 */
export function Gallery() {
  const [surface, setSurface] = useUrlParam('surface')

  return (
    <>
      <section className={styles.intro}>
        <h1 className={styles.title}>Components</h1>
        <p className={styles.lede}>
          Every component in the inventory, in every state. Not a page anyone visits: a
          place to check the tokens, the hairlines and the absence system against the
          design before a page depends on them.
        </p>
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>The three absences</h2>
        <p className={styles.note}>
          These are the reason the whole data model exists. They must never collapse into
          one another, and none of them is a zero.
        </p>
        <EmptyState
          heading="No serve statistics for this career"
          reason={absenceReason(AvailabilityNeverInEra)}
          action={<ButtonLink to="/">See what the database does hold</ButtonLink>}
        />
        <EmptyState
          heading="No serve statistics at this level"
          reason={absenceReason(AvailabilityNeverForTier)}
          action={<ButtonLink to="/">See what the database does hold</ButtonLink>}
        />
        <EmptyState
          heading="No serve statistics for these matches"
          reason={absenceReason(AvailabilityNotRecorded)}
        />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>StatTable, AbsentCell and PartialAggregate</h2>
        <p className={styles.note}>
          Sort the aces column both ways: the two unrecorded matches stay at the bottom
          either way, because sorting them as zero would read as the worst two performances
          in the table.
        </p>
        <PartialAggregate recorded={2} total={4} noun="Ace counts">
          <StatTable
            caption="Four matches, two of them from before serve statistics were kept."
            columns={columns}
            rows={rows}
            rowKey={(row) => row.date}
            defaultSort={{ key: 'date', direction: 'desc' }}
            aggregate={['Total', '', '', '', '27']}
          />
        </PartialAggregate>
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SurfaceToggle</h2>
        <p className={styles.note}>
          The active filter is underlined in its own surface colour, and the selection is in
          the URL: reload the page and it survives. Currently {surface ?? 'all surfaces'}.
        </p>
        <SurfaceToggle value={surface} onChange={setSurface} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SplitBar</h2>
        <SplitBar label="Career meetings" left={11} right={7} />
        <SplitBar label="On clay" left={2} right={6} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>RivalryStrip</h2>
        <RivalryStrip results={rivalry} nameA="Bjorn Borg" nameB="John McEnroe" />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>Sparkline</h2>
        <p className={styles.note}>
          Hand-rolled SVG, no axes and no library. The multi-series trajectory chart waits
          for the ratings endpoints.
        </p>
        <Sparkline points={trajectory} label="Elo rating over three seasons" />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>Marks and controls</h2>
        <div className={styles.row}>
          <WinLossMark won={true} />
          <WinLossMark won={false} />
          <RankDelta delta={12} />
          <RankDelta delta={-4} />
          <RankDelta delta={0} />
          <SurfaceDot surface="clay" />
          <SurfaceDot surface={null} />
          <AbsentCell label="Aces" />
          <Button>Simulate this matchup</Button>
          <Button disabled>Simulate this matchup</Button>
        </div>
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>Skeleton</h2>
        <p className={styles.note}>
          Shaped like the content it replaces, so nothing jumps when the answer lands.
        </p>
        <Skeleton lines={4} />
      </section>
    </>
  )
}
