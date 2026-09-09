import type { SparkPoint } from './Sparkline'
import styles from './TrajectoryPair.module.css'

export interface TrajectorySeries {
  name: string
  points: ReadonlyArray<SparkPoint>
}

interface TrajectoryPairProps {
  a: TrajectorySeries
  b: TrajectorySeries
  width?: number
  height?: number
}

/**
 * TrajectoryPair draws two players' ratings on one axis: A in --ink, B in
 * --ink-3, the same monochrome pair every other comparison uses.
 *
 * One axis is the whole point. Two sparklines side by side would each scale to
 * their own range, so the player who is 200 points behind would draw the same
 * line as the one who is ahead. Both the date range and the rating range are
 * shared, which is what makes crossing lines mean a lead changing hands.
 *
 * Still hand-rolled SVG and still no library: two series over a few hundred
 * weekly points does not need one.
 */
export function TrajectoryPair({ a, b, width = 320, height = 120 }: TrajectoryPairProps) {
  const drawable = [a, b].filter((series) => series.points.length > 1)
  if (drawable.length === 0) return null

  const all = drawable.flatMap((series) => series.points)
  const times = all.map((point) => Date.parse(point.date))
  const values = all.map((point) => point.elo)
  const from = Math.min(...times)
  const to = Math.max(...times)
  const min = Math.min(...values)
  const max = Math.max(...values)
  // A pair whose ratings never moved would divide by zero, and a single week
  // of history gives the same flat span.
  const spanX = to - from || 1
  const spanY = max - min || 1

  const path = (points: ReadonlyArray<SparkPoint>) =>
    points
      .map((point, index) => {
        const x = ((Date.parse(point.date) - from) / spanX) * width
        const y = height - ((point.elo - min) / spanY) * height
        return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`
      })
      .join(' ')

  const label = `${a.name} and ${b.name}, Elo from ${new Date(from).toISOString().slice(0, 10)} to ${new Date(to).toISOString().slice(0, 10)}, between ${Math.round(min)} and ${Math.round(max)}`

  return (
    <figure className={styles.figure}>
      <svg
        className={styles.svg}
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={label}
      >
        <line className={styles.baseline} x1="0" y1={height} x2={width} y2={height} />
        {b.points.length > 1 ? (
          <path className={styles.lineB} d={path(b.points)} vectorEffect="non-scaling-stroke" />
        ) : null}
        {a.points.length > 1 ? (
          <path className={styles.lineA} d={path(a.points)} vectorEffect="non-scaling-stroke" />
        ) : null}
      </svg>
      <figcaption className={styles.caption}>
        <span className={styles.keyA} aria-hidden="true" /> {a.name}
        <span className={styles.gap}> </span>
        <span className={styles.keyB} aria-hidden="true" /> {b.name}
        {' · '}
        {Math.round(min)} to {Math.round(max)} Elo, {new Date(from).getUTCFullYear()} to{' '}
        {new Date(to).getUTCFullYear()}. Rated only in the weeks they played.
      </figcaption>
    </figure>
  )
}
