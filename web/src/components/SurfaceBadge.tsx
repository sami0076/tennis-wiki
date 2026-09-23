import { surfaceLabel, surfaceVar, surfaceWash } from '../lib/surface'
import styles from './SurfaceBadge.module.css'

/** SurfaceBadge is the surface name set in its ink on its wash. */
export function SurfaceBadge({ surface }: { surface: string | null }) {
  return (
    <span
      className={styles.badge}
      style={{ color: surfaceVar(surface), background: surfaceWash(surface) }}
    >
      {surface === null ? 'n/r' : surfaceLabel(surface)}
    </span>
  )
}
