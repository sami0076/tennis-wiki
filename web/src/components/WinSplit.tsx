import { useEffect, useState } from 'react'
import { prefersReducedMotion } from '../lib/useReducedMotion'
import styles from './WinSplit.module.css'

interface WinSplitProps {
  nameA: string
  nameB: string
  /** A's share, between 0 and 1. B's is the rest. */
  share: number
  /**
   * Count the figures up from an even split when the share arrives, the bar
   * easing with them. The result of a choice the reader made, not ambient.
   */
  animate?: boolean
}

/** How long the figures take to reach the share, as the prototype has it. */
const COUNT_MS = 700

/**
 * WinSplit is the headline of a simulation: two names, two percentages, and one
 * bar divided between them.
 *
 * Monochrome, like every other two-player comparison on this site. Hue means
 * surface, and the simulator page already spends it on the surface control and
 * the draw odds.
 */
export function WinSplit({ nameA, nameB, share, animate = false }: WinSplitProps) {
  const a = Math.round(share * 1000) / 10
  const b = Math.round((1 - share) * 1000) / 10
  const shown = useCountUp(a, animate)

  return (
    <div>
      <div className={styles.split}>
        <span className={styles.nameA}>
          {nameA} <span className={styles.share}>{shown.toFixed(1)}%</span>
        </span>
        <span className={styles.nameB}>
          <span className={styles.share}>{(100 - shown).toFixed(1)}%</span> {nameB}
        </span>
      </div>
      <div
        className={styles.track}
        role="img"
        aria-label={`${nameA} ${a.toFixed(1)}%, ${nameB} ${b.toFixed(1)}%`}
      >
        <div className={styles.fill} style={{ width: `${shown}%` }} />
      </div>
    </div>
  )
}

/**
 * The displayed figure: 50.0 rising to the target over COUNT_MS on an
 * ease-out cubic, and the target itself the moment motion is not wanted. The
 * last frame writes the exact target, so what stays on the page is the API's
 * own figure.
 */
function useCountUp(target: number, animate: boolean): number {
  const [shown, setShown] = useState(() => (animate && !prefersReducedMotion() ? 50 : target))
  useEffect(() => {
    if (!animate || prefersReducedMotion() || typeof requestAnimationFrame !== 'function') {
      setShown(target)
      return
    }
    setShown(50)
    let frame = 0
    let start: number | null = null
    const step = (now: number) => {
      if (start === null) start = now
      const f = Math.min(1, (now - start) / COUNT_MS)
      const eased = 1 - (1 - f) ** 3
      setShown(f >= 1 ? target : 50 + (target - 50) * eased)
      if (f < 1) frame = requestAnimationFrame(step)
    }
    frame = requestAnimationFrame(step)
    return () => cancelAnimationFrame(frame)
  }, [target, animate])
  return shown
}
