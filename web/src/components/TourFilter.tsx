import styles from './TourFilter.module.css'

interface TourFilterProps {
  /** null is both tours. */
  value: string | null
  onChange: (tour: string | null) => void
}

const tours = [
  { value: null, label: 'Both tours' },
  { value: 'atp', label: 'ATP' },
  { value: 'wta', label: 'WTA' },
]

/**
 * TourFilter narrows a search to one tour. Same shape as SurfaceToggle and
 * deliberately monochrome: hue means surface, and a tour is not one.
 */
export function TourFilter({ value, onChange }: TourFilterProps) {
  return (
    <div className={styles.row} role="group" aria-label="Filter by tour">
      {tours.map((tour) => {
        const active = value === tour.value
        return (
          <button
            key={tour.label}
            type="button"
            className={active ? `${styles.option} ${styles.active}` : styles.option}
            aria-pressed={active}
            onClick={() => onChange(tour.value)}
          >
            {tour.label}
          </button>
        )
      })}
    </div>
  )
}
