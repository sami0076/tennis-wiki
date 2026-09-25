import { flagCode } from '../lib/country'
import styles from './Flag.module.css'

interface FlagProps {
  /** The IOC code the database stores, e.g. SUI. */
  country: string | null | undefined
  /**
   * Print the code beside the flag. On by default, and the reason a flag is
   * allowed here at all: colour is never the only encoding on this site, and
   * a hundred-odd flags at 16px are not something anyone can tell apart.
   *
   * Off is for a flag sitting against a name that already identifies the
   * player. There the flag is decoration and is hidden from assistive
   * technology entirely -- announcing "USA Jimmy Connors" would put the
   * country inside the link's name, where it is noise rather than help.
   */
  code?: boolean
  size?: 'sm' | 'md'
  className?: string
}

/**
 * A player's flag, beside their country code.
 *
 * Drawn from a static SVG rather than the flag emoji, which has no glyphs at
 * all in Chrome and Edge on Windows and renders there as two letters. A file
 * per country costs one small request and looks the same everywhere.
 *
 * A country with no flag -- the Soviet Union, Yugoslavia, Rhodesia, or a code
 * the map has never seen -- keeps its letters and gets no picture. Flying a
 * successor state's flag would be inventing a fact about somebody's career.
 */
export function Flag({ country, code = true, size = 'sm', className }: FlagProps) {
  const iso = flagCode(country)
  const label = country?.trim() ?? ''
  if (label === '') return null

  return (
    <span
      className={[styles.flag, styles[size], className].filter(Boolean).join(' ')}
      aria-hidden={code ? undefined : true}
    >
      {iso === null ? (
        // The slot stays, so a column of names does not go ragged where a
        // flag is missing.
        <span className={styles.blank} aria-hidden="true" />
      ) : (
        <img
          className={styles.image}
          src={`/flags/${iso}.svg`}
          alt=""
          width={18}
          height={12}
          loading="lazy"
          decoding="async"
        />
      )}
      {code ? <span className={styles.code}>{label}</span> : null}
    </span>
  )
}
