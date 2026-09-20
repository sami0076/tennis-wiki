import { useLayoutEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { advanced, column, type SheetPage, type Slot } from '../lib/bracket'
import { Score } from './Score'
import styles from './DrawSheet.module.css'

/** The line a name is typed on, in px; every column's pitch is a multiple. */
export const PITCH = 26

interface DrawSheetProps {
  page: SheetPage
  /** Whether the last column's winner is the champion, who is set in 700. */
  kind: 'main' | 'qualifying'
  /** The player whose run is lit, if any; every sheet on a page shares it. */
  lit?: string | null
  onLit?: (slug: string | null) => void
}

/**
 * DrawSheet draws one page of the bracket as the typed sheet: a column per
 * round, a name written above its rule, the two names of a pair joined by a
 * vertical at the right and the winner's rule stepping in from its midpoint,
 * with the score written under the name in the column it earned. A name that
 * advanced is ink and one that went out is pencil.
 *
 * The geometry is the column's pitch, doubling to the right, so every rule
 * and stub is a border the slot owns and the columns are as wide as their
 * longest name. Hovering a name lights every line of that player's run.
 */
export function DrawSheet({ page, kind, lit = null, onLit }: DrawSheetProps) {
  const wrap = useRef<HTMLDivElement>(null)
  const [dense, setDense] = useState(false)

  // Typed at 13px, and one step down the ramp when the page is too narrow
  // for it at 13; beyond that the wrapper scrolls.
  useLayoutEffect(() => {
    const node = wrap.current
    if (node === null || typeof ResizeObserver === 'undefined') return
    const check = () => setDense(node.scrollWidth > node.clientWidth + 1)
    check()
    const observer = new ResizeObserver(check)
    observer.observe(node)
    return () => observer.disconnect()
  }, [page])

  const columns = Array.from({ length: page.c1 - page.c0 + 1 }, (_, i) =>
    page.roots.flatMap((root) => column(root, page.c0 + i)),
  )

  return (
    <div
      ref={wrap}
      className={dense ? `${styles.wrap} ${styles.dense}` : styles.wrap}
      onMouseOver={(event) => {
        const slot = (event.target as HTMLElement).closest<HTMLElement>('[data-slug]')
        if (slot?.dataset.slug) onLit?.(slot.dataset.slug)
      }}
      onMouseLeave={() => onLit?.(null)}
    >
      <div className={styles.sheet}>
        {columns.map((slots, i) => {
          const pitch = PITCH * 2 ** i
          const last = i === columns.length - 1
          return (
            <div
              key={i}
              className={styles.column}
              style={{ '--pitch': `${pitch}px`, '--anchor': `${pitch / 2 + PITCH / 2 - 5}px` } as React.CSSProperties}
            >
              <div className={styles.head}>{page.heads[i]}</div>
              {slots.map((slot, j) => (
                <Line
                  key={j}
                  slot={slot}
                  entrant={i === 0}
                  upper={j % 2 === 0}
                  joins={!last}
                  stub={i > 0}
                  champion={kind === 'main' && slot.parent === null && slot.col === page.c1 && last}
                  lit={lit !== null && slot.side?.slug === lit}
                />
              ))}
            </div>
          )
        })}
      </div>
    </div>
  )
}

interface LineProps {
  slot: Slot
  /** The entrants' column writes the country; the others already have it. */
  entrant: boolean
  /** The upper of a pair joins downward to the winner's line, the lower upward. */
  upper: boolean
  /** Whether a column follows, so the line has a vertical to draw. */
  joins: boolean
  /** Whether the rule steps in from the gutter on its left. */
  stub: boolean
  champion: boolean
  lit: boolean
}

function Line({ slot, entrant, upper, joins, stub, champion, lit }: LineProps) {
  const classes = [
    styles.slot,
    advanced(slot) ? styles.advanced : null,
    upper ? styles.upper : styles.lower,
    joins ? styles.joins : null,
    stub ? styles.stub : null,
    champion ? styles.champion : null,
    lit ? styles.lit : null,
    slot.side === null ? styles.void : null,
  ]
    .filter((c) => c !== null)
    .join(' ')

  if (slot.side === null) {
    return (
      <div className={classes}>
        <div className={styles.name}>{slot.void}</div>
      </div>
    )
  }

  const side = slot.side
  const pre = side.seed !== null ? `[${side.seed}] ` : side.entry !== null ? `${side.entry} ` : ''
  return (
    <div className={classes} data-slug={side.slug}>
      <div className={styles.name}>
        {pre ? <span className={styles.pre}>{pre}</span> : null}
        <Link className={styles.player} to={`/players/${side.slug}`}>
          {side.name}
        </Link>
        {entrant && side.country !== null ? <span className={styles.country}> ({side.country})</span> : null}
      </div>
      {!entrant && slot.match !== null ? (
        <div className={styles.score}>
          <Score score={slot.match.score} incomplete={slot.match.incomplete} />
        </div>
      ) : null}
    </div>
  )
}
