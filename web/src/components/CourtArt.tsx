import { useReducedMotion } from '../lib/useReducedMotion'
import styles from './CourtArt.module.css'

interface CourtArtProps {
  surface?: 'hard' | 'clay' | 'grass'
  /** Cycle the court through hard, clay and grass. */
  cycle?: boolean
  /** Loop a ball over the net along a dashed flight. */
  rally?: boolean
  className?: string
}

const W = 520
const FAR = 70
const NEAR = 330

// A court seen from behind the near baseline: v runs far (0) to near (1), u
// across (0 left doubles line, 1 right). Depth is foreshortened toward the far end.
function at(u: number, v: number): [number, number] {
  const y = FAR + (NEAR - FAR) * ((v * 1.8) / (1 + 0.8 * v))
  const half = 105 + ((y - FAR) / (NEAR - FAR)) * 140
  return [W / 2 - half + u * half * 2, y]
}

function line(u1: number, v1: number, u2: number, v2: number) {
  const [x1, y1] = at(u1, v1)
  const [x2, y2] = at(u2, v2)
  return `M${x1.toFixed(1)} ${y1.toFixed(1)}L${x2.toFixed(1)} ${y2.toFixed(1)}`
}

const ALLEY = 0.125
const SERVICE = 0.269

const MARKINGS = [
  line(0, 0, 1, 0),
  line(0, 1, 1, 1),
  line(0, 0, 0, 1),
  line(1, 0, 1, 1),
  line(ALLEY, 0, ALLEY, 1),
  line(1 - ALLEY, 0, 1 - ALLEY, 1),
  line(ALLEY, 0.5 - SERVICE, 1 - ALLEY, 0.5 - SERVICE),
  line(ALLEY, 0.5 + SERVICE, 1 - ALLEY, 0.5 + SERVICE),
  line(0.5, 0.5 - SERVICE, 0.5, 0.5 + SERVICE),
  line(0.5, 0, 0.5, 0.025),
  line(0.5, 1, 0.5, 0.975),
]

const corners = [at(0, 0), at(1, 0), at(1, 1), at(0, 1)]
const surfacePath = `M${corners.map(([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`).join('L')}Z`
const apron = (() => {
  const pad = [at(-0.12, -0.08), at(1.12, -0.08), at(1.12, 1.06), at(-0.12, 1.06)]
  return `M${pad.map(([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`).join('L')}Z`
})()

const [netLeftX, netY] = at(-0.04, 0.5)
const [netRightX] = at(1.04, 0.5)

const [serveX, serveY] = at(0.3, 0.9)
const [landX, landY] = at(0.72, 0.18)
const [outX, outY] = at(0.9, -0.05)
const FLIGHT = `M${serveX.toFixed(1)} ${(serveY - 60).toFixed(1)} Q${(W / 2).toFixed(1)} -40 ${landX.toFixed(1)} ${landY.toFixed(1)} Q${((landX + outX) / 2).toFixed(1)} ${(landY - 60).toFixed(1)} ${outX.toFixed(1)} ${(outY - 20).toFixed(1)}`

/**
 * CourtArt is the site's illustration: a court whose lines draw themselves in,
 * with a ball looping over the net. Decorative, so hidden from assistive tech.
 */
export function CourtArt({ surface = 'hard', cycle = false, rally = true, className }: CourtArtProps) {
  const reduced = useReducedMotion()
  return (
    <svg
      className={[styles.court, styles[surface], cycle ? styles.cycle : '', className]
        .filter(Boolean)
        .join(' ')}
      viewBox={`0 0 ${W} 380`}
      aria-hidden="true"
      focusable="false"
    >
      <path className={styles.apron} d={apron} />
      <path className={styles.surface} d={surfacePath} />
      {MARKINGS.map((d, index) => (
        <path
          key={d}
          className={styles.marking}
          d={d}
          pathLength={1}
          style={{ animationDelay: `${200 + index * 70}ms` }}
        />
      ))}
      <g className={styles.net}>
        <line x1={netLeftX} y1={netY} x2={netLeftX} y2={netY - 34} />
        <line x1={netRightX} y1={netY} x2={netRightX} y2={netY - 34} />
        <path className={styles.mesh} d={`M${netLeftX} ${netY - 30}L${netRightX} ${netY - 30}L${netRightX} ${netY}L${netLeftX} ${netY}Z`} />
        <line className={styles.tape} x1={netLeftX} y1={netY - 30} x2={netRightX} y2={netY - 30} />
      </g>
      {rally ? (
        <>
          <path className={styles.flight} d={FLIGHT} />
          <ellipse className={styles.bounce} cx={landX} cy={landY} rx="14" ry="4" />
          <g className={styles.ball}>
            <circle r="9" />
            <path d="M-6 -6 Q0 0 -6 6 M6 -6 Q0 0 6 6" />
            {reduced ? null : (
              <animateMotion dur="2.8s" repeatCount="indefinite" path={FLIGHT} />
            )}
          </g>
        </>
      ) : null}
    </svg>
  )
}
