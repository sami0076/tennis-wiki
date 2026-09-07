import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PartialAggregate } from './PartialAggregate'

describe('PartialAggregate', () => {
  it('declares its denominator, which is the entire point', () => {
    render(
      <PartialAggregate recorded={41} total={68}>
        <p>12.4 aces</p>
      </PartialAggregate>,
    )
    expect(screen.getByText(/41 of 68 matches/)).toBeInTheDocument()
  })

  it('says so plainly when nothing is missing', () => {
    render(
      <PartialAggregate recorded={68} total={68}>
        <p>12.4 aces</p>
      </PartialAggregate>,
    )
    expect(screen.getByText(/all 68 matches/)).toBeInTheDocument()
  })
})
