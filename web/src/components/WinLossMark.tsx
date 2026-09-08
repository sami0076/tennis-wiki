import styles from './WinLossMark.module.css'

interface WinLossMarkProps {
  won: boolean
}

/**
 * WinLossMark is a bold W or L. The letter is always there: the colour is a
 * second encoding of something the character already says, never the only one.
 */
export function WinLossMark({ won }: WinLossMarkProps) {
  return (
    <span className={[styles.mark, won ? styles.win : styles.loss].join(' ')}>
      <span aria-hidden="true">{won ? 'W' : 'L'}</span>
      <span className="sr-only">{won ? 'Won' : 'Lost'}</span>
    </span>
  )
}
