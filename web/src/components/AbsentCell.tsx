import styles from './AbsentCell.module.css'

interface AbsentCellProps {
  /** What is missing, for the accessible name: "aces", "first serve won". */
  label?: string
}

/**
 * AbsentCell is one missing value in a table that otherwise has numbers.
 *
 * Typed n/r, the way a sheet marks a figure nobody wrote down: never a zero,
 * never a blank. A zero is a claim that the player did the thing zero times;
 * this says it was not recorded. The column stays in the table because hiding
 * it would misrepresent the dataset.
 */
export function AbsentCell({ label }: AbsentCellProps) {
  const description = label ? `${label}: not recorded` : 'Not recorded'
  return (
    <span className={styles.mark} title="Not recorded" aria-label={description}>
      n/r
    </span>
  )
}

/**
 * ABSENT_SORT_KEY sorts absent values after every present one, in both
 * directions. Sorting them as zero would put them at one end and read as the
 * worst performances in the table.
 */
export const ABSENT_SORT_KEY = null
