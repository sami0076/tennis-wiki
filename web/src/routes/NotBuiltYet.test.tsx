import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { NotBuiltYet } from './NotBuiltYet'

function show(ui: React.ReactElement) {
  render(<MemoryRouter>{ui}</MemoryRouter>)
}

describe('NotBuiltYet', () => {
  it('names the issue when the API is already there', () => {
    show(<NotBuiltYet page="rankings page" issue={45} />)
    expect(screen.getByText(/issue #45/)).toBeInTheDocument()
  })

  // Claiming an API exists when it does not is the same mistake as rendering an
  // absent statistic as a zero: a plausible answer in place of the true one.
  it('does not claim an API exists when nothing is built behind the page', () => {
    show(<NotBuiltYet page="simulator" note="Simulation is Phase 3." />)
    expect(screen.getByText(/Simulation is Phase 3/)).toBeInTheDocument()
    expect(screen.queryByText(/The API behind it exists/)).not.toBeInTheDocument()
  })
})
