import { useEffect, useState } from 'react'
import styles from './SectionRail.module.css'

export interface RailItem {
  /** The id of the section this links to, without the hash. */
  id: string
  label: string
}

interface SectionRailProps {
  items: ReadonlyArray<RailItem>
  /** Named for screen readers, which meet this as a second navigation. */
  label?: string
}

/**
 * A sticky strip of the sections below it, with the one in view marked.
 *
 * A career page is a dozen cards deep and every one of them is worth reading,
 * which makes the scrollbar the only way to find the one a reader came for.
 * This turns that scroll into a table of contents that stays put, and the
 * marked entry doubles as a "where am I" the scroll position cannot give.
 *
 * Each entry is a real anchor, so every section is a link somebody can send.
 */
export function SectionRail({ items, label = 'Sections' }: SectionRailProps) {
  const current = useSectionInView(items.map((item) => item.id))

  return (
    <nav className={styles.rail} aria-label={label}>
      <ul className={styles.list}>
        {items.map((item) => (
          <li key={item.id}>
            <a
              href={`#${item.id}`}
              className={item.id === current ? `${styles.item} ${styles.on}` : styles.item}
              aria-current={item.id === current ? 'true' : undefined}
            >
              {item.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  )
}

/**
 * Which of the sections is the one being read.
 *
 * An observer rather than a scroll handler, because the answer is "which
 * headings are on screen" and that is the question an IntersectionObserver
 * answers without running code on every frame of a scroll.
 *
 * The top margin pulls the trigger line below the sticky header and this rail;
 * without it a section counts as in view while still hidden behind them. The
 * bottom margin keeps only the top third of the viewport in play, so the
 * marked entry is the section a reader is looking at rather than the last one
 * that happens to be anywhere on screen.
 */
function useSectionInView(ids: ReadonlyArray<string>): string | null {
  const [current, setCurrent] = useState<string | null>(ids[0] ?? null)
  // The ids change identity every render as an inline array; their contents do
  // not, so the effect keys on the joined string.
  const key = ids.join(',')

  useEffect(() => {
    // Without an observer (jsdom, old browsers) the first entry stays marked
    // and every link still works: the rail is a table of contents first and a
    // position indicator second.
    if (typeof IntersectionObserver === 'undefined') return
    const sections = key
      .split(',')
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null)
    if (sections.length === 0) return

    // Kept outside the callback: an entry that leaves the observed band still
    // fires, and the last one still inside it is the answer.
    const visible = new Set<string>()
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) visible.add(entry.target.id)
          else visible.delete(entry.target.id)
        }
        const first = key.split(',').find((id) => visible.has(id))
        if (first !== undefined) setCurrent(first)
      },
      { rootMargin: '-120px 0px -62% 0px', threshold: 0 },
    )
    for (const section of sections) observer.observe(section)
    return () => observer.disconnect()
  }, [key])

  return current
}
