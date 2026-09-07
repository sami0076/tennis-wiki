import styles from './SplitBar.module.css'

interface SplitBarProps {
  label: string
  left: number
  right: number
  /**
   * What both halves are measured against. Defaults to the larger of the two,
   * which makes one bar full-length; pass a shared max across a group of bars
   * to make them comparable with each other.
   */
  max?: number
  /** How to render each number, e.g. as a percentage. */
  format?: (value: number) => string
}

/**
 * SplitBar is a centre-out paired bar: player A to the left in --ink, player B
 * to the right in --ink-3, each growing outward from the middle.
 *
 * Centre-out rather than a stacked bar split by share. 43% against 45% is two
 * short bars nearly the same length; as a share of the pair it would be a bar
 * split just off centre and would read as a close contest between two large
 * numbers. The length has to mean the value.
 *
 * Two players are always monochrome. Hue is spent on surface, and colouring the
 * players would collide with the surface encoding on the same page.
 */
export function SplitBar({ label, left, right, max, format = String }: SplitBarProps) {
  const scale = max ?? Math.max(left, right)
  const share = (value: number) => (scale > 0 ? Math.min(100, (value / scale) * 100) : 0)

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
        <div className={`${styles.half} ${styles.halfLeft}`}>
          <div className={styles.fillLeft} style={{ width: `${share(left)}%` }} />
        </div>
        <div className={styles.gap} />
        <div className={styles.half}>
          <div className={styles.fillRight} style={{ width: `${share(right)}%` }} />
        </div>
      </div>
    </div>
  )
}
