import type { SeriesRating } from '../api/client'
import { formatElo } from '../lib/format'
import { surfaceLabel, surfaceVar } from '../lib/surface'
import styles from './SurfaceEloStrip.module.css'

interface SurfaceEloStripProps {
  series: ReadonlyArray<SeriesRating>
  /**
   * Which rating to lead with. A player who stopped in 1983 has a current
   * rating and it describes a different player from the one who held the peak.
   */
  mode: 'current' | 'peak'
}

/**
 * SurfaceEloStrip is the row of surface ratings a player page opens with: one
 * typed cell per series, the surface name beside its square, the figure set
 * in the surface colour, and the best surface boxed in its own hue.
 *
 * Only surfaces the player actually has a rating in appear: an unplayed surface
 * is absent from the API and stays absent here rather than becoming a 1500 the
 * reader would take for a real, mediocre rating.
 */
export function SurfaceEloStrip({ series, mode }: SurfaceEloStripProps) {
  if (series.length === 0) return null

  const value = (s: SeriesRating) => (mode === 'peak' ? s.peak.elo : s.current.elo)
  const surfaces = series.filter((s) => s.surface !== 'overall')
  const best = surfaces.reduce<SeriesRating | null>(
    (top, s) => (top === null || value(s) > value(top) ? s : top),
    null,
  )

  return (
    <div className={styles.strip}>
      {series.map((s) => {
        const isBest = best !== null && s.surface === best.surface
        const overall = s.surface === 'overall'
        const colour = overall ? 'var(--ink)' : surfaceVar(s.surface)
        return (
          <div
            key={s.surface}
            className={isBest ? `${styles.cell} ${styles.best}` : styles.cell}
            style={isBest ? { borderColor: colour } : undefined}
          >
            <div className={styles.label} style={{ color: overall ? 'var(--pencil)' : colour }}>
              {overall ? null : (
                <span className={styles.square} style={{ background: colour }} aria-hidden="true" />
              )}
              {overall ? 'Overall' : surfaceLabel(s.surface)}
              {mode === 'peak' ? ', peak' : null}
            </div>
            <div className={styles.value} style={{ color: colour }}>
              {formatElo(value(s))}
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
