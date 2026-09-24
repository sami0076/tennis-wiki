import { useId } from 'react'
import type { SparkPoint } from './Sparkline'
import { useInView } from '../lib/useInView'
import styles from './AreaChart.module.css'

interface AreaChartProps {
  points: ReadonlyArray<SparkPoint>
  label: string
  side?: 'a' | 'b'
  height?: number
}

const W = 800
const PAD_TOP = 34
const PAD_BOTTOM = 8

/**
 * AreaChart is one rating over time: the line drawn in, the ground under it
 * washed in the player's colour, the peak ringed and the latest figure dotted.
 */
export function AreaChart({ points, label, side, height = 240 }: AreaChartProps) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const gradient = useId()
  if (points.length < 2) return null

  const times = points.map((p) => Date.parse(p.date))
  const values = points.map((p) => p.elo)
  const from = times[0]!
  const to = times[times.length - 1]!
  const min = Math.min(...values)
  const max = Math.max(...values)
  const spanX = to - from || 1
  const spanY = max - min || 1
  const plot = height - PAD_TOP - PAD_BOTTOM

  const x = (t: number) => ((t - from) / spanX) * W
  const y = (v: number) => PAD_TOP + plot - ((v - min) / spanY) * plot
  const line = points
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(times[i]!).toFixed(1)} ${y(p.elo).toFixed(1)}`)
    .join('')
  const area = `${line}L${W} ${height}L0 ${height}Z`

  const peakIndex = values.indexOf(max)
  const peak = points[peakIndex]!
  const last = points[points.length - 1]!
  const peakIsLast = peakIndex === points.length - 1

  const firstYear = new Date(from).getUTCFullYear()
  const lastYear = new Date(to).getUTCFullYear()
  const step = Math.max(1, Math.ceil((lastYear - firstYear + 1) / 6))
  const years: number[] = []
  for (let year = firstYear + 1; year <= lastYear; year += step) years.push(year)

  const pct = (value: number, of: number) => `${(value / of) * 100}%`

  return (
    <div ref={ref} className={[styles.chart, side ? styles[side] : '', seen ? styles.seen : ''].join(' ')}>
      <div className={styles.plot} style={{ height }}>
        <svg viewBox={`0 0 ${W} ${height}`} preserveAspectRatio="none" role="img" aria-label={label}>
          <defs>
            <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="currentColor" stopOpacity="0.22" />
              <stop offset="100%" stopColor="currentColor" stopOpacity="0.02" />
            </linearGradient>
          </defs>
          <path className={styles.area} d={area} fill={`url(#${gradient})`} />
          <path className={styles.line} d={line} pathLength={1} />
        </svg>
        {peakIsLast ? null : (
          <span
            className={styles.peak}
            style={{ left: pct(x(Date.parse(peak.date)), W), top: pct(y(peak.elo), height) }}
            aria-hidden="true"
          >
            <span className={styles.peakLabel}>Peak {Math.round(peak.elo)}</span>
          </span>
        )}
        <span
          className={styles.end}
          style={{ left: '100%', top: pct(y(last.elo), height) }}
          aria-hidden="true"
        >
          <span className={styles.endLabel}>{Math.round(last.elo)}</span>
        </span>
      </div>
      <div className={styles.years} aria-hidden="true">
        {years.map((year) => (
          <span key={year} style={{ left: pct(x(Date.UTC(year, 0, 1)), W) }}>
            {year}
          </span>
        ))}
      </div>
    </div>
  )
}
