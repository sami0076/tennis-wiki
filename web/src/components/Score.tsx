import { formatScore, scoreCarriesMark } from '../lib/format'
import styles from './Score.module.css'

interface ScoreProps {
  score: string | null
  /** The match was not played out; marked once, and not when the score already says so. */
  incomplete?: boolean
}

/**
 * Score types a scoreline the way the sheet does: 6-4 7-6(3), with ret. or w/o
 * where the source wrote them. A long score may take two lines in a narrow
 * column, but a set never breaks in the middle, which a bare string would do
 * at its hyphen.
 */
export function Score({ score, incomplete = false }: ScoreProps) {
  const typed = formatScore(score)
  if (typed === null) return null
  const sets = typed.split(' ')
  return (
    <span className={styles.score}>
      {sets.map((set, index) => (
        <span key={index}>
          {index > 0 ? ' ' : null}
          <span className={styles.set}>{set}</span>
        </span>
      ))}
      {incomplete && !scoreCarriesMark(score) ? (
        <span className={styles.mark}> incomplete</span>
      ) : null}
    </span>
  )
}
