import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RankDelta } from './RankDelta'
import { SurfaceDot } from './SurfaceDot'
import { WinLossMark } from './WinLossMark'
import { Sparkline } from './Sparkline'

// The quality floor: colour is never the only encoding. Each of these carries a
// character or a word that says the same thing the colour does.
describe('colour is never the only encoding', () => {
  it('WinLossMark always shows the letter', () => {
    const { rerender } = render(<WinLossMark won={true} />)
    expect(screen.getByText('W')).toBeInTheDocument()
    rerender(<WinLossMark won={false} />)
    expect(screen.getByText('L')).toBeInTheDocument()
  })

  it('RankDelta always shows the sign', () => {
    const { rerender } = render(<RankDelta delta={12} />)
    expect(screen.getByText('+12')).toBeInTheDocument()
    rerender(<RankDelta delta={-4} />)
    expect(screen.getByText('-4')).toBeInTheDocument()
    rerender(<RankDelta delta={0} />)
    expect(screen.getByText('0')).toBeInTheDocument()
  })

  it('SurfaceDot always names the surface, even with the label hidden', () => {
    render(<SurfaceDot surface="clay" label={false} />)
    expect(screen.getByText('Clay')).toBeInTheDocument()
  })

  it('SurfaceDot calls an unrecorded surface unrecorded, not a fifth surface', () => {
    render(<SurfaceDot surface={null} />)
    expect(screen.getByText('Not recorded')).toBeInTheDocument()
  })
})

describe('Sparkline', () => {
  it('draws nothing for a series too short to be a line', () => {
    const { container } = render(<Sparkline points={[{ date: '2020-01-01', elo: 1500 }]} label="Elo" />)
    expect(container.querySelector('svg')).toBeNull()
  })

  it('survives a flat series without dividing by zero', () => {
    render(
      <Sparkline
        points={[
          { date: '2020-01-01', elo: 1500 },
          { date: '2020-02-01', elo: 1500 },
        ]}
        label="Elo, unchanged"
      />,
    )
    const path = screen.getByRole('img', { name: 'Elo, unchanged' }).querySelector('path')
    expect(path?.getAttribute('d')).not.toMatch(/NaN/)
  })
})
