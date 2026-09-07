import {
  AvailabilityNeverForTier,
  AvailabilityNeverInEra,
  AvailabilityPartial,
  AvailabilityRecorded,
} from '../api/client'

/**
 * The API distinguishes three kinds of absence, and this is the one place that
 * turns each into a sentence. Keeping the wording here rather than in the
 * components is what stops two screens explaining the same gap differently.
 *
 * The first rule of the data model, restated for the client: a statistic that
 * was never recorded is not a zero, and it is never hidden either.
 */
export function absenceReason(availability: string): string {
  switch (availability) {
    case AvailabilityNeverForTier:
      return 'No Futures or ITF match has ever recorded serve statistics, in any year.'
    case AvailabilityNeverInEra:
      return 'Serve statistics were not kept before 1991, or before roughly 2010 at Challenger level.'
    case AvailabilityPartial:
      return 'Some of these matches recorded serve statistics and some did not.'
    default:
      return 'The source did not record serve statistics for these matches.'
  }
}

/** hasStatistics reports whether there is anything to render at all. */
export function hasStatistics(availability: string): boolean {
  return availability === AvailabilityRecorded || availability === AvailabilityPartial
}
