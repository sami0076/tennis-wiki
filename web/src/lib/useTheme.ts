import { useCallback, useEffect, useState } from 'react'

/**
 * Three states, not two. "system" is the default and follows the reader's
 * machine; light and dark are a choice that overrides it. Collapsing these
 * into a boolean would make a reader who has chosen nothing indistinguishable
 * from one who has chosen the theme their machine happens to be set to, and
 * their site would stop following them when they switch at dusk.
 */
export type Theme = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'deucepoint:theme'

/** The order the toggle cycles through, which is also the order it reads. */
export const THEMES: readonly Theme[] = ['system', 'light', 'dark']

function isTheme(value: string | null): value is Theme {
  return value === 'system' || value === 'light' || value === 'dark'
}

/**
 * Read the stored choice. Storage can throw outright in a private window or
 * with site data blocked, so every read and write is guarded: a reader with no
 * storage gets the system theme rather than a blank page.
 */
function readStored(): Theme {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    return isTheme(raw) ? raw : 'system'
  } catch {
    return 'system'
  }
}

/**
 * Apply writes the choice to the document. "system" removes the attribute
 * rather than resolving it here, because the stylesheet already answers the
 * media query and a resolved value would need re-resolving on every change.
 */
function apply(theme: Theme) {
  const root = document.documentElement
  if (theme === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', theme)
}

/**
 * useTheme is the whole theme: the stored choice, a setter that persists it,
 * and the theme that is actually on screen right now, which is what a label
 * has to say when the choice is "system".
 */
export function useTheme(): {
  theme: Theme
  resolved: 'light' | 'dark'
  setTheme: (next: Theme) => void
} {
  const [theme, setStored] = useState<Theme>(readStored)
  const [systemDark, setSystemDark] = useState(
    () => window.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false,
  )

  useEffect(() => apply(theme), [theme])

  // Following the system means following it while the page is open, not only
  // at load: a machine that switches at sunset switches the page with it.
  useEffect(() => {
    const query = window.matchMedia?.('(prefers-color-scheme: dark)')
    if (query === undefined) return
    const onChange = (event: MediaQueryListEvent) => setSystemDark(event.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])

  const setTheme = useCallback((next: Theme) => {
    setStored(next)
    try {
      if (next === 'system') window.localStorage.removeItem(STORAGE_KEY)
      else window.localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // A choice that cannot be stored still applies to this page.
    }
  }, [])

  const resolved = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme
  return { theme, resolved, setTheme }
}
