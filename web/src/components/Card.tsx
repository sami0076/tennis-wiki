import type { ElementType, PointerEvent, ReactNode } from 'react'
import { Reveal } from './Reveal'
import styles from './Card.module.css'

interface CardProps {
  children: ReactNode
  title?: ReactNode
  /** Small caps at the right of the title row: a window, a count, a link. */
  aside?: ReactNode
  /** Tint the card as one side of a comparison, as the ball, or as a dark broadcast panel. */
  tint?: 'a' | 'b' | 'lime' | 'ink'
  /** Lean toward the pointer. Off for cards holding wide tables or charts. */
  tilt?: boolean
  as?: ElementType
  delay?: number
  className?: string
  /**
   * An anchor for the section rail. A card carrying one also carries
   * data-anchor, which is what gives it the scroll margin that keeps its
   * heading out from under the two sticky bars above it.
   */
  id?: string
}

// The spotlight follows the pointer through two custom properties; a mouse
// only, so a touch never leaves a card tilted.
function track(event: PointerEvent<HTMLElement>) {
  if (event.pointerType !== 'mouse') return
  const box = event.currentTarget.getBoundingClientRect()
  const x = (event.clientX - box.left) / box.width
  const y = (event.clientY - box.top) / box.height
  event.currentTarget.style.setProperty('--mx', `${x * 100}%`)
  event.currentTarget.style.setProperty('--my', `${y * 100}%`)
  event.currentTarget.style.setProperty('--rx', `${(0.5 - y) * 5}deg`)
  event.currentTarget.style.setProperty('--ry', `${(x - 0.5) * 5}deg`)
}

function settle(event: PointerEvent<HTMLElement>) {
  event.currentTarget.style.setProperty('--rx', '0deg')
  event.currentTarget.style.setProperty('--ry', '0deg')
}

/** Card is a panel on the cream ground, risen into place on first view. */
export function Card({ children, title, aside, tint, tilt = false, as = 'section', delay, className, id }: CardProps) {
  const classes = [styles.card, tint ? styles[tint] : '', tilt ? styles.tilt : '', className]
    .filter(Boolean)
    .join(' ')
  return (
    <Reveal
      as={as}
      delay={delay}
      className={classes}
      id={id}
      data-anchor={id === undefined ? undefined : ''}
      onPointerMove={track}
      onPointerLeave={settle}
    >
      {title !== undefined || aside !== undefined ? (
        <div className={styles.head}>
          {title !== undefined ? <h2 className={styles.title}>{title}</h2> : <span />}
          {aside !== undefined ? <div className={styles.aside}>{aside}</div> : null}
        </div>
      ) : null}
      {children}
    </Reveal>
  )
}

/** Kicker is the small uppercase label over a figure or a section. */
export function Kicker({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={[styles.kicker, className].filter(Boolean).join(' ')}>{children}</div>
}
