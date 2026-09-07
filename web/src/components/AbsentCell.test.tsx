import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AbsentCell } from './AbsentCell'

describe('AbsentCell', () => {
  it('renders an em-dash, never a zero and never a blank', () => {
    render(<AbsentCell label="Aces" />)
    const cell = screen.getByLabelText('Aces: not recorded')
    expect(cell).toHaveTextContent('\u2014')
    expect(cell).not.toHaveTextContent('0')
    expect(cell.textContent?.trim()).not.toBe('')
  })

  it('says what is missing rather than only that something is', () => {
    render(<AbsentCell label="First serve won" />)
    expect(screen.getByLabelText('First serve won: not recorded')).toBeInTheDocument()
  })
})
