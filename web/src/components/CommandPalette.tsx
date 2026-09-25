import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import type { PlayerSearchResult } from '../api/types.gen'
import { MIN_QUERY, usePlayerSearch } from '../api/useSearch'
import { useRecentPlayers, type RecentPlayer } from '../lib/useRecentPlayers'
import { tierLabel } from '../lib/tier'
import { Meta } from './Meta'
import styles from './CommandPalette.module.css'

/** Where the palette can send a reader without a name being typed. */
const PAGES: ReadonlyArray<{ to: string; label: string; hint: string; keys: string }> = [
  { to: '/', label: 'Home', hint: 'Leaders, last week, coverage', keys: 'home start' },
  { to: '/players', label: 'Players', hint: 'Search 115,000 careers', keys: 'players search' },
  { to: '/h2h', label: 'Head to head', hint: 'Compare two players', keys: 'h2h rivalry versus' },
  { to: '/rankings', label: 'Rankings', hint: 'Elo against the official list', keys: 'rankings elo' },
  { to: '/leaders', label: 'Leaderboards', hint: 'Every rate, ranked', keys: 'leaders stats boards' },
  { to: '/tournaments', label: 'Tournaments', hint: 'Draws and editions', keys: 'tournaments draws events' },
  { to: '/seasons', label: 'Seasons', hint: 'A year at a time', keys: 'seasons years calendar' },
  { to: '/simulator', label: 'Simulator', hint: 'Point to match, and a draw', keys: 'simulator simulate odds' },
  { to: '/methodology', label: 'Methodology', hint: 'How every number is produced', keys: 'methodology how working' },
]

/**
 * What the palette is currently doing.
 *
 * - browsing: one field over players and pages.
 * - acting: a player is chosen and the list is what can be done to them.
 * - pairing: a player is chosen, an action needs a second one, and the field
 *   is searching for the opponent.
 *
 * Pairing is the reason this component exists rather than a second search box
 * in the header. A rivalry page needs two names, and every other route to it
 * is two searches on two pages.
 */
type Mode =
  | { kind: 'browsing' }
  | { kind: 'acting'; subject: RecentPlayer }
  | { kind: 'pairing'; subject: RecentPlayer; action: 'h2h' | 'simulate' }

/**
 * One row. Everything in the list is one of these, which is what lets the
 * arrow keys walk the whole list without knowing what each row is.
 */
interface Row {
  key: string
  kicker: string
  label: string
  hint?: ReactNode
  /** The player this row is about, where it is about one: the Tab target. */
  player?: RecentPlayer
  run: () => void
}

interface CommandPaletteProps {
  open: boolean
  onClose: () => void
}

function asRecent(player: PlayerSearchResult | RecentPlayer): RecentPlayer {
  return { slug: player.slug, name: player.name, tour: player.tour }
}

/**
 * The command palette: one field that reaches every player, every page and the
 * two comparisons the site is built on.
 *
 * Opened with ⌘K or Ctrl-K anywhere, and with / when nothing else has focus.
 * Arrows move, Enter runs, Tab turns the highlighted player into the subject
 * of a second step, Escape goes back one step and then closes.
 */
export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const [mode, setMode] = useState<Mode>({ kind: 'browsing' })
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const { recent } = useRecentPlayers()

  // Searching is pointless while the list is three fixed actions, and a
  // request per keystroke against an ignored list is a request wasted.
  const searchable = mode.kind !== 'acting'
  const search = usePlayerSearch(searchable ? query : '', { limit: 7 })

  const go = useCallback(
    (to: string) => {
      onClose()
      navigate(to)
    },
    [navigate, onClose],
  )

  const rows = useMemo<Row[]>(() => {
    if (mode.kind === 'acting') return actionRows(mode.subject, go, setMode, setQuery)
    if (mode.kind === 'pairing') return pairingRows(mode, search.results, go)

    const trimmed = query.trim()
    const out: Row[] = []

    // With nothing typed the list is where this browser has already been,
    // which is the fastest thing a palette can offer.
    if (trimmed === '') {
      for (const player of recent) {
        out.push({
          key: `recent-${player.slug}`,
          kicker: 'Recent',
          label: player.name,
          hint: player.tour.toUpperCase(),
          player,
          run: () => go(`/players/${player.slug}`),
        })
      }
    }

    for (const player of search.results) {
      out.push({
        key: `player-${player.slug}`,
        kicker: 'Player',
        label: player.name,
        hint: <PlayerHint player={player} />,
        player: asRecent(player),
        run: () => go(`/players/${player.slug}`),
      })
    }

    const needle = trimmed.toLowerCase()
    for (const page of PAGES) {
      if (needle !== '' && !`${page.label} ${page.keys}`.toLowerCase().includes(needle)) continue
      out.push({
        key: `page-${page.to}`,
        kicker: 'Go to',
        label: page.label,
        hint: page.hint,
        run: () => go(page.to),
      })
    }
    return out
  }, [mode, query, recent, search.results, go])

  // A new list is a new first row. Leaving the highlight on a stale index
  // would have Enter open whatever took that row's place, which is the one
  // mistake a palette must never make.
  useEffect(() => setActive(0), [rows.length, mode])

  useEffect(() => {
    if (!open) return
    setQuery('')
    setMode({ kind: 'browsing' })
    setActive(0)
    // After the dialog is on screen, or the browser puts focus back.
    const frame = requestAnimationFrame(() => inputRef.current?.focus())
    return () => cancelAnimationFrame(frame)
  }, [open])

  useEffect(() => {
    listRef.current?.children[active]?.scrollIntoView({ block: 'nearest' })
  }, [active, rows.length])

  if (!open) return null

  const highlighted = rows[active]
  const pickable = mode.kind === 'browsing' ? highlighted?.player : undefined

  function onKeyDown(event: React.KeyboardEvent) {
    switch (event.key) {
      case 'Escape':
        event.preventDefault()
        if (mode.kind === 'pairing') setMode({ kind: 'acting', subject: mode.subject })
        else if (mode.kind === 'acting') setMode({ kind: 'browsing' })
        else onClose()
        setQuery('')
        return
      case 'ArrowDown':
        event.preventDefault()
        setActive((i) => (rows.length === 0 ? 0 : (i + 1) % rows.length))
        return
      case 'ArrowUp':
        event.preventDefault()
        setActive((i) => (rows.length === 0 ? 0 : (i - 1 + rows.length) % rows.length))
        return
      case 'Enter':
        event.preventDefault()
        rows[active]?.run()
        return
      case 'Tab':
        // Tab is the second step, not a focus move: the palette is one field
        // and one list, so there is nowhere else for focus to go.
        if (pickable === undefined) return
        event.preventDefault()
        setMode({ kind: 'acting', subject: pickable })
        setQuery('')
        return
      case 'Backspace':
        if (mode.kind !== 'browsing' && query === '') {
          event.preventDefault()
          if (mode.kind === 'pairing') setMode({ kind: 'acting', subject: mode.subject })
          else setMode({ kind: 'browsing' })
        }
        return
      default:
        return
    }
  }

  const subject = mode.kind === 'browsing' ? null : mode.subject

  return (
    <div
      className={styles.scrim}
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <div
        className={styles.panel}
        role="dialog"
        aria-modal="true"
        aria-label="Search and commands"
        onKeyDown={onKeyDown}
      >
        <div className={styles.field}>
          {subject === null ? (
            <span className={styles.glyph} aria-hidden="true">
              ⌕
            </span>
          ) : (
            <span className={styles.subject}>
              {subject.name}
              {mode.kind === 'pairing' ? (
                <span className={styles.versus}>{mode.action === 'h2h' ? ' v' : ' vs'}</span>
              ) : null}
            </span>
          )}
          <input
            ref={inputRef}
            className={styles.input}
            type="text"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={placeholderFor(mode)}
            aria-label={placeholderFor(mode)}
            aria-autocomplete="list"
            aria-controls="dp-palette-list"
            aria-activedescendant={highlighted === undefined ? undefined : `dp-row-${active}`}
            autoComplete="off"
            spellCheck={false}
          />
          <kbd className={styles.esc}>esc</kbd>
        </div>

        <ul className={styles.list} id="dp-palette-list" role="listbox" ref={listRef}>
          {rows.map((row, index) => (
            <li
              key={row.key}
              id={`dp-row-${index}`}
              role="option"
              aria-selected={index === active}
              className={index === active ? `${styles.row} ${styles.on}` : styles.row}
              onMouseMove={() => setActive(index)}
              onMouseDown={(event) => {
                event.preventDefault()
                row.run()
              }}
            >
              <span className={styles.kicker}>{row.kicker}</span>
              <span className={styles.label}>{row.label}</span>
              <span className={styles.hint}>{row.hint}</span>
            </li>
          ))}
        </ul>

        <Foot
          empty={rows.length === 0}
          tooShort={searchable && query.trim() !== '' && query.trim().length < MIN_QUERY}
          busy={search.busy}
          canPick={pickable !== undefined}
          nested={mode.kind !== 'browsing'}
        />
      </div>
    </div>
  )
}

function placeholderFor(mode: Mode): string {
  switch (mode.kind) {
    case 'acting':
      return 'Pick what to do'
    case 'pairing':
      return mode.action === 'h2h' ? 'Search the other player' : 'Search the opponent'
    default:
      return 'Player, page or command'
  }
}

/** What can be done to a player the palette has already found. */
function actionRows(
  subject: RecentPlayer,
  go: (to: string) => void,
  setMode: (mode: Mode) => void,
  setQuery: (query: string) => void,
): Row[] {
  const step = (action: 'h2h' | 'simulate') => () => {
    setMode({ kind: 'pairing', subject, action })
    setQuery('')
  }
  return [
    {
      key: 'open',
      kicker: 'Open',
      label: `${subject.name}'s page`,
      hint: 'Career, ratings, every match',
      run: () => go(`/players/${subject.slug}`),
    },
    {
      key: 'compare',
      kicker: 'Compare',
      label: 'Head to head against…',
      hint: 'Then pick the other player',
      run: step('h2h'),
    },
    {
      key: 'simulate',
      kicker: 'Simulate',
      label: 'A match against…',
      hint: 'Point to match, from the ratings',
      run: step('simulate'),
    },
  ]
}

/**
 * The second half of a pair. The subject is excluded from its own list: a
 * player cannot be compared with themselves, and the API says so with a 400
 * rather than a page.
 */
function pairingRows(
  mode: { subject: RecentPlayer; action: 'h2h' | 'simulate' },
  results: PlayerSearchResult[],
  go: (to: string) => void,
): Row[] {
  return results
    .filter((player) => player.slug !== mode.subject.slug)
    .map((player) => ({
      key: `pair-${player.slug}`,
      kicker: mode.action === 'h2h' ? 'Compare' : 'Simulate',
      label: player.name,
      hint: <PlayerHint player={player} />,
      run: () =>
        go(
          mode.action === 'h2h'
            ? `/h2h/${mode.subject.slug}/${player.slug}`
            : `/simulator?a=${encodeURIComponent(mode.subject.slug)}&b=${encodeURIComponent(player.slug)}`,
        ),
    }))
}

function PlayerHint({ player }: { player: PlayerSearchResult }) {
  return (
    <Meta
      parts={[
        player.tour.toUpperCase(),
        player.country,
        player.matches === 0 ? 'no matches' : `${player.matches} matches`,
        tierLabel(player.best_tier),
      ]}
    />
  )
}

function Foot({
  empty,
  tooShort,
  busy,
  canPick,
  nested,
}: {
  empty: boolean
  tooShort: boolean
  busy: boolean
  canPick: boolean
  nested: boolean
}) {
  let message: string | null = null
  if (tooShort) message = 'Two letters or more to search.'
  else if (empty && busy) message = 'Searching…'
  else if (empty) message = 'Nothing matches that. Try fewer letters, or a page name.'

  if (message !== null) {
    return (
      <div className={styles.foot}>
        <span className={styles.message}>{message}</span>
      </div>
    )
  }

  return (
    <div className={styles.foot}>
      <span className={styles.keys}>
        <kbd>↑</kbd>
        <kbd>↓</kbd>
        <span>move</span>
        <kbd>↵</kbd>
        <span>open</span>
        {canPick ? (
          <>
            <kbd>tab</kbd>
            <span>compare</span>
          </>
        ) : null}
        {nested ? (
          <>
            <kbd>esc</kbd>
            <span>back</span>
          </>
        ) : null}
      </span>
    </div>
  )
}
