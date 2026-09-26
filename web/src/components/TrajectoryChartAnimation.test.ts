import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const css = readFileSync('src/components/TrajectoryChart.module.css', 'utf8')
// Comments explain the rules and name the properties while doing it, which
// would match every assertion below.
const rules = css.replace(/\/\*[\s\S]*?\*\//g, '')

/**
 * The lines are revealed by a clip that widens from the left. A stroke dash
 * was tried first and failed twice: it outlived the animation, and it did not
 * run along the path in order, painting the two ends before the middle. Both
 * look like a hole in the data on a chart whose whole job is to show data.
 */
describe('the draw-in animation', () => {
  it('reveals with geometry, never with a stroke dash', () => {
    expect(rules).toContain('@keyframes wipe')
    expect(rules).not.toContain('stroke-dasharray')
    expect(rules).not.toContain('stroke-dashoffset')
  })

  // Holding the end state is how the first version outlived its animation.
  it('does not hold its end state', () => {
    expect(rules).toContain('animation: wipe 900ms cubic-bezier(0.16, 1, 0.3, 1) backwards')
    expect(rules).not.toMatch(/animation:[^;]*forwards/)
  })

  // Without this the clip shrinks toward the middle of the plot instead of
  // collapsing to its left edge, and the reveal opens outwards from the centre.
  it('wipes from the left edge', () => {
    expect(rules).toMatch(/\.wipe\s*\{[^}]*transform-box:\s*fill-box/)
    expect(rules).toMatch(/\.wipe\s*\{[^}]*transform-origin:\s*left/)
  })
})
