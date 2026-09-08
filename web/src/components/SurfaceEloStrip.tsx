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
 * SurfaceEloStrip is the row of surface ratings a player page opens with.
 *
 * The best surface is washed in its own tint, which is the one place a
 * background carries meaning. Everything else here is hairlines on paper.
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
        const colour = s.surface === 'overall' ? 'var(--ink)' : surfaceVar(s.surface)
        return (
          <div
            key={s.surface}
            className={styles.cell}
            style={
              isBest
                ? { background: `color-mix(in srgb, ${colour} 7%, var(--paper))` }
                : undefined
            }
          >
            <div className={styles.label} style={{ color: colour }}>
              {s.surface === 'overall' ? 'Overall' : surfaceLabel(s.surface)}
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
