/**
 * spread keeps a set of positions at least `gap` apart, moving the later ones
 * down, then pulls the whole run back up if it ran off the bottom.
 */
export function spread(positions: ReadonlyArray<number>, gap: number): number[] {
  const order = positions
    .map((value, index) => ({ value, index }))
    .sort((a, b) => a.value - b.value)
  const placed: number[] = []
  for (const item of order) {
    const last = placed[placed.length - 1]
    placed.push(last === undefined ? item.value : Math.max(item.value, last + gap))
  }
  const overflow = (placed[placed.length - 1] ?? 0) - 100
  const out = new Array<number>(positions.length)
  order.forEach((item, rank) => {
    out[item.index] = placed[rank]! - Math.max(0, overflow)
  })
  return out
}
