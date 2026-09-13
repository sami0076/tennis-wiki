import { mulberry32 } from './prng'
import { buildMatch, type PlaybackEvent, type SampleInput } from './sample'

export interface Playback {
  seed: number
  events: ReadonlyArray<PlaybackEvent>
}

/** One match from the chain's serve figures, replayable from its seed. */
export function buildPlayback(input: SampleInput, seed: number): Playback {
  return { seed, events: buildMatch(input, mulberry32(seed)) }
}

export { game, matchFromSets, scorelines, set, tiebreak, type Scoreline } from './closedForms'
export { announced, commentary } from './narrate'
export { newSeed } from './prng'
export { WAIT, type PlaybackEvent, type SampleInput, type SetScore, type Side, type Snapshot } from './sample'
