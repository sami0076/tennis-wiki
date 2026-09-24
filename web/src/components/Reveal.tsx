import type { CSSProperties, ElementType, HTMLAttributes, ReactNode } from 'react'
import { useInView } from '../lib/useInView'
import styles from './Reveal.module.css'

interface RevealProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode
  /** Milliseconds, for staggering siblings. */
  delay?: number
  as?: ElementType
}

/** Reveal rises its children into place the first time they scroll into view. */
export function Reveal({ children, delay = 0, as: Tag = 'div', className, style, ...rest }: RevealProps) {
  const [ref, seen] = useInView<HTMLElement>()
  return (
    <Tag
      ref={ref}
      className={[styles.reveal, seen ? styles.seen : '', className].filter(Boolean).join(' ')}
      style={{ '--delay': `${delay}ms`, ...style } as CSSProperties}
      {...rest}
    >
      {children}
    </Tag>
  )
}
