import { checks, headings, html, source } from '../generated/methodology'
import { Meta } from '../components'
import styles from './Methodology.module.css'

const repo = 'https://github.com/sami0076/tennis-wiki/blob/main'

// The document splits at its first section heading: what is above it is the
// title and the standfirst.
const cut = html.indexOf('<h2')
const head = cut === -1 ? html : html.slice(0, cut)
const body = cut === -1 ? '' : html.slice(cut)

/**
 * The methodology, rendered from the document in the repository rather than
 * written a second time here.
 *
 * The numbers in the prose are checked against the committed output of
 * `make validate` when this page is generated, so the page and the run cannot
 * quietly disagree; the build fails instead. The figures below the title come
 * from that run directly.
 */
export function Methodology() {
  return (
    <div className={styles.layout}>
      <nav className={styles.contents} aria-label="Contents">
        <h2 className={styles.contentsTitle}>Contents</h2>
        <ol className={styles.list}>
          {headings
            .filter((heading) => heading.level === 2 || heading.level === 3)
            .map((heading) => (
              <li
                key={heading.id}
                className={heading.level === 3 ? styles.nested : undefined}
              >
                <a className={styles.link} href={`#${heading.id}`}>
                  {heading.text}
                </a>
              </li>
            ))}
        </ol>
      </nav>

      <div>
        {/* The title and its standfirst come first; the run's own figures sit
            under them, before the first section. */}
        <article className={styles.article} dangerouslySetInnerHTML={{ __html: head }} />
        <div className={styles.provenance}>
          <Meta
            parts={[
              `${checks.matchesReplayed.toLocaleString('en-GB')} matches replayed`,
              `${checks.tourAccuracy.toFixed(1)}% at tour level`,
              `Brier ${checks.brier.toFixed(3)} over ${checks.events} draws`,
              `run ${checks.generatedAt.slice(0, 10)}`,
            ]}
          />
          <p className={styles.note}>
            Every figure below comes from that run of <code>make validate</code>, committed
            as <a href={`${repo}/docs/validation.json`}>validation.json</a>. This page is
            generated from <a href={`${repo}/${source}`}>{source}</a>, so the repository and
            the site cannot say different things.
          </p>
        </div>

        {/* The document is written in this repository and rendered at build
            time. Nothing here comes from a request. */}
        <article className={styles.article} dangerouslySetInnerHTML={{ __html: body }} />
      </div>
    </div>
  )
}
