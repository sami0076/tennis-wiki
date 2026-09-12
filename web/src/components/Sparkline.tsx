import styles from './Sparkline.module.css'

export interface SparkPoint {
  date: string
  elo: number
}

interface SparklineProps {
  points: ReadonlyArray<SparkPoint>
  /** The accessible description; a line with no axes needs one. */
  label: string
  width?: number
  height?: number
}

/**
 * Sparkline is a hand-rolled inline SVG: one ink line over the ruling of the
 * sheet, no axes and no labels.
 *
 * Deliberately not a charting library. The multi-series trajectory chart -- top
 * eight, hover readout, sixty years of weeks -- arrives with the ratings
 * endpoints and gets its own decision then. Adding visx now to draw forty
 * points would be paying for it early.
 */
export function Sparkline({ points, label, width = 240, height = 40 }: SparklineProps) {
  if (points.length < 2) return null

  const values = points.map((p) => p.elo)
  const min = Math.min(...values)
  const max = Math.max(...values)
  // A flat series would divide by zero and, worse, draw the line off the top.
  const span = max - min || 1

  const path = points
    .map((point, index) => {
      const x = (index / (points.length - 1)) * width
      const y = height - ((point.elo - min) / span) * height
      return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(' ')

  return (
    <figure className={styles.figure}>
      <svg
        className={styles.svg}
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={label}
      >
        {[0.25, 0.5, 0.75].map((share) => (
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
        <path className={styles.line} d={path} vectorEffect="non-scaling-stroke" />
      </svg>
    </figure>
  )
}
