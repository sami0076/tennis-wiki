import { Fragment, type ReactNode } from 'react'
import styles from './Meta.module.css'

interface MetaProps {
  /**
   * The parts of the string. Anything null, undefined or empty is dropped, so a
   * player with no recorded hand does not leave a stranded gap.
   */
  parts: ReadonlyArray<ReactNode>
  className?: string
}

/**
 * Meta sets the parts of a meta line as typed fields: "Spain  right-handed  23
 * turned pro 2018". Two spaces between the parts, which on a mono sheet is the
 * whole separator, and which survives being copied out of the page.
 *
 * A component rather than a template string in each page, because the gap is
 * the whole design of the thing, and two screens writing it by hand is two
 * screens that will eventually disagree.
 */
export function Meta({ parts, className }: MetaProps) {
  const present = parts.filter(
    (part) => part !== null && part !== undefined && part !== '' && part !== false,
  )
  if (present.length === 0) return null

  return (
    <p className={[styles.meta, className].filter(Boolean).join(' ')}>
      {present.map((part, index) => (
        <Fragment key={index}>
          {index > 0 ? <span className={styles.gap}>{'  '}</span> : null}
          {part}
        </Fragment>
      ))}
    </p>
  )
}
