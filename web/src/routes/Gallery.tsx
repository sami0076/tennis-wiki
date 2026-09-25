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
  AreaChart,
  Button,
  ButtonLink,
  Card,
  ChartedMark,
  ChartedSheet,
  CountUp,
  CourtArt,
  DrawSheet,
  EmptyState,
  FormPills,
  Kicker,
  Meta,
  OddsBar,
  PageHeader,
  PartialAggregate,
  Playback,
  PlayerSearch,
  PlayerSummary,
  RankDelta,
  RivalryStrip,
  RoundFunnel,
  RoundList,
  RoundStepper,
  SectionRail,
  Scoreboard,
  Scorelines,
  Skeleton,
  Sparkline,
  SplitBar,
  StatTable,
  SurfaceBadge,
  SurfaceDot,
  SurfaceToggle,
  TourFilter,
  Tracker,
  TrajectoryChart,
  WinLossMark,
  WinSplit,
  type Column,
} from '../components'
import type { EditionMatch, EditionSide } from '../api/client'
import { buildBracket, pageSheets, roundGroups, MAIN_ROUNDS } from '../lib/bracket'
import type { Snapshot } from '../lib/playback'
import styles from './Gallery.module.css'

/**
 * A complete draw of `size` players in which the lower number always wins,
 * so the sheet's every line is predictable; seeds on the top eight. `drop`
 * removes matches, to show a bye and a row the file lacks.
 */
function sampleDraw(size: number, drop: ReadonlyArray<[string, number]> = []): EditionMatch[] {
  const rounds: string[] = MAIN_ROUNDS.slice(-Math.log2(size))
  const side = (n: number): EditionSide => ({
    slug: `player-${n}`,
    name: `Player ${n}`,
    country: n % 3 === 0 ? 'ESP' : n % 3 === 1 ? 'SRB' : 'ITA',
    seed: n <= 8 ? n : null,
    entry: n > size - 3 ? 'Q' : null,
    rank: n,
  })
  const out: EditionMatch[] = []
  let players = Array.from({ length: size }, (_, i) => i + 1)
  let num = 1
  for (const round of rounds) {
    const next: number[] = []
    for (let i = 0; i < players.length; i += 2) {
      const a = Math.min(players[i] as number, players[i + 1] as number)
      const b = Math.max(players[i] as number, players[i + 1] as number)
      const scores = ['6-4 6-4', '7-6(5) 3-6 6-2', '6-3 6-7(4) 7-5', '6-2 3-0 RET']
      out.push({
        round,
        match_num: num++,
        qualifying: false,
        best_of: 3,
        tie: null,
        players: [side(a), side(b)],
        score: scores[(a + b) % scores.length] as string,
        incomplete: (a + b) % scores.length === 3,
        minutes: null,
        serve: [undefined, undefined],
        charting_id: null,
      })
      next.push(a)
    }
    players = next
  }
  return out.filter((m) => !drop.some(([round, n]) => m.round === round && m.match_num === n))
}

const drawRounds = (size: number): string[] => MAIN_ROUNDS.slice(-Math.log2(size))
const eightDraw = sampleDraw(8, [['QF', 1], ['SF', 2]])
const eightPages = pageSheets(buildBracket(eightDraw, drawRounds(8)), drawRounds(8), 'main')
const thirtyTwoPages = pageSheets(buildBracket(sampleDraw(32), drawRounds(32)), drawRounds(32), 'main')
const bigPages = pageSheets(buildBracket(sampleDraw(128), drawRounds(128)), drawRounds(128), 'main')

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
  const [chartOpen, setChartOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [lit, setLit] = useState<string | null>(null)
  const [round, setRound] = useState('R32')

  return (
    <>
      <PageHeader
        kicker="Inventory"
        title="Components"
        lede="Every component in the inventory, in every state. Not a page anyone visits: a place to check the tokens, the colour and the absence system against the design before a page depends on them."
        art={<CourtArt cycle />}
      />

      <section className={styles.block}>
        <h2 className={styles.name}>Card, Kicker and CountUp</h2>
        <p className={styles.note}>
          White on the cream ground; the player colours go on the figures, not the card.
          Each rises into place the first time it scrolls into view.
        </p>
        <div className={styles.cards}>
          <Card>
            <Kicker>Plain</Kicker>
            <CountUp className={styles.figure} value={2418} />
          </Card>
          <Card>
            <Kicker>Player A</Kicker>
            <CountUp className={`${styles.figure} ${styles.figureA}`} value={56.4} format={(v) => `${v.toFixed(1)}%`} />
          </Card>
          <Card>
            <Kicker>Player B</Kicker>
            <CountUp className={`${styles.figure} ${styles.figureB}`} value={43.6} format={(v) => `${v.toFixed(1)}%`} />
          </Card>
          <Card tilt>
            <Kicker>Tilted</Kicker>
            <CountUp className={styles.figure} value={1624318} format={(v) => Math.round(v).toLocaleString('en-US')} />
          </Card>
        </div>
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SurfaceBadge and FormPills</h2>
        <div className={styles.row}>
          <SurfaceBadge surface="hard" />
          <SurfaceBadge surface="clay" />
          <SurfaceBadge surface="grass" />
          <SurfaceBadge surface="carpet" />
          <SurfaceBadge surface={null} />
        </div>
        <p className={styles.sub}>Last ten, oldest first, for each side</p>
        <div className={styles.row}>
          <FormPills results={[true, true, false, true, true, true, false, true, true, true]} />
          <FormPills side="b" results={[true, false, false, true, true, true, true, false, true, true]} />
        </div>
        <div className={styles.pillsWide}>
          <FormPills size="lg" results={[true, true, true, true, false, true, true, true, true, false]} />
        </div>
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>AreaChart</h2>
        <p className={styles.note}>
          One rating over time: the line draws in, the ground under it takes the side&apos;s
          wash, the peak is ringed and the latest figure dotted. Point at it, or focus it and
          press an arrow, and a crosshair reads out the nearest week.
        </p>
        <AreaChart points={trajectory} label="Elo rating over three seasons" />
        <AreaChart points={trajectory} side="b" height={160} label="The same series as player B" />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>SectionRail</h2>
        <p className={styles.note}>
          A sticky table of contents for a page that runs long. Every entry is an anchor, so a
          section is a link somebody can send, and the one in view is marked. Sticky here too,
          which is why it sits under the site header rather than at the top of the page.
        </p>
        <SectionRail
          label="Example"
          items={[
            { id: 'gallery-rating', label: 'Rating' },
            { id: 'gallery-serve', label: 'Serve and return' },
            { id: 'gallery-matches', label: 'Every match' },
          ]}
        />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>RoundFunnel</h2>
        <p className={styles.note}>
          The record round by round, scaled against the busiest one, so how far a career
          usually got is legible before a number is read. Won is ink and lost is grey; the
          record at the end of each bar is what carries it without colour.
        </p>
        <RoundFunnel
          rounds={[
            { round: 'R128', matches: 78, wins: 74 },
            { round: 'R64', matches: 74, wins: 68 },
            { round: 'R32', matches: 68, wins: 57 },
            { round: 'R16', matches: 57, wins: 44 },
            { round: 'QF', matches: 44, wins: 31 },
            { round: 'SF', matches: 31, wins: 21 },
            { round: 'F', matches: 21, wins: 12 },
          ]}
        />
      </section>

      <section className={styles.block}>
        <h2 className={styles.name}>CourtArt</h2>
        <p className={styles.note}>
          The site&apos;s illustration, per surface. Decorative and hidden from assistive
          technology; the ball stops for anyone who asked for less motion.
        </p>
        <div className={styles.courts}>
          <CourtArt surface="hard" />
          <CourtArt surface="clay" />
          <CourtArt surface="grass" rally={false} />
        </div>
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
        <h2 className={styles.name}>ChartedMark and ChartedSheet</h2>
        <p className={styles.note}>
          The mark sits on a match row the Match Charting Project has charted, and on no
          other row. Opening it puts the per-set sheet under the row: a column per set and
          one for the match, both players in every cell, A in orange and B in turquoise.
          The sheet below reads a real charted match from the API and says so if it cannot.
        </p>
        <p>
          7-6(5) 6-3
          <ChartedMark open={chartOpen} onToggle={() => setChartOpen((o) => !o)} />
        </p>
        {chartOpen ? (
          <ChartedSheet chartingId="20260412-M-Monte_Carlo_Masters-F-Carlos_Alcaraz-Jannik_Sinner" />
        ) : null}
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
        <h2 className={styles.name}>DrawSheet, RoundStepper and RoundList</h2>
        <p className={styles.note}>
          The bracket as the typed sheet: a name above its rule, the pair joined at the
          right, the winner&apos;s rule stepping in from the midpoint, the score under the
          name in the column it earned. An eight draw with a bye and a row the file lacks,
          typed bye and n/r; a 32 draw on one sheet; a 128 draw paged as the printed one is.
          Hover a name and the whole run lights.
        </p>
        {eightPages.map((page, i) => (
          <DrawSheet key={i} page={page} kind="main" lit={lit} onLit={setLit} />
        ))}
        {thirtyTwoPages.map((page, i) => (
          <DrawSheet key={i} page={page} kind="main" lit={lit} onLit={setLit} />
        ))}
        {bigPages.map((page, i) => (
          <div key={i}>
            <h3 className={styles.sub}>{page.title}</h3>
            <DrawSheet page={page} kind="main" lit={lit} onLit={setLit} />
          </div>
        ))}
        <p className={styles.note}>
          Below 880px the same draw is one round at a time, the sheet&apos;s page headings
          kept so the shape survives the list.
        </p>
        <RoundStepper rounds={[...drawRounds(128)]} qualifying={['Q1', 'Q2']} value={round} onChange={setRound} />
        <RoundList
          groups={roundGroups(bigPages, Math.max(1, drawRounds(128).indexOf(round) + 1))}
          final={round === 'F'}
        />
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
