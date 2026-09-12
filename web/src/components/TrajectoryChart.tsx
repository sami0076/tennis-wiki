import type { SparkPoint } from './Sparkline'
import { surname } from '../lib/format'
import { spread } from '../lib/spread'
import styles from './TrajectoryChart.module.css'

export interface TrajectoryLineData {
  name: string
  /** Where the player stands at the end of the window; 1 is the leader. */
  position: number
  points: ReadonlyArray<SparkPoint>
}

interface TrajectoryChartProps {
  lines: ReadonlyArray<TrajectoryLineData>
  /** How many lines are named. The rest are drawn as the field. */
  named?: number
  /**
   * Draw the lines in on first render. The sheet gives the site one piece of
   * ambient motion and this is it, so nothing else animates.
   */
  animate?: boolean
  width?: number
  height?: number
}

/**
 * TrajectoryChart is the leaders' ratings on one pair of axes, drawn over the
 * ruling of the sheet: the top few in the ink ramp and named at the ends of
 * their lines, everyone else drawn as the field and counted.
 *
 * Still no charting library. Eight polylines over a shared range do not need
 * one, and visx would be 40kB to draw what fifteen lines of arithmetic already
 * draw.
 *
 * TrajectoryPair is the two-player version and stays: it spends ink and pencil
 * on "player A" and "player B", which is a different meaning from the ranking
 * order this one encodes.
 */
export function TrajectoryChart({
  lines,
  named = 3,
  animate = false,
  width = 320,
  height = 140,
}: TrajectoryChartProps) {
  // One point is a dot, not a line. A player rated in a single week of the
  // window has nothing to draw and would otherwise widen the axis for nothing.
  const drawable = lines.filter((line) => line.points.length > 1)
  if (drawable.length === 0) return null

  const all = drawable.flatMap((line) => line.points)
  const times = all.map((point) => Date.parse(point.date))
  const values = all.map((point) => point.elo)
  const from = Math.min(...times)
  const to = Math.max(...times)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const spanX = to - from || 1
  const spanY = max - min || 1

  const x = (point: SparkPoint) => ((Date.parse(point.date) - from) / spanX) * width
  const y = (point: SparkPoint) => height - ((point.elo - min) / spanY) * height
  const path = (points: ReadonlyArray<SparkPoint>) =>
    points
      .map((point, index) => `${index === 0 ? 'M' : 'L'}${x(point).toFixed(2)} ${y(point).toFixed(2)}`)
      .join(' ')

  const ordered = [...drawable].sort((a, b) => a.position - b.position)
  const leaders = ordered.slice(0, named)
  const field = ordered.slice(named)
  const label = `Elo of the top ${ordered.length}, ${new Date(from).getUTCFullYear()} to ${new Date(to).getUTCFullYear()}, between ${Math.round(min)} and ${Math.round(max)}. Leading: ${leaders.map((line) => line.name).join(', ')}.`

  // Each name sits at the end of its line, as a share of the rendered height,
  // and names that would sit on top of each other are pushed apart.
  const tags = spread(
    leaders.map((line) => (y(line.points[line.points.length - 1]!) / height) * 100),
    (16 / height) * 100,
  )

  return (
    <figure className={styles.figure}>
      <div className={styles.plot}>
        <svg
          className={animate ? `${styles.svg} ${styles.animate}` : styles.svg}
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="none"
          role="img"
          aria-label={label}
        >
          {[0, 0.25, 0.5, 0.75].map((share) => (
            <line
              key={share}
              className={styles.ruling}
              x1="0"
              y1={height * share}
              x2={width}
              y2={height * share}
              vectorEffect="non-scaling-stroke"
            />
          ))}
          <line
            className={styles.baseline}
            x1="0"
            y1={height}
            x2={width}
            y2={height}
            vectorEffect="non-scaling-stroke"
          />
          {/* The field first, so a named line is never drawn underneath one. */}
          {field.map((line) => (
            <path
              key={line.name}
              className={styles.field}
              d={path(line.points)}
              pathLength="1"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          {leaders.map((line, index) => (
            <path
              key={line.name}
              className={`${styles.named} ${rank(index)}`}
              d={path(line.points)}
              pathLength="1"
              vectorEffect="non-scaling-stroke"
            />
          ))}
        </svg>
        {leaders.map((line, index) => (
          <span
            key={line.name}
            className={`${styles.tag} ${rank(index)}`}
            style={{ top: `${tags[index]}%` }}
            aria-hidden="true"
          >
            {surname(line.name)}
          </span>
        ))}
        <span className={styles.axis} aria-hidden="true">
          <span>{Math.round(max)}</span>
          <span>{Math.round(min)}</span>
        </span>
      </div>
      <figcaption className={styles.caption}>
        {field.length > 0 ? `${field.length} more drawn as the field. ` : null}
        {Math.round(min)} to {Math.round(max)} Elo. Rated only in the weeks they played.
      </figcaption>
    </figure>
  )
}

/** The ink ramp, in ranking order. Hue is spent on surface and cannot be borrowed. */
function rank(index: number) {
  return [styles.first, styles.second, styles.third][index] ?? styles.field
}
