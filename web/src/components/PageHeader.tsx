import { Fragment, type CSSProperties, type ReactNode } from 'react'
import styles from './PageHeader.module.css'

interface PageHeaderProps {
  kicker?: ReactNode
  title: ReactNode
  /** A word of a string title to sweep a lime highlighter under. */
  mark?: string
  lede?: ReactNode
  /** Beside the title on a wide screen: an illustration or a panel. */
  art?: ReactNode
  accent?: 'a' | 'b' | 'lime'
  children?: ReactNode
}

/** A string title arrives a word at a time; the full text is what is read out. */
function Title({ text, mark }: { text: string; mark?: string }) {
  const words = text.split(' ')
  return (
    <>
      <span className="sr-only">{text}</span>
      <span aria-hidden="true">
        {words.map((word, index) => (
          <Fragment key={index}>
          <span className={styles.wordWrap}>
            <span
              className={
                mark !== undefined && word.replace(/[.,]$/, '') === mark
                  ? `${styles.word} ${styles.marked}`
                  : styles.word
              }
              style={{ '--w': index } as CSSProperties}
            >
              {word}
            </span>
          </span>
          {index < words.length - 1 ? ' ' : null}
          </Fragment>
        ))}
      </span>
    </>
  )
}

/** PageHeader opens a page: a kicker with its accent dash, a big title, a standfirst. */
export function PageHeader({ kicker, title, mark, lede, art, accent = 'lime', children }: PageHeaderProps) {
  return (
    <header className={art ? `${styles.header} ${styles.withArt}` : styles.header}>
      <div className={styles.text}>
        {kicker !== undefined ? (
          <div className={styles.kicker}>
            <span className={`${styles.dash} ${styles[accent]}`} aria-hidden="true" />
            {kicker}
          </div>
        ) : null}
        <h1 className={styles.title}>
          {typeof title === 'string' ? <Title text={title} mark={mark} /> : title}
        </h1>
        {lede !== undefined ? <p className={styles.lede}>{lede}</p> : null}
        {children}
      </div>
      {art ? <div className={styles.art}>{art}</div> : null}
    </header>
  )
}
