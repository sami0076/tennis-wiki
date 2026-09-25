import { useId, useMemo } from 'react'
import type { ReplayableDraw } from '../api/types.gen'
import { surfaceLabel } from '../lib/surface'
import styles from './DrawPicker.module.css'

interface DrawPickerProps {
  draws: ReadonlyArray<ReplayableDraw>
  /** The draw on screen, as the URL addresses it. */
  value: { event: string; season: number }
  onChange: (draw: { event: string; season: number }) => void
  /** Rendered in place of the list while it is still arriving. */
  busy?: boolean
}

/** The value a single <select> can carry for a draw that needs two fields. */
function key(draw: { event: string; season: number } | ReplayableDraw): string {
  const slug = 'slug' in draw ? draw.slug : draw.event
  return `${slug}\u0000${draw.season}`
}

/**
 * Choose which draw to replay.
 *
 * The simulator could only ever replay the draw named in the URL, and nothing
 * on the page said a URL was involved, so landing on /simulator meant the
 * featured draw and nothing else. This is the missing control.
 *
 * One select rather than a season picker and an event picker: two controls
 * where the second depends on the first is two states to get wrong, and the
 * seasons are the optgroups, which is what optgroups are for. The select
 * carries both halves of the address in one value because a draw is a slug
 * *and* a season, and a control that held only one of them could name a draw
 * that was never played.
 */
export function DrawPicker({ draws, value, onChange, busy = false }: DrawPickerProps) {
  const id = useId()

  // The list arrives newest-first and already in the order a reader wants
  // within a season, so the groups fall out of a single pass.
  const seasons = useMemo(() => {
    const bySeason = new Map<number, ReplayableDraw[]>()
    for (const draw of draws) {
      const group = bySeason.get(draw.season)
      if (group === undefined) bySeason.set(draw.season, [draw])
      else group.push(draw)
    }
    return [...bySeason.entries()]
  }, [draws])

  const selected = key(value)
  // The draw on screen may not be in the list: the featured one is addressed
  // by a slug this list would only carry if it also passed the eligibility
  // cut. Keeping the option means the select never shows a blank field for
  // the thing it is currently showing.
  const known = draws.some((draw) => key(draw) === selected)

  return (
    <div className={styles.picker}>
      <label className={styles.label} htmlFor={id}>
        Draw
      </label>
      <select
        id={id}
        className={styles.select}
        value={selected}
        disabled={busy || draws.length === 0}
        onChange={(event) => {
          const [slug, season] = event.target.value.split('\u0000')
          if (slug === undefined || season === undefined) return
          onChange({ event: slug, season: Number(season) })
        }}
      >
        {known ? null : <option value={selected}>Showing this draw</option>}
        {seasons.map(([season, group]) => (
          <optgroup key={season} label={String(season)}>
            {group.map((draw) => (
              <option key={key(draw)} value={key(draw)}>
                {describe(draw)}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
      {busy ? <span className={styles.busy}>Loading draws…</span> : null}
    </div>
  )
}

/**
 * One line a reader can choose between: the event, the tour it belongs to,
 * the surface, and how big the draw was. The size matters because a 128 draw
 * and a 32 draw are very different things to replay, and the source's own
 * figure is used where it recorded one.
 */
function describe(draw: ReplayableDraw): string {
  const size = draw.draw_size === null ? `${draw.matches} matches` : `${draw.draw_size} draw`
  const surface = draw.surface === 'unknown' ? null : surfaceLabel(draw.surface)
  return [draw.name, draw.tour.toUpperCase(), surface, size].filter(Boolean).join(' · ')
}
