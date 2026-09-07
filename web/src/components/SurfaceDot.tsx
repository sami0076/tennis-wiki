import { surfaceLabel, surfaceVar } from '../lib/surface'
import styles from './SurfaceDot.module.css'

interface SurfaceDotProps {
  surface: string | null
  /** Hide the text only when an adjacent cell already carries it. */
  label?: boolean
}

/**
 * SurfaceDot is an 8px square in the surface colour, always next to the surface
 * name. Colour is never the only encoding: with the label off, the name still
 * reaches a screen reader, and a sighted reader still has the column heading.
 */
export function SurfaceDot({ surface, label = true }: SurfaceDotProps) {
  const name = surfaceLabel(surface)
  return (
    <span className={styles.wrap}>
      <span className={styles.dot} style={{ background: surfaceVar(surface) }} aria-hidden="true" />
      {label ? name : <span className="sr-only">{name}</span>}
    </span>
  )
}
