import type { PlaybackEvent, Side } from './sample'

/**
 * The line under the board for one beat of the match, in the prototype's
 * words. Each phrase follows what the sample did; nothing is decoration.
 */
export function commentary(event: PlaybackEvent, names: readonly [string, string]): string {
  const name = (side: Side) => names[side]
  switch (event.kind) {
    case 'breakPoint':
      return `Break point, ${name(event.side)}`
    case 'hold':
      switch (event.how) {
        case 'love':
          return `${name(event.side)} holds comfortably`
        case 'deuce':
          return `${name(event.side)} holds from deuce`
        case 'front':
          return `${name(event.side)} holds to stay in front`
        default:
          return `${name(event.side)} holds serve`
      }
    case 'break':
      return `${name(event.side)} breaks`
    case 'tiebreak':
      return 'Tiebreak at 6-6'
    case 'tiebreakWon': {
      const [a, b] = event.points
      return `${name(event.side)} takes the tiebreak ${Math.max(a, b)}-${Math.min(a, b)}`
    }
    case 'set':
      return `Set ${event.number} to ${name(event.side)}`
    case 'match':
      return `Game, set, match: ${name(event.side)} wins ${event.sets[0]}-${event.sets[1]}`
  }
}

/** Whether a beat is one a screen reader should hear: sets and the result. */
export function announced(event: PlaybackEvent): boolean {
  return event.kind === 'set' || event.kind === 'match'
}
