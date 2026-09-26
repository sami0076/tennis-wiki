/**
 * Ticks for an axis: round numbers inside the range, not the range's own ends
 * divided into fifths. 2632, 2547, 2462 are arithmetic; 2600, 2500, 2400 are
 * numbers a reader already holds in their head.
 */
export function niceTicks(min: number, max: number, target = 4): number[] {
  const span = max - min
  if (!Number.isFinite(span) || span <= 0) return [min]

  // The step is the first of these at or above the raw interval, so the ticks
  // land on figures that read as figures.
  const raw = span / target
  const magnitude = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= raw) ?? 10 * magnitude

  const ticks: number[] = []
  for (let v = Math.ceil(min / step) * step; v <= max + step / 1000; v += step) {
    // Floating point leaves 2499.9999999 where 2500 belongs.
    ticks.push(Math.round(v * 1000) / 1000)
  }
  return ticks
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const DAY = 86_400_000

export interface TimeTick {
  at: number
  label: string
}

/**
 * Ticks for a time axis, on the boundaries a reader looks for: whole years
 * over a long window, month starts over a season, plain dates over weeks.
 *
 * Evenly spaced ticks would be easier and would read "14 Mar 2024, 27 Jul
 * 2024", which is four labels nobody can compare. A year boundary is a label
 * that means something on its own.
 */
export function timeTicks(from: number, to: number, target = 4): TimeTick[] {
  const span = to - from
  if (!Number.isFinite(span) || span <= 0) return []

  // Whole years read best -- a year needs no other label to mean something --
  // but only when enough of them fall inside the window. Fourteen months
  // contains one January, and one tick is not an axis. A surface view is
  // routinely that long: a clay rating only moves during the clay swing, so
  // its window runs from one spring to the next.
  const years = yearTicks(from, to, target)
  if (years.length >= 3) return years

  return span / DAY > 80 ? monthTicks(from, to, target) : dayTicks(from, to, target)
}

function yearTicks(from: number, to: number, target: number): TimeTick[] {
  // Start at the first January inside the window rather than at the window's
  // own year, whose January is usually behind it and would be dropped --
  // spending one of the four ticks on nothing.
  const start = new Date(from)
  const firstYear =
    start.getUTCFullYear() + (start.getTime() > Date.UTC(start.getUTCFullYear(), 0, 1) ? 1 : 0)
  const lastYear = new Date(to).getUTCFullYear()
  const every = Math.max(1, Math.ceil((lastYear - firstYear + 1) / target))

  const ticks: TimeTick[] = []
  for (let y = firstYear; y <= lastYear; y += every) {
    const at = Date.UTC(y, 0, 1)
    if (at >= from && at <= to) ticks.push({ at, label: String(y) })
  }
  return ticks
}

function monthTicks(from: number, to: number, target: number): TimeTick[] {
  const start = new Date(from)
  const every = Math.max(1, Math.ceil(from === to ? 1 : (to - from) / DAY / 30 / target))

  const ticks: TimeTick[] = []
  for (let i = 0; i < 64; i++) {
    const d = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + i * every, 1))
    const at = d.getTime()
    if (at > to) break
    if (at < from) continue
    // The year is carried by the first tick and by every tick that crosses
    // into a new one. Naming it only in January loses it entirely on a window
    // whose ticks happen to step over January, which is how a chart spanning
    // two years ends up saying which year none of it is.
    const year = d.getUTCFullYear()
    const turned = ticks.length === 0 || year !== new Date(ticks[ticks.length - 1]!.at).getUTCFullYear()
    const month = MONTHS[d.getUTCMonth()]!
    ticks.push({ at, label: !turned ? month : d.getUTCMonth() === 0 ? String(year) : `${month} ${year}` })
  }
  return ticks
}

function dayTicks(from: number, to: number, target: number): TimeTick[] {
  const ticks: TimeTick[] = []
  for (let i = 0; i <= target; i++) {
    const at = from + ((to - from) * i) / target
    const d = new Date(at)
    ticks.push({ at, label: `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}` })
  }
  return ticks
}

/**
 * A rating line is weekly snapshots for the weeks a player was rated, and
 * nothing at all for the weeks they were not. Joining two points either side
 * of a long absence draws a smooth climb or slide that never happened, so the
 * line is cut instead and the absence is left blank.
 *
 * Six months, because the off-season is about two and a schedule built round
 * the Slams alone can leave three between appearances. Half a year away is not
 * a gap in a career, it is an absence from it.
 */
export const minAbsenceDays = 180

/**
 * ...but six months is only an absence relative to how often this line is
 * sampled at all. A series carrying a point a year is not absent between them,
 * it is annual, and cutting it at every step would shatter it into dots. So the
 * floor above is raised to a multiple of the line's own usual spacing, and a
 * gap has to dwarf that to count.
 */
export const absenceFactor = 4

/** Nearest-rank, so the result is always one of the observations. */
function quantile(numbers: number[], q: number): number {
  if (numbers.length === 0) return 0
  const sorted = [...numbers].sort((a, b) => a - b)
  const rank = Math.max(1, Math.ceil(q * sorted.length))
  return sorted[rank - 1]!
}

export function splitOnGaps<T>(points: ReadonlyArray<T>, at: (point: T) => number): T[][] {
  if (points.length === 0) return []

  const steps = points.slice(1).map((point, index) => at(point) - at(points[index]!))
  // A low quantile rather than the median: the gap being looked for is itself
  // one of the steps, and on a short series it drags a median up past itself.
  // Two points six years apart have a median step of six years and would never
  // look anomalous against it.
  //
  // Under three steps there is no cadence to infer -- the one step *is* the
  // spacing -- so the absolute floor answers alone.
  const limit =
    steps.length < 3
      ? minAbsenceDays * DAY
      : Math.max(minAbsenceDays * DAY, absenceFactor * quantile(steps, 0.25))

  const runs: T[][] = []
  let run: T[] = []
  for (const point of points) {
    const last = run[run.length - 1]
    if (last !== undefined && at(point) - at(last) > limit) {
      runs.push(run)
      run = []
    }
    run.push(point)
  }
  if (run.length > 0) runs.push(run)
  return runs
}
