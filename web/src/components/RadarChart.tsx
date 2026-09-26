import { useState, type KeyboardEvent } from 'react'

import { formatOrdinal } from '../lib/format'
import { useInView } from '../lib/useInView'
import styles from './RadarChart.module.css'

export interface RadarSeries {
  name: string
  tone: 'a' | 'b'
  /** 0 to 100 per axis, null where the player has no figure. */
  values: (number | null)[]
  /** The raw figure behind each value, for the readout and the table. */
  details: (string | null)[]
}

const SIZE = 400
const CENTRE = SIZE / 2
const RADIUS = 150
const RINGS = [25, 50, 75, 100]

function point(axis: number, count: number, value: number) {
  const angle = (axis / count) * 2 * Math.PI - Math.PI / 2
  const r = (value / 100) * RADIUS
  return { x: CENTRE + r * Math.cos(angle), y: CENTRE + r * Math.sin(angle) }
}

/**
 * The outline is cut where a player has no figure: the chord across the gap
 * is drawn faint and dotted, because a missing figure is not a zero and a
 * vertex at the centre would say it was.
 */
function Shape({ series, count }: { series: RadarSeries; count: number }) {
  const measured = series.values
    .map((value, axis) => (value === null ? null : { axis, ...point(axis, count, value) }))
    .filter((p) => p !== null)
  if (measured.length === 0) return null

  const fill = measured.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join('') + 'Z'
  const edges = measured.map((p, i) => {
    const next = measured[(i + 1) % measured.length] ?? p
    const skips = (next.axis - p.axis + count) % count !== 1
    return { from: p, to: next, skips: skips || measured.length === 1 }
  })

  return (
    <g className={`${styles.shape} ${styles[series.tone]}`}>
      <path className={styles.fill} d={fill} />
      {edges.map(({ from, to, skips }) => (
        <line
          key={from.axis}
          className={skips ? styles.gap : styles.edge}
          x1={from.x}
          y1={from.y}
          x2={to.x}
          y2={to.y}
        />
      ))}
      {measured.map((p) => (
        <circle key={p.axis} className={styles.vertex} cx={p.x} cy={p.y} r={4} />
      ))}
    </g>
  )
}

/**
 * Two players over the same axes, each axis a percentile. Pointing at an axis,
 * or focusing the chart and pressing an arrow, reads both players' figures.
 */
export function RadarChart({
  label,
  axes,
  series,
}: {
  label: string
  axes: string[]
  series: RadarSeries[]
}) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const [active, setActive] = useState<number | null>(null)
  const count = axes.length

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0
    if (step === 0) return
    event.preventDefault()
    setActive((current) => ((current ?? (step > 0 ? -1 : 0)) + step + count) % count)
  }

  const reading = (s: RadarSeries, axis: number) => {
    const value = s.values[axis] ?? null
    if (value === null) return 'no figure'
    const detail = s.details[axis] ?? null
    return detail === null ? formatOrdinal(value) : `${formatOrdinal(value)} · ${detail}`
  }

  return (
    <div ref={ref} className={`${styles.chart} ${seen ? styles.seen : ''}`}>
      <ul className={styles.legend}>
        {series.map((s) => (
          <li key={s.tone} className={styles[s.tone]}>
            <span className={styles.swatch} aria-hidden="true" />
            {s.name}
          </li>
        ))}
      </ul>

      <div
        className={styles.plot}
        role="figure"
        tabIndex={0}
        aria-label={`${label}. Use the arrow keys to read an axis.`}
        onKeyDown={onKeyDown}
        onPointerLeave={() => setActive(null)}
        onBlur={() => setActive(null)}
      >
        <svg viewBox={`0 0 ${SIZE} ${SIZE}`} aria-hidden="true">
          {RINGS.map((ring) => (
            <polygon
              key={ring}
              className={ring === 50 ? styles.median : styles.ring}
              points={axes
                .map((_, axis) => {
                  const p = point(axis, count, ring)
                  return `${p.x},${p.y}`
                })
                .join(' ')}
            />
          ))}
          {axes.map((_, axis) => {
            const end = point(axis, count, 100)
            return (
              <line
                key={axis}
                className={axis === active ? styles.spokeActive : styles.spoke}
                x1={CENTRE}
                y1={CENTRE}
                x2={end.x}
                y2={end.y}
              />
            )
          })}
          {series.map((s) => (
            <Shape key={s.tone} series={s} count={count} />
          ))}
          {axes.map((_, axis) => {
            const a = point(axis - 0.5, count, 118)
            const b = point(axis + 0.5, count, 118)
            return (
              <path
                key={axis}
                className={styles.hit}
                d={`M${CENTRE},${CENTRE}L${a.x},${a.y}L${b.x},${b.y}Z`}
                onPointerEnter={() => setActive(axis)}
                onPointerDown={() => setActive(axis)}
              />
            )
          })}
        </svg>

        {axes.map((name, axis) => {
          const at = point(axis, count, 118)
          return (
            <span
              key={name}
              className={`${styles.axis} ${axis === active ? styles.axisActive : ''}`}
              style={{ left: `${(at.x / SIZE) * 100}%`, top: `${(at.y / SIZE) * 100}%` }}
              aria-hidden="true"
            >
              {name}
            </span>
          )
        })}
        <span
          className={styles.medianLabel}
          style={{ top: `${((CENTRE - RADIUS / 2) / SIZE) * 100}%` }}
          aria-hidden="true"
        >
          Tour median
        </span>
      </div>

      <div className={styles.readout} aria-hidden="true">
        {active === null ? (
          <span className={styles.hint}>Point at an axis to read both figures</span>
        ) : (
          <>
            <strong>{axes[active]}</strong>
            {series.map((s) => (
              <span key={s.tone} className={styles[s.tone]}>
                <span className={styles.swatch} />
                {reading(s, active)}
              </span>
            ))}
          </>
        )}
      </div>

      <p className="sr-only" aria-live="polite">
        {active === null
          ? ''
          : `${axes[active]}: ${series.map((s) => `${s.name} ${reading(s, active)}`).join(', ')}`}
      </p>

      <table className="sr-only">
        <caption>{label}</caption>
        <thead>
          <tr>
            <th scope="col">Axis</th>
            {series.map((s) => (
              <th key={s.tone} scope="col">
                {s.name}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {axes.map((name, axis) => (
            <tr key={name}>
              <th scope="row">{name}</th>
              {series.map((s) => (
                <td key={s.tone}>{reading(s, axis)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
