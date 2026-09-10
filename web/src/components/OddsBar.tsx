import { surfaceVar } from '../lib/surface'
import styles from './OddsBar.module.css'

interface OddsBarProps {
  name: string
  /** Between 0 and 1. */
  probability: number
  /** The 95% half-width, drawn as a whisker around the end of the bar. */
  interval?: number
  /** What the bar is scaled against, so a field of rows is comparable. */
  max: number
  /** The event's surface. The one thing hue is allowed to mean. */
  surface: string | null
}

/**
 * OddsBar is one row of a draw simulation: who, how likely, and how sure.
 *
 * The interval is not optional decoration. These figures come from sampling, so
 * 31.2 without a half-width beside it is a claim the simulation never made.
 */
export function OddsBar({ name, probability, interval = 0, max, surface }: OddsBarProps) {
  const scale = (value: number) => (max > 0 ? Math.min(100, (value / max) * 100) : 0)
  const percent = probability * 100
  const low = Math.max(0, probability - interval)
  const high = Math.min(1, probability + interval)

  const label =
    interval > 0
      ? `${name}: ${percent.toFixed(1)}%, give or take ${(interval * 100).toFixed(1)}`
      : `${name}: ${percent.toFixed(1)}%`

  return (
    <div className={styles.row}>
      <span className={styles.name}>{name}</span>
      <span className={styles.track} role="img" aria-label={label}>
        {interval > 0 ? (
          <span
            className={styles.whisker}
            style={{ left: `${scale(low)}%`, width: `${scale(high) - scale(low)}%` }}
          />
        ) : null}
        <span
          className={styles.fill}
          style={{ width: `${scale(probability)}%`, background: surfaceVar(surface) }}
        />
      </span>
      <span className={styles.figure}>
        {percent.toFixed(1)}
        {interval > 0 ? (
          <span className={styles.interval}> &plusmn;{(interval * 100).toFixed(1)}</span>
        ) : null}
      </span>
    </div>
  )
}
