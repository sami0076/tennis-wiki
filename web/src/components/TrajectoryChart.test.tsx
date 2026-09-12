import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { TrajectoryChart, type TrajectoryLineData } from './TrajectoryChart'

function line(name: string, position: number, weeks: number): TrajectoryLineData {
  return {
    name,
    position,
    points: Array.from({ length: weeks }, (_, week) => ({
      date: `2024-0${week + 1}-01`,
      elo: 2000 + week * 10 + position,
    })),
  }
}

describe('TrajectoryChart', () => {
  it('names the leaders at their line ends and counts the rest as the field', () => {
    const lines = [1, 2, 3, 4, 5].map((n) => line(`Player Number${n}`, n, 4))
    render(<TrajectoryChart lines={lines} />)

    // The tag at a line's end is the surname; the whole name is in the description.
    for (const n of [1, 2, 3]) {
      expect(screen.getByText(`Number${n}`)).toBeInTheDocument()
    }
    expect(screen.getByRole('img')).toHaveAttribute(
      'aria-label',
      expect.stringContaining('Player Number1, Player Number2, Player Number3'),
    )
    // The field is a count, not four more names nobody can tell apart.
    expect(screen.getByText(/2 more drawn as the field/)).toBeInTheDocument()
    expect(screen.queryByText('Number4')).not.toBeInTheDocument()
  })

  // A line is what the chart is; one point is a dot, and it would widen the
  // shared axis for something that cannot be drawn on it.
  it('drops a series too short to be a line', () => {
    const { container } = render(
      <TrajectoryChart lines={[line('Long', 1, 4), line('Short', 2, 1)]} />,
    )

    expect(container.querySelectorAll('path')).toHaveLength(1)
    expect(screen.queryByText('Short')).not.toBeInTheDocument()
  })

  it('draws nothing at all rather than an empty frame', () => {
    const { container } = render(<TrajectoryChart lines={[line('Short', 1, 1)]} />)
    expect(container.querySelector('svg')).toBeNull()
  })

  // No axes and no labels, so the whole chart is one described image.
  it('describes itself for anyone who cannot see it', () => {
    render(<TrajectoryChart lines={[line('Iga Swiatek', 1, 4), line('Aryna Sabalenka', 2, 4)]} />)

    const chart = screen.getByRole('img')
    expect(chart).toHaveAttribute('aria-label', expect.stringContaining('Iga Swiatek'))
    expect(chart).toHaveAttribute('aria-label', expect.stringContaining('2024'))
  })

  it('survives a flat series without dividing by zero', () => {
    const flat: TrajectoryLineData = {
      name: 'Flat',
      position: 1,
      points: [
        { date: '2024-01-01', elo: 1500 },
        { date: '2024-01-01', elo: 1500 },
      ],
    }
    const { container } = render(<TrajectoryChart lines={[flat]} />)

    const d = container.querySelector('path')?.getAttribute('d') ?? ''
    expect(d).not.toContain('NaN')
  })
})
