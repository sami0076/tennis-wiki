import { useCallback, useEffect, useState } from 'react'

/** Enough to be a shortcut back, short enough to be read at a glance. */
const LIMIT = 6

const STORAGE_KEY = 'deucepoint:recent-players'

export interface RecentPlayer {
  slug: string
  name: string
  tour: string
}

/**
 * The last few players this browser looked at. Per browser and nowhere else:
 * there are no accounts on this site, nothing is sent anywhere, and a reader
 * who clears their site data has cleared it.
 *
 * Storage throws outright in a private window and with site data blocked, so
 * every read and write is guarded and the list simply stays empty.
 */
function read(): RecentPlayer[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw === null) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isRecent).slice(0, LIMIT)
  } catch {
    return []
  }
}

function isRecent(value: unknown): value is RecentPlayer {
  if (typeof value !== 'object' || value === null) return false
  const row = value as Record<string, unknown>
  return typeof row.slug === 'string' && typeof row.name === 'string' && typeof row.tour === 'string'
}

/** The custom event is how two mounted readers of this list stay in step. */
const CHANGED = 'deucepoint:recent-players-changed'

/**
 * remember moves a player to the front of the list. Called from the player
 * page rather than from the palette, so the list is what was actually read
 * rather than what was searched for and abandoned.
 */
export function rememberPlayer(player: RecentPlayer) {
  try {
    const next = [player, ...read().filter((row) => row.slug !== player.slug)].slice(0, LIMIT)
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
    window.dispatchEvent(new CustomEvent(CHANGED))
  } catch {
    // A list that cannot be stored is a list that stays empty. Nothing else
    // on the page depends on it.
  }
}

export function useRecentPlayers(): { recent: RecentPlayer[]; clear: () => void } {
  const [recent, setRecent] = useState<RecentPlayer[]>(read)

  useEffect(() => {
    const refresh = () => setRecent(read())
    // "storage" fires for other tabs, the custom event for this one.
    window.addEventListener('storage', refresh)
    window.addEventListener(CHANGED, refresh)
    return () => {
      window.removeEventListener('storage', refresh)
      window.removeEventListener(CHANGED, refresh)
    }
  }, [])

  const clear = useCallback(() => {
    try {
      window.localStorage.removeItem(STORAGE_KEY)
    } catch {
      // Already gone, as far as this page is concerned.
    }
    setRecent([])
    window.dispatchEvent(new CustomEvent(CHANGED))
  }, [])

  return { recent, clear }
}
