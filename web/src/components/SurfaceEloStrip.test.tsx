import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { SeriesRating } from '../api/client'
import { SurfaceEloStrip } from './SurfaceEloStrip'

const series: SeriesRating[] = [
  {
    surface: 'overall',
    matches: 1705,
    current: { elo: 2498.03, as_of: '2005-06-13' },
    peak: { elo: 2920.33, as_of: '1984-11-19' },
  },
  {
    surface: 'grass',
    matches: 358,
    current: { elo: 2336.01, as_of: '2005-06-13' },
    peak: { elo: 2613.88, as_of: '1986-06-23' },
  },
  {
    surface: 'clay',
    matches: 296,
    current: { elo: 2237.91, as_of: '2004-05-24' },
    peak: { elo: 2567.7, as_of: '1985-04-29' },
  },
]

describe('SurfaceEloStrip', () => {
  it('shows the current rating for a player still playing', () => {
    render(<SurfaceEloStrip series={series} mode="current" />)
    expect(screen.getByText('2498')).toBeInTheDocument()
    expect(screen.queryByText(/peak/)).not.toBeInTheDocument()
  })

  // A career that ended below its best still has a best, and it is the number
  // that describes the player.
  it('shows the peak, labelled as such, for a career that is over', () => {
    render(<SurfaceEloStrip series={series} mode="peak" />)
    expect(screen.getByText('2920')).toBeInTheDocument()
    expect(screen.getAllByText(/, peak/).length).toBe(3)
  })

  it('renders only the surfaces the player has a rating in', () => {
    render(<SurfaceEloStrip series={series} mode="current" />)
    expect(screen.queryByText('Hard')).not.toBeInTheDocument()
    expect(screen.queryByText('1500')).not.toBeInTheDocument()
  })

  it('draws nothing at all when the player was never rated', () => {
    const { container } = render(<SurfaceEloStrip series={[]} mode="current" />)
    expect(container).toBeEmptyDOMElement()
  })
})
