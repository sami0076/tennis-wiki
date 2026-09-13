import { useEffect, useState } from 'react'
import { scorelines } from '../lib/playback'
import { prefersReducedMotion } from '../lib/useReducedMotion'
import styles from './Scorelines.module.css'

interface ScorelinesProps {
  /** A's chance of a set, from the chain. */
  setShare: number
  bestOf: number
  nameA: string
  nameB: string
  /** Grow the bars in when the result lands, after the split and the rungs. */
  animate?: boolean
}

/** The bars start growing this long after the result arrives, as the prototype has it. */
const GROW_DELAY_MS = 300

/**
 * Scorelines is the chance of each set score, most likely first, A's sets
 * written first the way the split above reads. A score A wins is ink, one B
 * wins is pencil: the same pair as everywhere else.
 */
export function Scorelines({ setShare, bestOf, nameA, nameB, animate = false }: ScorelinesProps) {
  const rows = scorelines(setShare, bestOf)
  const max = rows[0]?.p ?? 1
  const grown = useGrown(animate)

  return (
    <div className={styles.scorelines}>
      <h2 className={styles.title}>How it ends</h2>
      <ol className={styles.rows}>
        {rows.map((row) => {
          const percent = (row.p * 100).toFixed(1)
          return (
            <li
              key={`${row.aWins ? 'a' : 'b'}-${row.score}`}
              className={styles.row}
              aria-label={`${row.aWins ? nameA : nameB} wins ${row.score}: ${percent}%`}
            >
              <span className={row.aWins ? styles.scoreA : styles.scoreB}>{row.score}</span>
              <span className={styles.track} aria-hidden="true">
                <span
                  className={`${row.aWins ? styles.fillA : styles.fillB} ${animate ? styles.growing : ''}`}
                  style={{ width: `${(96 * row.p) / max}%`, transform: grown ? 'none' : 'scaleX(0)' }}
                />
              </span>
              <span className={row.aWins ? styles.figureA : styles.figureB}>{percent}%</span>
            </li>
          )
        })}
      </ol>
      <p className={styles.caption}>
        {nameA} first. Sets modelled as independent.
      </p>
    </div>
  )
}

/** Bars at full width at once, or from nothing after the reveal's delay. */
function useGrown(animate: boolean): boolean {
  const [grown, setGrown] = useState(() => !animate || prefersReducedMotion())
  useEffect(() => {
    if (!animate || prefersReducedMotion()) {
      setGrown(true)
      return
    }
    setGrown(false)
    const timer = setTimeout(() => setGrown(true), GROW_DELAY_MS)
    return () => clearTimeout(timer)
  }, [animate])
  return grown
}
