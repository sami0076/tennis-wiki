import { useEffect, useState } from 'react'

/**
 * useDebounced trails a value by `ms`, so a query typed into a field is one
 * request rather than one per keystroke.
 */
export function useDebounced<T>(value: T, ms = 180): T {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return settled
}
