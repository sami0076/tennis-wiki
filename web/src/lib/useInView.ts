import { useEffect, useRef, useState } from 'react'

/**
 * Whether the element has scrolled into view, latched once true. Without an
 * IntersectionObserver (jsdom, old browsers) it starts true, so nothing waits
 * on a signal that will never come.
 */
export function useInView<T extends Element>(margin = '0px 0px -10% 0px') {
  const ref = useRef<T>(null)
  const [seen, setSeen] = useState(() => typeof IntersectionObserver === 'undefined')

  useEffect(() => {
    if (seen || ref.current === null) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setSeen(true)
          observer.disconnect()
        }
      },
      { rootMargin: margin },
    )
    observer.observe(ref.current)
    return () => observer.disconnect()
  }, [seen, margin])

  return [ref, seen] as const
}
