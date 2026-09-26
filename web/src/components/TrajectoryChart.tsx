import type { SparkPoint } from './Sparkline'
import { surname } from '../lib/format'
import { spread } from '../lib/spread'
import { niceTicks, splitOnGaps, timeTicks } from '../lib/axis'
import { Note } from './Note'
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
  const atValue = (value: number) => height - ((value - min) / spanY) * height

  // One run per stretch the player was actually rated through. A run of one is
  // a week on its own: drawn as a dot, because a path of one point draws
  // nothing at all and the week would silently vanish.
  const path = (points: ReadonlyArray<SparkPoint>) =>
    splitOnGaps(points, (point) => Date.parse(point.date))
      .map((run) =>
        run
          .map((point, index) => `${index === 0 ? 'M' : 'L'}${x(point).toFixed(2)} ${y(point).toFixed(2)}`)
          .join(' '),
      )
      .join(' ')

  const lone = (points: ReadonlyArray<SparkPoint>) =>
    splitOnGaps(points, (point) => Date.parse(point.date)).filter((run) => run.length === 1).flat()

  // The grid sits on round ratings inside the range, and the ends are labelled
  // separately, so a tick landing on top of one is dropped.
  const grid = niceTicks(min, max).filter((tick) => tick > min && tick < max)
  const dates = timeTicks(from, to)
  // The ends say what the data actually reaches, which the round ticks do not.
  // Where an end would sit on top of a tick, the tick wins: two figures a few
  // pixels apart are worse than one.
  const clearOfGrid = (value: number) =>
    grid.every((tick) => Math.abs(atValue(tick) - atValue(value)) / height > 0.07)

  const ordered = [...drawable].sort((a, b) => a.position - b.position)
  const leaders = ordered.slice(0, named)
  const field = ordered.slice(named)
  // The tick labels are aria-hidden, so the description carries both axes.
  const broken = ordered.filter(
    (line) => splitOnGaps(line.points, (point) => Date.parse(point.date)).length > 1,
  ).length
  const label =
    `Elo of the top ${ordered.length}, ${new Date(from).getUTCFullYear()} to ${new Date(to).getUTCFullYear()}, ` +
    `between ${Math.round(min)} and ${Math.round(max)}. ` +
    `Leading: ${leaders.map((line) => line.name).join(', ')}.` +
    (broken > 0 ? ` ${broken} of the lines break where the player went unrated for six months or more.` : '')

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
          {grid.map((tick) => (
            <line
              key={tick}
              className={styles.ruling}
              x1="0"
              y1={atValue(tick)}
              x2={width}
              y2={atValue(tick)}
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
          {/* A week with an absence either side of it: a path of one point
              draws nothing, so it is marked rather than lost. */}
          {leaders.map((line, index) =>
            lone(line.points).map((point) => (
              <path
                key={`${line.name}-${point.date}`}
                className={`${styles.dot} ${rank(index)}`}
                d={`M${x(point).toFixed(2)} ${y(point).toFixed(2)}l0 0`}
                vectorEffect="non-scaling-stroke"
              />
            )),
          )}
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
        {/* Both axes are HTML over the plot, not text inside it: the SVG is
            stretched to the column width with preserveAspectRatio="none",
            which would stretch any glyph drawn in it with the lines. */}
        <span className={styles.axis} aria-hidden="true">
          <span className={styles.unit}>Elo</span>
          {clearOfGrid(max) ? (
            <span className={styles.tick} style={{ top: 0 }}>
              {Math.round(max)}
            </span>
          ) : null}
          {grid.map((tick) => (
            <span key={tick} className={styles.tick} style={{ top: `${(atValue(tick) / height) * 100}%` }}>
              {tick}
            </span>
          ))}
          {clearOfGrid(min) ? (
            <span className={styles.tick} style={{ top: '100%' }}>
              {Math.round(min)}
            </span>
          ) : null}
        </span>
      </div>
      <div className={styles.dates} aria-hidden="true">
        {dates.map((tick) => (
          <span
            key={tick.at}
            className={styles.date}
            style={{ left: `${((tick.at - from) / spanX) * 100}%` }}
          >
            {tick.label}
          </span>
        ))}
      </div>
      {field.length > 0 || broken > 0 ? (
        <Note>
          {[
            field.length > 0 ? `${field.length} more drawn as the field, in grey.` : null,
            'Rated only in the weeks they played',
            broken > 0 ? 'so a line breaks where a player went unrated for six months or more' : null,
          ]
            .filter(Boolean)
            .join(broken > 0 ? ', ' : '. ') + '.'}
        </Note>
      ) : null}
    </figure>
  )
}

/** The ink ramp, in ranking order. Hue is spent on surface and cannot be borrowed. */
function rank(index: number) {
  return [styles.first, styles.second, styles.third][index] ?? styles.field
}
