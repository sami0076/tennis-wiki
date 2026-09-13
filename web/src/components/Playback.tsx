import { useEffect, useMemo, useRef, useState } from 'react'
import type { SimulationChain } from '../api/client'
import {
  announced,
  buildPlayback,
  commentary,
  newSeed,
  type PlaybackEvent,
  type Snapshot,
} from '../lib/playback'
import { surname } from '../lib/format'
import { prefersReducedMotion } from '../lib/useReducedMotion'
import { Button } from './Button'
import { Scoreboard } from './Scoreboard'
import { Tracker } from './Tracker'
import styles from './Playback.module.css'

interface PlaybackProps {
  chain: SimulationChain
  bestOf: number
  players: readonly [{ name: string }, { name: string }]
  surface: string
}

type Phase = 'idle' | 'playing' | 'finished'

/** The line under the board fades out for this long before the next beat lands. */
const FADE_MS = 200

const NONE: ReadonlyArray<PlaybackEvent> = []

/**
 * Playback is one simulated match, a single draw from the odds above: the
 * board, a line of commentary, and a tally, played out beat by beat at the
 * prototype's pace. Started by the reader, never on its own, and rendered
 * finished at once for anyone who asked for less motion.
 */
export function Playback({ chain, bestOf, players, surface }: PlaybackProps) {
  const [seed, setSeed] = useState<number | null>(null)
  const [phase, setPhase] = useState<Phase>('idle')
  const [cursor, setCursor] = useState(-1)
  const [fading, setFading] = useState(false)

  const playback = useMemo(
    () => (seed === null ? null : buildPlayback({ point: chain.point, bestOf }, seed)),
    [chain.point, bestOf, seed],
  )
  const events = playback === null ? NONE : playback.events
  const event: PlaybackEvent | null = cursor >= 0 ? (events[cursor] ?? null) : null

  // Surnames tell the two apart on a board with six-character cells; whole
  // names only when the surnames would not.
  const names = useMemo((): readonly [string, string] => {
    const [a, b] = [surname(players[0].name), surname(players[1].name)]
    return a === b ? [players[0].name, players[1].name] : [a, b]
  }, [players])

  // The beat: the line fades out, then the next state lands and holds for its
  // wait. One timer at a time, cleared on every change, so unmounting or a new
  // pair mid-match leaves nothing ticking.
  useEffect(() => {
    if (phase !== 'playing' || playback === null) return
    const next = cursor + 1
    if (next >= events.length) {
      setPhase('finished')
      return
    }
    let land: ReturnType<typeof setTimeout> | null = null
    const wait = cursor < 0 ? 0 : (events[cursor]?.wait ?? 0)
    const fade = setTimeout(() => {
      setFading(true)
      land = setTimeout(() => {
        setFading(false)
        setCursor(next)
      }, FADE_MS)
    }, wait)
    return () => {
      clearTimeout(fade)
      if (land !== null) clearTimeout(land)
    }
  }, [phase, cursor, playback, events])

  const started = useRef(false)
  useEffect(() => {
    if (seed === null || !started.current) return
    started.current = false
    if (prefersReducedMotion()) {
      setCursor(events.length - 1)
      setPhase('finished')
    } else {
      setCursor(-1)
      setPhase('playing')
    }
  }, [seed, events.length])

  function watch() {
    started.current = true
    setSeed(newSeed())
  }

  const last = events[events.length - 1]
  const shown: Snapshot | null = event?.snapshot ?? (phase === 'playing' ? startingBoard(last) : null)
  const line = event ? commentary(event, names) : ''
  const heard = event && announced(event) ? line : ''
  // The line names a side in that side's hue.
  const coloured = line
    .split(new RegExp(`(${names.map(escape).join('|')})`))
    .map((part, index) =>
      part === names[0] ? (
        <span key={index} className={styles.sideA}>
          {part}
        </span>
      ) : part === names[1] ? (
        <span key={index} className={styles.sideB}>
          {part}
        </span>
      ) : (
        part
      ),
    )

  return (
    <div className={styles.playback}>
      <h2 className={styles.title}>One simulated match</h2>
      <p className={styles.lede}>One draw from the odds above. A sample, not a prediction.</p>

      {shown ? (
        <>
          <Scoreboard snapshot={shown} names={names} surface={surface} playing={phase === 'playing'} />
          <p className={fading ? `${styles.line} ${styles.faded}` : styles.line}>{coloured}</p>
          <p className="sr-only" aria-live="polite">
            {heard}
          </p>
          <Tracker snapshot={shown} />
        </>
      ) : null}

      <div className={styles.controls}>
        <Button onClick={watch} disabled={phase === 'playing'}>
          {phase === 'idle' ? 'Watch a simulated match' : phase === 'playing' ? 'Playing…' : 'Watch another'}
        </Button>
      </div>
    </div>
  )
}

/** A name as a regex literal: O'Brien and a dotted initial must match themselves. */
function escape(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/** The empty board a match starts from: nothing played, A to serve. */
function startingBoard(last: PlaybackEvent | undefined): Snapshot | null {
  if (last === undefined) return null
  return {
    sets: [],
    current: { a: 0, b: 0 },
    setsWon: [0, 0],
    points: [0, 0],
    breakPointsWon: [0, 0],
    breakPointsFaced: [0, 0],
    holds: [0, 0],
    server: 0,
    flash: null,
    pop: null,
  }
}
