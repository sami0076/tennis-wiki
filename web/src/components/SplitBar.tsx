import styles from './SplitBar.module.css'

interface SplitBarProps {
  label: string
  left: number
  right: number
  /** How to render each number, e.g. as a percentage. */
  format?: (value: number) => string
}

/**
 * SplitBar is a centre-out paired bar: player A to the left in --ink, player B
 * to the right in --ink-3.
 *
 * Two players are always monochrome. Hue is spent on surface, and a bar that
 * coloured the players would collide with the surface encoding on the same page.
 */
export function SplitBar({ label, left, right, format = String }: SplitBarProps) {
  const total = left + right
  const leftShare = total > 0 ? (left / total) * 100 : 50

  return (
    <div className={styles.split}>
      <div className={styles.numbers}>
        <span className={styles.left}>{format(left)}</span>
        <span className={styles.label}>{label}</span>
        <span className={styles.right}>{format(right)}</span>
      </div>
      <div
        className={styles.track}
        role="img"
        aria-label={`${label}: ${format(left)} to ${format(right)}`}
      >
        <div className={styles.fillLeft} style={{ width: `${leftShare}%` }} />
        <div className={styles.gap} />
        <div className={styles.fillRight} style={{ width: `${100 - leftShare}%` }} />
      </div>
    </div>
  )
}
