import type { ReactNode } from 'react'
import styles from './PartialAggregate.module.css'

interface PartialAggregateProps {
  /** The aggregate itself: a row, a figure, whatever is being summarised. */
  children: ReactNode
  /** How many rows carried the value. */
  recorded: number
  /** How many rows there are in total. */
  total: number
  /** What is being averaged, for the caption: "Averages", "Totals". */
  noun?: string
  /** What n/r means, for the rows that carry it. */
  dashMeans?: string
}

/**
 * PartialAggregate is a summary over a column with gaps in it.
 *
 * The aggregate is computed over recorded rows only -- there is nothing else it
 * could honestly be -- and the caption says so. An average that silently skips
 * the gaps is a correctness bug wearing a design costume, and the only thing
 * separating this component from that bug is the sentence underneath.
 *
 * The same caption explains the n/r mark, because a reader meets both at once:
 * the rows that are missing and the average that had to skip them.
 */
export function PartialAggregate({
  children,
  recorded,
  total,
  noun = 'Averages',
  dashMeans = "the tournament didn't record serve statistics",
}: PartialAggregateProps) {
  const complete = recorded === total
  return (
    <>
      {children}
      <p className={styles.caption}>
        {complete ? null : `n/r means ${dashMeans}. `}
        {complete
          ? `${noun} cover all ${total} matches.`
          : `${noun} cover the ${recorded} of ${total} matches that did.`}
      </p>
    </>
  )
}
