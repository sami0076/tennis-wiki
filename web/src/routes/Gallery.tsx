import { useState } from 'react'
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
  Meta,
  OddsBar,
  PartialAggregate,
  Playback,
  PlayerSearch,
  PlayerSummary,
  RankDelta,
  RivalryStrip,
  Scoreboard,
  Scorelines,
  Skeleton,
  Sparkline,
  SplitBar,
  StatTable,
  SurfaceDot,
  SurfaceToggle,
  TourFilter,
  Tracker,
  TrajectoryChart,
  WinLossMark,
  WinSplit,
  type Column,
} from '../components'
import type { Snapshot } from '../lib/playback'
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

// Five lines off the one above, far enough apart to tell the ramp's four steps
// from each other at 360px.
const leaders = ['Leader', 'Second', 'Third', 'Fourth', 'Fifth'].map((name, index) => ({
  name,
  position: index + 1,
  points: trajectory.map((point, week) => ({
    date: point.date,
    elo: point.elo - index * 60 + week * index * 4,
  })),
}))

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
/** The simulator's fixture chain, for the played-out match. */
const chain = {
  point: [0.621, 0.606] as [number, number],
  hold: [0.777, 0.748] as [number, number],
  set: [0.552, 0.448] as [number, number],
  match: [0.596, 0.404] as [number, number],
}

/** A board one set in, the first taken on a tiebreak, the second at 3-4. */
const midMatch: Snapshot = {
  sets: [{ a: 7, b: 6, tiebreakA: null, tiebreakB: 5, aWon: true }],
  current: { a: 3, b: 4 },
  setsWon: [1, 0],
  points: [61, 58],
  breakPointsWon: [1, 2],
  breakPointsFaced: [3, 4],
  holds: [9, 8],
  server: 1,
  flash: null,
  pop: null,
}

export function Gallery() {
  const [surface, setSurface] = useUrlParam('surface')
  const [tour, setTour] = useState<string | null>(null)
  const [query, setQuery] = useState('')

  return (
    <>
      <section className={styles.intro}>
        <h1 className={styles.title}>Components</h1>
        <p className={styles.lede}>
          Every component in the inventory, in every state. Not a page anyone visits: a
          place to check the tokens, the ruling and the absence system against the
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
        <h2 className={styles.name}>Meta</h2>
        <p className={styles.note}>
          Typed fields two spaces apart. Absent parts drop out rather than leaving a gap
          with nothing after it.
        </p>
        <Meta parts={['Spain', 'right-handed', 23, 'turned pro 2018']} />
        <Meta parts={['Sweden', 'right-handed', '1973-1983']} />
        <Meta parts={['Cincinnati QF', '6-4 7-5']} />
        <Meta parts={['Unknown country', null, '', 'no hand recorded']} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SurfaceToggle</h2>
        <p className={styles.note}>
          The active filter is boxed in its own surface colour, and the selection is in the
          URL: reload the page and it survives. Currently {surface ?? 'all surfaces'}.
        </p>
        <SurfaceToggle value={surface} onChange={setSurface} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>PlayerSearch and TourFilter</h2>
        <p className={styles.note}>
          Live against the API this page is served from. Arrows move, Enter picks, Escape
          backs out, and below two characters it does not ask at all.
        </p>
        <TourFilter value={tour} onChange={setTour} />
        <PlayerSearch
          label="Search players"
          placeholder="Try a surname"
          value={query}
          onChange={setQuery}
          tour={tour}
          onSelect={(player) => setQuery(player.name)}
        />
        <div className={styles.note}>
          One result row on its own, which is what the search page lists:
        </div>
        <PlayerSummary
          player={{
            slug: 'bjorn-borg',
            name: 'Bjorn Borg',
            tour: 'atp',
            country: 'Sweden',
            matches: 807,
            best_tier: 'tour',
            score: 1,
          }}
        />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SplitBar</h2>
        <SplitBar label="Career meetings" left={11} right={7} max={11} />
        <SplitBar label="Serve points won" left={72} right={68} max={100} format={(v) => `${v}%`} />
        <SplitBar label="Return points won" left={43} right={45} max={100} format={(v) => `${v}%`} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>WinSplit and OddsBar</h2>
        <p className={styles.note}>
          The two marks the simulator adds. The players stay monochrome; the odds bars take
          the event&apos;s surface colour, and each carries the interval it earned rather
          than reporting a sampled figure as though it were exact.
        </p>
        <WinSplit nameA="Carlos Alcaraz" nameB="Jannik Sinner" share={0.622} />
        <div className={styles.note}>Title odds, 128 draw, 10,000 runs:</div>
        <OddsBar name="Novak Djokovic" probability={0.401} interval={0.01} max={0.401} surface="grass" />
        <OddsBar name="Roger Federer" probability={0.277} interval={0.009} max={0.401} surface="grass" />
        <OddsBar name="Rafael Nadal" probability={0.075} interval={0.005} max={0.401} surface="grass" />
        <OddsBar name="The field" probability={0.247} max={0.401} surface={null} />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>Scorelines, Scoreboard, Tracker and Playback</h2>
        <p className={styles.note}>
          The simulator&apos;s second half: the chance of each set score under the chain, and
          one match played out from it. The board boxes the set in progress in the
          surface&apos;s hue and marks the server with a square in it; the tally is counted
          from the points the sample played. Watch a match to see the beats.
        </p>
        <Scorelines setShare={0.552} bestOf={5} nameA="Alcaraz" nameB="Sinner" />
        <div className={styles.note}>Mid-match, a tiebreak set on the board:</div>
        <Scoreboard snapshot={midMatch} names={['Alcaraz', 'Sinner']} surface="clay" playing />
        <Tracker snapshot={midMatch} />
        <div className={styles.note}>The whole section, idle until asked:</div>
        <Playback chain={chain} bestOf={3} players={[{ name: 'Carlos Alcaraz' }, { name: 'Jannik Sinner' }]} surface="clay" />
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
        <h2 className={styles.name}>TrajectoryChart</h2>
        <p className={styles.note}>
          The multi-series version: one shared pair of axes, the top three down the ink
          ramp, everyone else as the field. Crossing lines mean a lead changing hands,
          which is only true because the range is shared. Still no charting library.
        </p>
        <TrajectoryChart lines={leaders} />
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
