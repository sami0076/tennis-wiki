import type { ElementType, ReactNode } from 'react'
import { Reveal } from './Reveal'
import styles from './Card.module.css'

interface CardProps {
  children: ReactNode
  title?: ReactNode
  /** Small caps at the right of the title row: a window, a count, a link. */
  aside?: ReactNode
  /** Tint the card as one side of a comparison, or as the ball. */
  tint?: 'a' | 'b' | 'lime'
  as?: ElementType
  delay?: number
  className?: string
}

/** Card is a white panel on the cream ground, risen into place on first view. */
export function Card({ children, title, aside, tint, as = 'section', delay, className }: CardProps) {
  const classes = [styles.card, tint ? styles[tint] : '', className].filter(Boolean).join(' ')
  return (
    <Reveal as={as} delay={delay} className={classes}>
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
