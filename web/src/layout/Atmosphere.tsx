import { useEffect, useRef } from 'react'
import styles from './Atmosphere.module.css'

/** Soft coloured light drifting behind every page. Decorative only. */
export function Backdrop() {
  return (
    <div className={styles.backdrop} aria-hidden="true">
      <span className={`${styles.orb} ${styles.violet}`} />
      <span className={`${styles.orb} ${styles.magenta}`} />
      <span className={`${styles.orb} ${styles.lime}`} />
      <span className={styles.grain} />
    </div>
  )
}

/** A lime bar across the top that fills as the page scrolls. */
export function ScrollProgress() {
  const bar = useRef<HTMLDivElement>(null)
  useEffect(() => {
    let frame = 0
    const update = () => {
      frame = 0
      const max = document.documentElement.scrollHeight - window.innerHeight
      const share = max > 0 ? window.scrollY / max : 0
      bar.current?.style.setProperty('--progress', String(share))
    }
    const onScroll = () => {
      if (frame === 0) frame = requestAnimationFrame(update)
    }
    update()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => {
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
      cancelAnimationFrame(frame)
    }
  }, [])
  return <div ref={bar} className={styles.progress} aria-hidden="true" />
}
