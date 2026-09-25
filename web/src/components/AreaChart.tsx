import { useId, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import type { SparkPoint } from './Sparkline'
import { useInView } from '../lib/useInView'
import styles from './AreaChart.module.css'

interface AreaChartProps {
  points: ReadonlyArray<SparkPoint>
  label: string
  side?: 'a' | 'b'
  height?: number
  /**
   * What one point is, for the readout: "Elo 2145" needs the word. Defaults to
   * Elo because every chart on the site currently plots one.
   */
  unit?: string
}

const W = 800
const PAD_TOP = 34
const PAD_BOTTOM = 8

/**
 * AreaChart is one rating over time: the line drawn in, the ground under it
 * washed in the player's colour, the peak ringed and the latest figure dotted.
 *
 * It reads as well as it draws. A career line whose only legible values are its
 * two ends is a picture of a career, not a record of one, so pointing at it
 * puts a crosshair on the nearest week and writes that week's date and rating
 * over the line. The same readout answers to the arrow keys, because the
 * question "what was he rated in 2016" should not need a mouse.
 */
export function AreaChart({ points, label, side, height = 240, unit = 'Elo' }: AreaChartProps) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const gradient = useId()
  const plotRef = useRef<HTMLDivElement>(null)
  // The week under the crosshair, or null when nothing is pointing at it.
  const [at, setAt] = useState<number | null>(null)

  const geometry = useMemo(() => {
    const times = points.map((p) => Date.parse(p.date))
    const values = points.map((p) => p.elo)
    return {
      times,
      from: times[0] ?? 0,
      to: times[times.length - 1] ?? 1,
      min: Math.min(...values),
      max: Math.max(...values),
      peakIndex: values.indexOf(Math.max(...values)),
    }
  }, [points])

  if (points.length < 2) return null

  const { times, from, to, min, max, peakIndex } = geometry
  const spanX = to - from || 1
  const spanY = max - min || 1
  const plot = height - PAD_TOP - PAD_BOTTOM

  const x = (t: number) => ((t - from) / spanX) * W
  const y = (v: number) => PAD_TOP + plot - ((v - min) / spanY) * plot
  const line = points
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(times[i]!).toFixed(1)} ${y(p.elo).toFixed(1)}`)
    .join('')
  const area = `${line}L${W} ${height}L0 ${height}Z`

  const peak = points[peakIndex]!
  const last = points[points.length - 1]!
  const peakIsLast = peakIndex === points.length - 1

  const firstYear = new Date(from).getUTCFullYear()
  const lastYear = new Date(to).getUTCFullYear()
  const step = Math.max(1, Math.ceil((lastYear - firstYear + 1) / 6))
  const years: number[] = []
  for (let year = firstYear + 1; year <= lastYear; year += step) years.push(year)

  const pct = (value: number, of: number) => `${(value / of) * 100}%`

  /** The point whose date is closest to a fraction across the plot. */
  function nearest(fraction: number): number {
    const target = from + fraction * spanX
    let best = 0
    let bestGap = Infinity
    for (let i = 0; i < times.length; i += 1) {
      const gap = Math.abs(times[i]! - target)
      if (gap < bestGap) {
        best = i
        bestGap = gap
      }
    }
    return best
  }

  function onPointer(event: PointerEvent<HTMLDivElement>) {
    const box = plotRef.current?.getBoundingClientRect()
    if (box === undefined || box.width === 0) return
    setAt(nearest((event.clientX - box.left) / box.width))
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const current = at ?? points.length - 1
    switch (event.key) {
      case 'ArrowRight':
        event.preventDefault()
        setAt(Math.min(points.length - 1, current + 1))
        return
      case 'ArrowLeft':
        event.preventDefault()
        setAt(Math.max(0, current - 1))
        return
      case 'Home':
        event.preventDefault()
        setAt(0)
        return
      case 'End':
        event.preventDefault()
        setAt(points.length - 1)
        return
      case 'Escape':
        setAt(null)
        return
      default:
        return
    }
  }

  const reading = at === null ? null : points[at]
  const readingX = reading === undefined || reading === null ? 0 : x(times[at!]!)

  return (
    <div
      ref={ref}
      className={[styles.chart, side ? styles[side] : '', seen ? styles.seen : ''].join(' ')}
    >
      <div
        ref={plotRef}
        className={styles.plot}
        style={{ height }}
        // A figure rather than an image: it carries a value a reader can move
        // through, and the label says so.
        role="figure"
        tabIndex={0}
        aria-label={`${label}. Use the arrow keys to read a week.`}
        onPointerMove={onPointer}
        onPointerDown={onPointer}
        onPointerLeave={() => setAt(null)}
        onFocus={() => setAt((current) => current ?? points.length - 1)}
        onBlur={() => setAt(null)}
        onKeyDown={onKeyDown}
      >
        <svg viewBox={`0 0 ${W} ${height}`} preserveAspectRatio="none" aria-hidden="true">
          <defs>
            <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="currentColor" stopOpacity="0.22" />
              <stop offset="100%" stopColor="currentColor" stopOpacity="0.02" />
            </linearGradient>
          </defs>
          <path className={styles.area} d={area} fill={`url(#${gradient})`} />
          <path className={styles.line} d={line} pathLength={1} />
          {reading === null || reading === undefined ? null : (
            <line
              className={styles.crosshair}
              x1={readingX}
              y1={PAD_TOP - 8}
              x2={readingX}
              y2={height}
            />
          )}
        </svg>

        {peakIsLast || reading !== null ? null : (
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

        {reading === null || reading === undefined ? null : (
          <>
            <span
              className={styles.marker}
              style={{ left: pct(readingX, W), top: pct(y(reading.elo), height) }}
              aria-hidden="true"
            />
            <span
              className={styles.readout}
              // Clamped so the card never has to scroll sideways to show a
              // readout taken near either edge of the plot.
              style={{ left: `clamp(0%, ${(readingX / W) * 100}%, 100%)` }}
              data-side={readingX > W * 0.7 ? 'left' : 'right'}
            >
              <span className={styles.readoutValue}>
                {unit} {Math.round(reading.elo)}
              </span>
              <span className={styles.readoutDate}>{reading.date}</span>
            </span>
          </>
        )}
      </div>

      {/* The readout is drawn; this is the same fact, announced. */}
      <p className="sr-only" aria-live="polite">
        {reading === null || reading === undefined
          ? ''
          : `${reading.date}: ${unit} ${Math.round(reading.elo)}`}
      </p>

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
