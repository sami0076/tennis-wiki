import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// Read as text rather than imported: the point is what the stylesheet declares,
// and importing it would hand back the CSS-module class map instead.
const css = readFileSync('src/components/TrajectoryChart.module.css', 'utf8')
// Comments explain the rule and name the property while doing it, which would
// match every assertion below.
const rules = css.replace(/\/\*[\s\S]*?\*\//g, '')

/**
 * The draw-in animation hides the line by dashing it and then reveals it. If
 * the dash is declared on the element rather than inside the keyframes, it
 * outlives the animation: anything that stops the animation finishing leaves a
 * stroke pattern over real data, and a chart with a hole in it is
 * indistinguishable from a gap in the database. It cost a long debugging
 * session to find that once.
 */
describe('the draw-in animation', () => {
  it('declares the dash only inside the keyframes', () => {
    const keyframes = rules.slice(rules.indexOf('@keyframes draw'))
    const outside = rules.slice(0, rules.indexOf('@keyframes draw'))
    expect(keyframes).toContain('stroke-dasharray')
    expect(outside).not.toContain('stroke-dasharray')
    expect(outside).not.toContain('stroke-dashoffset')
  })

  // forwards would hold the last keyframe, dash and all, which is the thing
  // above by another route.
  it('does not hold its end state', () => {
    expect(rules).toContain('animation: draw 900ms cubic-bezier(0.16, 1, 0.3, 1) backwards')
    expect(rules).not.toMatch(/animation: draw[^;]*forwards/)
  })
})
