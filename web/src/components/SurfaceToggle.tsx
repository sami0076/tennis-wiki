import { SURFACES } from '../lib/surface'
import { surfaceLabel, surfaceVar } from '../lib/surface'
import styles from './SurfaceToggle.module.css'

interface SurfaceToggleProps {
  /** null is "all surfaces". */
  value: string | null
  onChange: (surface: string | null) => void
  /** Restrict the options, e.g. to the surfaces a player has actually played. */
  options?: ReadonlyArray<string>
}

/**
 * SurfaceToggle is a row of typed filter cells. The active one is boxed in its
 * own surface colour and set in it -- the one place a filter is allowed to be
 * coloured, because the filter *is* the surface.
 *
 * Pair it with useUrlParam so the selection survives a reload and a shared link.
 */
export function SurfaceToggle({ value, onChange, options = SURFACES }: SurfaceToggleProps) {
  return (
    <div className={styles.row} role="group" aria-label="Filter by surface">
      <button
        type="button"
        className={[styles.option, value === null ? styles.active : ''].join(' ')}
        aria-pressed={value === null}
        onClick={() => onChange(null)}
      >
        All surfaces
      </button>
      {options.map((surface) => {
        const active = value === surface
        return (
          <button
            key={surface}
            type="button"
            className={[styles.option, active ? styles.active : ''].join(' ')}
            style={active ? { color: surfaceVar(surface), borderColor: surfaceVar(surface) } : undefined}
            aria-pressed={active}
            onClick={() => onChange(surface)}
          >
            {surfaceLabel(surface)}
          </button>
        )
      })}
    </div>
  )
}
