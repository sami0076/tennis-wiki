import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import type { RankingRow } from '../api/client'
import { spread } from '../lib/spread'
import { RankDelta } from './RankDelta'
import type { SparkPoint } from './Sparkline'
import styles from './SeedingSheet.module.css'

export interface SeedLine {
  slug: string
  name: string
  points: ReadonlyArray<SparkPoint>
}

interface SeedingSheetProps {
  /** The leaders' rating lines over the window. */
  lines: ReadonlyArray<SeedLine>
  /** The leaders as ranked, in order; one row of the sheet each. */
  seeds: ReadonlyArray<RankingRow>
  /** Draw the lines in on first render: the sheet's one piece of motion. */
  animate?: boolean
  /** The chart's coordinate width. Height is fixed by the rows. */
  width?: number
}

const ROW = 40
const NAMED = 3
/** The lines draw for 900ms, the steps reach across from 700ms for 420ms. */
const SETTLE_MS = 1400

/**
 * SeedingSheet is the first thing on the sheet: the leaders' form lines on the
 * left, the seeding list on the right, and a ruled step joining each line's end
 * to its seed's row, the way a draw sheet's lines step a name into the next
 * round. The rows are the labels, so the chart carries no legend.
 *
 * On a phone the two stack and the top three lines carry their seed number at
 * their ends instead, because a step across a page break joins nothing.
 */
export function SeedingSheet({ lines, seeds, animate = false, width = 640 }: SeedingSheetProps) {
  const [lit, setLit] = useState<string | null>(null)
  // Once the sheet has drawn in, the animation class comes off: what is left is
  // the finished sheet, with no filled-forward animation for a resize or a
  // capture to trip over.
  const [settled, setSettled] = useState(false)
  useEffect(() => {
    if (!animate) return
    const timer = setTimeout(() => setSettled(true), SETTLE_MS)
    return () => clearTimeout(timer)
  }, [animate])
  const drawing = animate && !settled
  const height = seeds.length * ROW

  const drawable = lines.filter((line) => line.points.length > 1)
  const all = drawable.flatMap((line) => line.points)
  const times = all.map((point) => Date.parse(point.date))
  const values = all.map((point) => point.elo)
  const from = Math.min(...times)
  const to = Math.max(...times)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const spanX = to - from || 1
  const spanY = max - min || 1

  // The ruling leaves a row of room above and below the lines so a line at the
  // edge of the range does not sit on a rule.
  const inset = ROW / 2
  const x = (point: SparkPoint) => ((Date.parse(point.date) - from) / spanX) * width
  const y = (point: SparkPoint) => inset + (height - 2 * inset) * (1 - (point.elo - min) / spanY)
  const path = (points: ReadonlyArray<SparkPoint>) =>
    points
      .map((point, index) => `${index === 0 ? 'M' : 'L'}${x(point).toFixed(2)} ${y(point).toFixed(2)}`)
      .join(' ')

  const bySlug = new Map(drawable.map((line) => [line.slug, line]))
  const ranked = seeds.map((seed, index) => {
    const line = bySlug.get(seed.slug) ?? null
    const last = line ? line.points[line.points.length - 1]! : null
    return {
      seed,
      line,
      index,
      endX: last ? x(last) : null,
      endY: last ? y(last) : null,
      rowY: index * ROW + (ROW - 1) / 2,
    }
  })

  const named = ranked.filter((r) => r.line !== null).slice(0, NAMED)
  const tags = spread(
    named.map((r) => ((r.endY ?? 0) / height) * 100),
    (16 / height) * 100,
  )

  const label = `Elo of the top ${ranked.length}, ${new Date(from).getUTCFullYear()} to ${new Date(to).getUTCFullYear()}, between ${Math.round(min)} and ${Math.round(max)}. Leading: ${named.map((r) => r.seed.name).join(', ')}.`
  const tone = (slug: string, index: number) => {
    const base = index === 0 ? styles.first : index === 1 ? styles.second : index === 2 ? styles.third : styles.field
    if (lit === null) return base
    return lit === slug ? `${base} ${styles.lit}` : `${base} ${styles.dim}`
  }

  return (
    <div className={drawing ? `${styles.sheet} ${styles.animate}` : styles.sheet}>
      <div className={styles.plot} style={{ height }}>
        <svg
          className={styles.chart}
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="none"
          role="img"
          aria-label={label}
        >
          {ranked.map((r) => (
            <line
              key={r.seed.slug}
              className={styles.ruling}
              x1="0"
              y1={r.index * ROW + ROW - 0.5}
              x2={width}
              y2={r.index * ROW + ROW - 0.5}
              vectorEffect="non-scaling-stroke"
            />
          ))}
          {/* The field first, so a named line is never drawn underneath one. */}
          {[...ranked].reverse().map((r) =>
            r.line === null || r.endX === null || r.endY === null ? null : (
              <g key={r.seed.slug} className={tone(r.seed.slug, r.index)}>
                <path
                  className={styles.line}
                  d={path(r.line.points)}
                  pathLength="1"
                  vectorEffect="non-scaling-stroke"
                />
                {r.endX < width - 1 ? (
                  <line
                    className={styles.lead}
                    x1={r.endX}
                    y1={r.endY}
                    x2={width}
                    y2={r.endY}
                    vectorEffect="non-scaling-stroke"
                  />
                ) : null}
              </g>
            ),
          )}
        </svg>
        {named.map((r, i) => (
          <span
            key={r.seed.slug}
            className={`${styles.tag} ${tone(r.seed.slug, r.index)}`}
            style={{ top: `${tags[i]}%` }}
            aria-hidden="true"
          >
            [{r.seed.position}]
          </span>
        ))}
        <span className={styles.axis} aria-hidden="true">
          <span>{Math.round(max)}</span>
          <span>{Math.round(min)}</span>
        </span>
      </div>

      <svg
        className={styles.steps}
        viewBox={`0 0 40 ${height}`}
        preserveAspectRatio="none"
        aria-hidden="true"
        style={{ height }}
      >
        {ranked.map((r) =>
          r.endY === null ? null : (
            <path
              key={r.seed.slug}
              className={`${styles.step} ${tone(r.seed.slug, r.index)}`}
              d={`M0 ${r.endY.toFixed(2)} H16 V${r.rowY} H40`}
              vectorEffect="non-scaling-stroke"
            />
          ),
        )}
      </svg>

      <ol className={styles.seeds} onMouseLeave={() => setLit(null)}>
        {ranked.map((r) => (
          <li
            key={r.seed.slug}
            className={lit !== null && lit !== r.seed.slug ? `${styles.row} ${styles.rowDim}` : styles.row}
            style={{ '--i': r.index } as React.CSSProperties}
            onMouseEnter={() => setLit(r.seed.slug)}
            onFocus={() => setLit(r.seed.slug)}
            onBlur={() => setLit(null)}
          >
            <span className={styles.seed}>[{r.seed.position}]</span>
            <Link className={styles.name} to={`/players/${r.seed.slug}`}>
              {r.seed.name}
            </Link>
            <span className={styles.country}>({r.seed.country})</span>
            <span className={styles.leader} aria-hidden="true" />
            <span className={styles.elo}>{Math.round(r.seed.elo ?? 0)}</span>
            <span className={styles.delta}>
              {r.seed.delta === null ? null : <RankDelta delta={r.seed.delta} label="on rank" />}
            </span>
          </li>
        ))}
      </ol>

      <span className={styles.from} aria-hidden="true">
        {new Date(from).toISOString().slice(0, 7)}
      </span>
      <span className={styles.to} aria-hidden="true">
        {new Date(to).toISOString().slice(0, 7)}
      </span>
    </div>
  )
}
