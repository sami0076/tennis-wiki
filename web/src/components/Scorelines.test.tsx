import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Scorelines } from './Scorelines'

describe('Scorelines', () => {
  it('lists every set score most likely first, A first, and says whose win it is', () => {
    render(<Scorelines setShare={0.6} bestOf={3} nameA="Alcaraz" nameB="Sinner" />)
    const rows = screen.getAllByRole('listitem')
    expect(rows).toHaveLength(4)
    // 2-0 at 36.0%, 2-1 at 28.8%, 1-2 at 19.2%, 0-2 at 16.0%.
    expect(rows[0]).toHaveAccessibleName('Alcaraz wins 2-0: 36.0%')
    expect(rows[1]).toHaveAccessibleName('Alcaraz wins 2-1: 28.8%')
    expect(rows[2]).toHaveAccessibleName('Sinner wins 1-2: 19.2%')
    expect(rows[3]).toHaveAccessibleName('Sinner wins 0-2: 16.0%')
  })

  it('has six scores in a best of five', () => {
    render(<Scorelines setShare={0.552} bestOf={5} nameA="Alcaraz" nameB="Sinner" />)
    expect(screen.getAllByRole('listitem')).toHaveLength(6)
    expect(screen.getByText(/sets modelled as independent/i)).toBeInTheDocument()
  })
})
