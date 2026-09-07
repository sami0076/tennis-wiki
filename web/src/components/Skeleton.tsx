import styles from './Skeleton.module.css'

interface SkeletonProps {
  /** How many lines the real content will occupy. */
  lines?: number
  width?: string
}

/**
 * Skeleton stands in for content on its way, at the size that content will be.
 * A spinner tells a reader to wait; this tells them what for, and stops the
 * page jumping when the answer lands.
 */
export function Skeleton({ lines = 3, width = '100%' }: SkeletonProps) {
  return (
    <div aria-hidden="true">
      {Array.from({ length: lines }, (_, index) => (
        <div
          key={index}
          className={`${styles.block} ${styles.line}`}
          style={{ width: index === lines - 1 ? '60%' : width }}
        />
      ))}
    </div>
  )
}
