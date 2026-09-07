/**
 * Surface is the one thing hue is allowed to mean. Everything that reaches for
 * a colour goes through here, so a component cannot invent a decorative one.
 */
export const SURFACES = ['hard', 'clay', 'grass', 'carpet'] as const

export type Surface = (typeof SURFACES)[number]

/**
 * surfaceVar returns the CSS custom property for a surface. An unrecorded
 * surface is --ink-3, the same grey absence is drawn in everywhere else: it is
 * not a fifth surface.
 */
export function surfaceVar(surface: string | null): string {
  switch (surface) {
    case 'hard':
      return 'var(--hard)'
    case 'clay':
      return 'var(--clay)'
    case 'grass':
      return 'var(--grass)'
    case 'carpet':
      return 'var(--carpet)'
    default:
      return 'var(--ink-3)'
  }
}

/** surfaceLabel is the text that must accompany every surface colour. */
export function surfaceLabel(surface: string | null): string {
  switch (surface) {
    case 'hard':
      return 'Hard'
    case 'clay':
      return 'Clay'
    case 'grass':
      return 'Grass'
    case 'carpet':
      return 'Carpet'
    default:
      return 'Not recorded'
  }
}
