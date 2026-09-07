import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import {
  AvailabilityNeverForTier,
  AvailabilityNeverInEra,
  AvailabilityNotRecorded,
} from '../api/client'
import { absenceReason } from '../lib/absence'
import { ButtonLink } from './Button'
import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('names what is absent and why', () => {
    render(
      <EmptyState heading="No serve statistics for this career" reason="Not kept before 1991." />,
    )
    expect(screen.getByRole('heading', { name: 'No serve statistics for this career' })).toBeInTheDocument()
    expect(screen.getByText('Not kept before 1991.')).toBeInTheDocument()
  })

  it('offers somewhere with data, because emptiness is direction', () => {
    render(
      <MemoryRouter>
        <EmptyState
          heading="Nothing here"
          reason="Because of the era."
          action={<ButtonLink to="/">Back to coverage</ButtonLink>}
        />
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: 'Back to coverage' })).toHaveAttribute('href', '/')
  })
})

// The whole data model exists to keep these apart. If two of them ever render
// the same sentence, the API's three-way distinction has been thrown away at
// the last step.
describe('the three absences', () => {
  it('reads differently for each kind', () => {
    const era = absenceReason(AvailabilityNeverInEra)
    const tier = absenceReason(AvailabilityNeverForTier)
    const unknown = absenceReason(AvailabilityNotRecorded)

    expect(new Set([era, tier, unknown]).size).toBe(3)
    expect(tier).toMatch(/Futures/)
    expect(era).toMatch(/1991/)
  })

  it('never explains an absence as a zero', () => {
    for (const availability of [
      AvailabilityNeverInEra,
      AvailabilityNeverForTier,
      AvailabilityNotRecorded,
    ]) {
      expect(absenceReason(availability)).not.toMatch(/\bzero\b|\b0\b/)
    }
  })
})
