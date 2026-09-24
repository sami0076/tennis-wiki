import type { CSSProperties } from 'react'
import type { SeriesRating } from '../api/client'
import { formatElo } from '../lib/format'
import { useInView } from '../lib/useInView'
import { surfaceLabel, surfaceVar } from '../lib/surface'
import styles from './SurfaceEloStrip.module.css'

interface SurfaceEloStripProps {
  series: ReadonlyArray<SeriesRating>
  /**
   * Which rating to lead with. A player who stopped in 1983 has a current
   * rating and it describes a different player from the one who held the peak.
   */
  mode: 'current' | 'peak'
  /** Leave the overall series out where the page already leads with it. */
  overall?: boolean
}

const FLOOR = 1200
const CEILING = 2600

/**
 * SurfaceEloStrip is a player's ratings as bars, one per series, each in its
 * surface colour on one fixed scale, the best surface marked.
 *
 * Only surfaces the player actually has a rating in appear: an unplayed surface
 * is absent from the API and stays absent here rather than becoming a 1500 the
 * reader would take for a real, mediocre rating.
 */
export function SurfaceEloStrip({ series, mode, overall = true }: SurfaceEloStripProps) {
  const [ref, seen] = useInView<HTMLDivElement>()
  const shown = overall ? series : series.filter((s) => s.surface !== 'overall')
  if (shown.length === 0) return null

  const value = (s: SeriesRating) => (mode === 'peak' ? s.peak.elo : s.current.elo)
  const surfaces = series.filter((s) => s.surface !== 'overall')
  const best = surfaces.reduce<SeriesRating | null>(
    (top, s) => (top === null || value(s) > value(top) ? s : top),
    null,
  )
  const ceiling = Math.max(CEILING, ...shown.map(value))
  const share = (v: number) => Math.max(0.04, Math.min(1, (v - FLOOR) / (ceiling - FLOOR)))

  return (
    <div ref={ref} className={seen ? `${styles.strip} ${styles.seen}` : styles.strip}>
      {shown.map((s, index) => {
        const isOverall = s.surface === 'overall'
        const colour = isOverall ? 'var(--ink)' : surfaceVar(s.surface)
        return (
          <div
            key={s.surface}
            className={styles.row}
            style={{ '--tone': colour, '--i': index } as CSSProperties}
          >
            <div className={styles.label}>
              {isOverall ? 'Overall' : surfaceLabel(s.surface)}
              {mode === 'peak' ? ', peak' : null}
              {best !== null && s.surface === best.surface ? (
                <span className={styles.best}>best</span>
              ) : null}
            </div>
            <div className={styles.value}>{formatElo(value(s))}</div>
            <div className={styles.track}>
              <div className={styles.fill} style={{ transform: `scaleX(${share(value(s))})` }} />
            </div>
            <div className={styles.matches}>
              {s.matches.toLocaleString()} {s.matches === 1 ? 'match' : 'matches'}
            </div>
          </div>
        )
      })}
    </div>
  )
}
