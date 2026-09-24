import type { CSSProperties } from 'react'
import { useInView } from '../lib/useInView'
import { prefersReducedMotion } from '../lib/useReducedMotion'
import styles from './Odometer.module.css'

const DIGITS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']

/**
 * Odometer rolls each digit of a whole number up into place, the way a
 * stadium scoreboard turns over. The accessible text is the number itself.
 */
export function Odometer({ value, className }: { value: number; className?: string }) {
  const [ref, seen] = useInView<HTMLSpanElement>()
  const text = String(Math.round(value))
  const animates = typeof IntersectionObserver !== 'undefined' && !prefersReducedMotion()

  if (!animates) {
    return (
      <span ref={ref} className={className}>
        {text}
      </span>
    )
  }

  return (
    <span ref={ref} className={[styles.odometer, seen ? styles.seen : '', className].filter(Boolean).join(' ')}>
      <span className="sr-only">{text}</span>
      {text.split('').map((char, index) => {
        const digit = DIGITS.indexOf(char)
        if (digit < 0) {
          return (
            <span key={index} className={styles.static} aria-hidden="true">
              {char}
            </span>
          )
        }
        // One full turn of the reel, then on to the digit.
        const turns = 10 + digit
        return (
          <span key={index} className={styles.slot} aria-hidden="true">
            <span
              className={styles.reel}
              style={{ '--to': turns, '--i': index } as CSSProperties}
            >
              {[...DIGITS, ...DIGITS].map((d, i) => (
                <span key={i}>{d}</span>
              ))}
            </span>
            <span className={styles.sizer}>{char}</span>
          </span>
        )
      })}
    </span>
  )
}
