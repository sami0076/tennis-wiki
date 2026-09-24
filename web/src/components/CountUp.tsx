import { useEffect, useState } from 'react'
import { useInView } from '../lib/useInView'
import { prefersReducedMotion } from '../lib/useReducedMotion'

interface CountUpProps {
  value: number
  format?: (value: number) => string
  duration?: number
  className?: string
}

/**
 * CountUp runs a figure up from zero when it first comes into view. The
 * accessible text is always the final figure, and so is the first paint
 * wherever there is nothing to animate with.
 */
export function CountUp({ value, format = (v) => String(Math.round(v)), duration = 1100, className }: CountUpProps) {
  const [ref, seen] = useInView<HTMLSpanElement>()
  const animates = typeof IntersectionObserver !== 'undefined' && !prefersReducedMotion()
  const [shown, setShown] = useState(animates ? 0 : value)

  useEffect(() => {
    if (!animates) {
      setShown(value)
      return
    }
    if (!seen) return
    let frame = 0
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      setShown(value * (1 - Math.pow(1 - t, 4)))
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    // A throttled tab can starve animation frames; the figure must still land.
    const settle = window.setTimeout(() => setShown(value), duration + 150)
    return () => {
      cancelAnimationFrame(frame)
      window.clearTimeout(settle)
    }
  }, [seen, value, duration, animates])

  if (!animates) {
    return (
      <span ref={ref} className={className}>
        {format(value)}
      </span>
    )
  }

  return (
    <span ref={ref} className={className}>
      <span aria-hidden="true">{format(shown)}</span>
      <span className="sr-only">{format(value)}</span>
    </span>
  )
}
