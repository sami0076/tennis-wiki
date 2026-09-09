import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  ApiError,
  type DrawSimulation,
  type MatchSimulation,
  type PlayerSearchResult,
  type SimulationChain,
} from '../api/client'
import { simulateDraw, simulateMatch } from '../api/endpoints'
import { useResource, type Resource } from '../api/useResource'
import {
  ButtonLink,
  EmptyState,
  Meta,
  OddsBar,
  PlayerSearch,
  Skeleton,
  SurfaceToggle,
  WinSplit,
} from '../components'
import { formatPercent } from '../lib/format'
import { surfaceLabel } from '../lib/surface'
import { useUrlParam } from '../lib/useUrlParam'
import styles from './Simulator.module.css'

/**
 * The default draw. There is no upcoming tournament in this database and never
 * will be, so the panel opens on a played one -- and on one whose answer a
 * reader can check, which is the whole advantage of simulating the past.
 */
const defaultDraw = { tour: 'atp', season: 2019, event: 'Wimbledon' }

const SURFACES = ['hard', 'clay', 'grass', 'carpet']

/**
 * The simulator: two players, every rung between a point and a match, and a
 * draw played ten thousand times.
 *
 * The chain is the page. A single win probability would be a number to take on
 * trust; the rungs are what show the scoring system doing the amplifying.
 */
export function Simulator() {
  const [a, setA] = useUrlParam('a')
  const [b, setB] = useUrlParam('b')
  const [surface, setSurface] = useUrlParam('surface')
  const [bestOf, setBestOf] = useUrlParam('best_of')

  const chosen = surface ?? 'hard'
  const sets = bestOf === '5' ? 5 : 3

  const match = useResource<MatchSimulation | null>(
    (signal) =>
      a === null || b === null
        ? Promise.resolve(null)
        : simulateMatch(a, b, { surface: chosen, best_of: sets }, signal),
    [a, b, chosen, sets],
  )
  const draw = useResource((signal) => simulateDraw(defaultDraw, signal), [])

  return (
    <>
      <h1 className={styles.title}>Simulator</h1>
      <p className={styles.standfirst}>
        From first principles: a probability per service point, compounded by the scoring
        system into a probability per match. Every step is shown, because the compounding is
        the interesting part.
      </p>

      <Pickers a={a} b={b} onA={setA} onB={setB} match={match} />

      <div className={styles.controls}>
        <SurfaceToggle value={surface} onChange={setSurface} options={SURFACES} />
        <div className={styles.format} role="group" aria-label="Match length">
          {[3, 5].map((n) => (
            <button
              key={n}
              type="button"
              className={sets === n ? `${styles.option} ${styles.active}` : styles.option}
              aria-pressed={sets === n}
              onClick={() => setBestOf(n === 3 ? null : '5')}
            >
              Best of {n}
            </button>
          ))}
        </div>
      </div>

      {a === null || b === null ? (
        <EmptyState
          heading="Pick two players"
          reason="Any two the model has a rating for, which is 75,021 of them. The simulation runs from those ratings rather than from serve statistics, which only 3% of players have."
          action={<ButtonLink to="/players">Browse players</ButtonLink>}
        />
      ) : (
        <MatchPanel match={match} surface={chosen} sets={sets} />
      )}

      <DrawPanel draw={draw} />
    </>
  )
}

function Pickers({
  a,
  b,
  onA,
  onB,
  match,
}: {
  a: string | null
  b: string | null
  onA: (v: string | null) => void
  onB: (v: string | null) => void
  match: Resource<MatchSimulation | null>
}) {
  const [queryA, setQueryA] = useState('')
  const [queryB, setQueryB] = useState('')
  const named = match.state === 'ready' && match.data !== null ? match.data.players : undefined

  // The box shows who is being simulated once that is known, and what was
  // typed until then.
  const value = (side: 0 | 1, typed: string) =>
    typed !== '' ? typed : (named?.[side].name ?? '')

  return (
    <div className={styles.pickers}>
      <PlayerSearch
        label="First player"
        placeholder="Search by name"
        value={value(0, queryA)}
        onChange={setQueryA}
        onSelect={(p: PlayerSearchResult) => {
          setQueryA('')
          onA(p.slug)
        }}
      />
      <PlayerSearch
        label="Second player"
        placeholder="Search by name"
        value={value(1, queryB)}
        onChange={setQueryB}
        onSelect={(p: PlayerSearchResult) => {
          setQueryB('')
          onB(p.slug)
        }}
      />
      {a !== null && b !== null && a === b ? (
        <p className={styles.note}>Pick two different players.</p>
      ) : null}
    </div>
  )
}

function MatchPanel({
  match,
  surface,
  sets,
}: {
  match: Resource<MatchSimulation | null>
  surface: string
  sets: number
}) {
  if (match.state === 'loading') return <Skeleton lines={6} />

  if (match.state === 'error') {
    const notFound = match.error instanceof ApiError && match.error.status === 404
    return (
      <EmptyState
        heading={notFound ? 'One of those players does not exist' : 'That simulation failed'}
        reason={match.error.message}
        action={<ButtonLink to="/players">Search for a player</ButtonLink>}
      />
    )
  }
  if (match.data === null) return null

  const sim = match.data
  const [playerA, playerB] = sim.players

  if (sim.chain === null) {
    const missing = playerA.elo === null ? playerA.name : playerB.name
    return (
      <EmptyState
        heading="This pair cannot be simulated"
        reason={
          sim.availability === 'unrated'
            ? `${missing} has no rating, so there is nothing to derive a point probability from. A rating needs matches this database holds, and most of its 115,000 players have too few.`
            : 'No serve statistics exist for this tour, so there is no average to anchor the derivation on.'
        }
        action={<ButtonLink to={`/players/${playerA.slug}`}>See {playerA.name}</ButtonLink>}
      />
    )
  }

  return (
    <section className={styles.section}>
      <Meta
        parts={[
          'Match simulator',
          surfaceLabel(surface).toLowerCase(),
          sets === 5 ? 'best of five' : 'best of three',
        ]}
      />
      <WinSplit nameA={playerA.name} nameB={playerB.name} share={sim.chain.match[0]} />
      <Chain chain={sim.chain} />
      <Amplification chain={sim.chain} />
      <Inputs sim={sim} />
    </section>
  )
}

const rungs: ReadonlyArray<{ key: keyof SimulationChain; label: string }> = [
  { key: 'point', label: 'Point, on serve' },
  { key: 'hold', label: 'Hold' },
  { key: 'set', label: 'Set' },
  { key: 'match', label: 'Match' },
]

function Chain({ chain }: { chain: SimulationChain }) {
  return (
    <div className={styles.chain}>
      <h2 className={styles.sectionTitle}>How the edge compounds</h2>
      {rungs.map(({ key, label }) => (
        <div key={key} className={styles.rung}>
          <span className={styles.rungLabel}>{label}</span>
          <span className={styles.rungValue}>
            {formatPercent(chain[key][0] * 100)}
            <span className={styles.dot}> &middot; </span>
            {formatPercent(chain[key][1] * 100)}
          </span>
        </div>
      ))}
    </div>
  )
}

/**
 * The caption the whole page is built around, with its own numbers in it rather
 * than the design's.
 */
function Amplification({ chain }: { chain: SimulationChain }) {
  const point = Math.abs(chain.point[0] - chain.point[1]) * 100
  const match = Math.abs(chain.match[0] - chain.match[1]) * 100

  if (point < 0.05) {
    return (
      <p className={styles.caption}>
        Two players this evenly matched stay even all the way up. The scoring system
        amplifies a difference; it does not invent one.
      </p>
    )
  }
  return (
    <p className={styles.caption}>
      A {point.toFixed(0)}-point edge on serve becomes a {match.toFixed(0)}-point edge on the
      match. Tennis scoring is an amplifier.
    </p>
  )
}

/** Where the numbers came from, which ADR-0007 requires the page to say. */
function Inputs({ sim }: { sim: MatchSimulation }) {
  const { inputs } = sim
  const [playerA, playerB] = sim.players

  return (
    <p className={styles.caption}>
      Point probabilities are derived from the ratings rather than measured: {playerA.name}{' '}
      {playerA.elo === null ? 'unrated' : Math.round(playerA.elo)} against {playerB.name}{' '}
      {playerB.elo === null ? 'unrated' : Math.round(playerB.elo)}, blended{' '}
      {Math.round((playerA.surface_weight ?? 0) * 100)}% toward the surface. Anchored on{' '}
      {inputs.anchor === null ? 'no measured average' : formatPercent(inputs.anchor * 100)} of
      service points won across {inputs.tier} level
      {inputs.anchor_scope === 'tier_surface_decade' ? ` in the ${inputs.decade}s` : ''}, over{' '}
      {inputs.anchor_points} recorded points.
    </p>
  )
}

function DrawPanel({ draw }: { draw: Resource<DrawSimulation> }) {
  if (draw.state === 'loading') {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Draw simulator</h2>
        <Skeleton lines={8} />
      </section>
    )
  }
  if (draw.state === 'error') {
    return (
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Draw simulator</h2>
        <p className={styles.note}>
          The draw simulation could not be loaded: {draw.error.message}
        </p>
      </section>
    )
  }

  const sim = draw.data
  // The eight the design shows, and everyone else as one row. A 128-draw list
  // in full would bury the answer in ninety rows of rounding error.
  const shown = sim.odds.slice(0, 8)
  const rest = sim.odds.slice(8).reduce((sum, o) => sum + o.title, 0)
  const max = shown[0]?.title ?? 1

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Draw simulator</h2>
      <Meta
        parts={[
          `${sim.event.name} ${sim.event.season}`,
          `${sim.entered} draw`,
          `${sim.runs.toLocaleString('en-GB')} runs`,
        ]}
      />
      {shown.map((o) => (
        <OddsBar
          key={o.slug}
          name={o.name}
          probability={o.title}
          interval={o.title_interval}
          max={max}
          surface={sim.event.surface}
        />
      ))}
      {rest > 0 ? (
        <OddsBar name="The field" probability={rest} max={max} surface={null} />
      ) : null}
      <p className={styles.caption}>
        This draw was played. The ratings are as of {sim.event.ratings_as_of}, the week it
        began, so the simulation knows only what was known then
        {sim.champion === null ? null : (
          <>
            {' '}
            &mdash; and it can be marked: <Link to={`/players/${sim.champion}`}>the player
            who actually won it</Link> is in the list above.
          </>
        )}
        . Every figure is one sample of ten thousand, so each carries the interval it earned.
      </p>
    </section>
  )
}
