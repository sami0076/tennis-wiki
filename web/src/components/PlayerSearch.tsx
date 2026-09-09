import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import type { PlayerSearchResult } from '../api/types.gen'
import { MIN_QUERY, usePlayerSearch, type PlayerSearchState } from '../api/useSearch'
import { PlayerSummary } from './PlayerSummary'
import styles from './PlayerSearch.module.css'

interface PlayerSearchProps {
  /** Always rendered, visibly or not. A placeholder is not a label. */
  label: string
  /** Keep the label for screen readers only, where the context already says it. */
  hideLabel?: boolean
  value: string
  onChange: (value: string) => void
  onSelect: (player: PlayerSearchResult) => void
  /** Enter with nothing highlighted. Omit and Enter does nothing. */
  onSubmit?: (value: string) => void
  tour?: string | null
  placeholder?: string
  limit?: number
}

/**
 * PlayerSearch is the combobox behind every way into a player page.
 *
 * A real combobox rather than a div that looks like one: the input owns the
 * ARIA state, the options are options, and the highlighted one is named by
 * aria-activedescendant so focus never leaves the input the reader is typing
 * into. Arrows move, Enter picks, Escape backs out.
 *
 * The text is controlled by the caller. The header keeps it in state, the
 * search page keeps it in the URL, and a head-to-head picker sets it to the
 * name just chosen -- one input, three different owners of the query.
 */
export function PlayerSearch({
  label,
  hideLabel = false,
  value,
  onChange,
  onSelect,
  onSubmit,
  tour = null,
  placeholder,
  limit = 8,
}: PlayerSearchProps) {
  const id = useId()
  const listId = `${id}-list`
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const listRef = useRef<HTMLUListElement>(null)

  const search = usePlayerSearch(value, { tour, limit })
  const results = search.tooShort ? [] : search.results
  // Results from the previous query stay on screen while the next request is in
  // flight, so the highlight is clamped rather than trusted.
  const highlighted = active >= 0 && active < results.length ? active : -1
  const showPanel = open && value.trim() !== ''
  const message = panelMessage(search)

  useEffect(() => {
    setActive(-1)
  }, [search.query])

  // Arrowing past the visible rows has to bring them into view, or the
  // highlight walks off the bottom of a scrolled list.
  useEffect(() => {
    if (highlighted < 0) return
    listRef.current?.children[highlighted]?.scrollIntoView({ block: 'nearest' })
  }, [highlighted])

  function choose(player: PlayerSearchResult) {
    setOpen(false)
    setActive(-1)
    onSelect(player)
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        setOpen(true)
        if (results.length > 0) setActive((current) => (current + 1) % results.length)
        break
      case 'ArrowUp':
        event.preventDefault()
        setOpen(true)
        if (results.length > 0) {
          setActive((current) => (current <= 0 ? results.length - 1 : current - 1))
        }
        break
      case 'Enter': {
        const chosen = highlighted >= 0 ? results[highlighted] : undefined
        if (chosen !== undefined) {
          event.preventDefault()
          choose(chosen)
        } else if (onSubmit !== undefined) {
          event.preventDefault()
          setOpen(false)
          onSubmit(value.trim())
        }
        break
      }
      case 'Escape':
        // The first Escape dismisses the list and the second clears the query.
        // The other order throws away typing the reader only wanted to see past.
        if (showPanel) setOpen(false)
        else onChange('')
        break
      case 'Tab':
        setOpen(false)
        break
      default:
    }
  }

  return (
    <div
      className={styles.wrap}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false)
      }}
    >
      <label className={hideLabel ? 'sr-only' : styles.label} htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        className={styles.input}
        type="text"
        role="combobox"
        aria-expanded={showPanel}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={highlighted >= 0 ? `${id}-option-${highlighted}` : undefined}
        autoComplete="off"
        placeholder={placeholder}
        value={value}
        onChange={(event) => {
          onChange(event.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
      />

      <div className={showPanel ? styles.panel : styles.hidden}>
        <ul ref={listRef} id={listId} role="listbox" aria-label={label} className={styles.list}>
          {showPanel
            ? results.map((player, index) => (
                <li
                  key={player.slug}
                  id={`${id}-option-${index}`}
                  role="option"
                  aria-selected={index === highlighted}
                  className={
                    index === highlighted ? `${styles.option} ${styles.on}` : styles.option
                  }
                  // Mouse down rather than click: a click lands after blur, by
                  // which time the list is closed and there is nothing to pick.
                  onMouseDown={(event) => {
                    event.preventDefault()
                    choose(player)
                  }}
                  onMouseEnter={() => setActive(index)}
                >
                  <PlayerSummary player={player} />
                </li>
              ))
            : null}
        </ul>
        {message !== '' ? <p className={styles.message}>{message}</p> : null}
      </div>

      {/* Sighted readers watch the list fill in. This is the same information
          for anyone who cannot, and deliberately not the same words as the
          message above, so nothing is read out twice. */}
      <p className="sr-only" role="status">
        {announcement(search, showPanel)}
      </p>
    </div>
  )
}

/** panelMessage is every state the list can be in that is not a list. */
function panelMessage(search: PlayerSearchState): string {
  if (search.error !== null) {
    return `The search could not run: ${search.error.message} The API may not be running.`
  }
  if (search.tooShort) return `Type at least ${MIN_QUERY} characters to search.`
  if (search.query === '') return 'Searching.'
  if (search.results.length === 0) {
    return `No player matches "${search.query}". Names follow the source data, so a surname on its own often finds more.`
  }
  return ''
}

/** announcement is what the live region says, which is a count rather than prose. */
function announcement(search: PlayerSearchState, showPanel: boolean): string {
  if (!showPanel) return ''
  if (search.error !== null) return 'The search could not run.'
  if (search.tooShort) return ''
  if (search.query === '') return 'Searching.'
  const count = search.results.length
  return `${count} player${count === 1 ? '' : 's'} match "${search.query}".`
}
