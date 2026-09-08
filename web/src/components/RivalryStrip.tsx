import { surfaceLabel, surfaceVar } from '../lib/surface'
import styles from './RivalryStrip.module.css'

export interface RivalryResult {
  /** True when player A won. */
  wonByA: boolean
  surface: string | null
  /** Shown in the tooltip and the accessible name of one square. */
  description: string
}

interface RivalryStripProps {
  results: ReadonlyArray<RivalryResult>
  nameA: string
  nameB: string
}

/**
 * RivalryStrip is one 13px square per meeting: filled when player A won,
 * outlined when player B did, coloured by surface either way.
 *
 * Fill against outline is the encoding; colour carries the surface only. The
 * legend underneath is not optional -- a strip of squares means nothing without
 * it, and the caption belongs directly under the thing it explains.
 */
export function RivalryStrip({ results, nameA, nameB }: RivalryStripProps) {
  return (
    <div>
      <div className={styles.strip}>
        {results.map((result, index) => {
          const colour = surfaceVar(result.surface)
          const winner = result.wonByA ? nameA : nameB
          const title = `${winner} won, ${surfaceLabel(result.surface).toLowerCase()}: ${result.description}`
          return (
            <span
              key={`${result.description}-${index}`}
              className={styles.square}
              style={{
                borderColor: colour,
                background: result.wonByA ? colour : 'transparent',
              }}
              title={title}
              role="img"
              aria-label={title}
            />
          )
        })}
      </div>
      <p className={styles.caption}>
        Filled squares are wins for {nameA}, outlined are wins for {nameB}. Colour is the
        surface.
      </p>
    </div>
  )
}
