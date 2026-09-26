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

  // The chart used to carry two figures and no time axis at all, so a reader
  // could see a line rise without being able to say from what, to what, or when.
  it('numbers both axes', () => {
    render(<TrajectoryChart lines={[line('A Player', 1, 4), line('B Player', 2, 4)]} />)

    // The ratings run 2001..2032 here, so the grid lands on 2010, 2020, 2030.
    expect(screen.getByText('2010')).toBeInTheDocument()
    expect(screen.getByText('2020')).toBeInTheDocument()
    expect(screen.getByText('Elo')).toBeInTheDocument()
    // And the time axis says when, which nothing did before: the series runs
    // January to April 2024, so the months are named and the year is carried.
    expect(screen.getByText('Feb')).toBeInTheDocument()
    expect(screen.getByText('Mar')).toBeInTheDocument()
    expect(screen.getByText('2024')).toBeInTheDocument()
  })

  // Joining two points either side of a long absence draws a climb that never
  // happened. The line is cut instead, and the cut is declared.
  it('breaks a line where the player went unrated, rather than bridging it', () => {
    const gapped: TrajectoryLineData = {
      name: 'Gone Away',
      position: 1,
      points: [
        { date: '2019-01-07', elo: 2400 },
        { date: '2019-03-26', elo: 2420 },
        { date: '2025-12-29', elo: 2500 },
      ],
    }
    const { container } = render(<TrajectoryChart lines={[gapped, line('Present', 2, 4)]} />)

    const d = container.querySelector('path')?.getAttribute('d') ?? ''
    // Two subpaths: one per stretch the player was actually rated through.
    expect((d.match(/M/g) ?? []).length).toBeGreaterThan(1)
    expect(screen.getByRole('img')).toHaveAttribute(
      'aria-label',
      expect.stringContaining('break'),
    )
    expect(screen.getByText(/six months or more/)).toBeInTheDocument()
  })

  it('keeps an uninterrupted line in one piece', () => {
    const { container } = render(
      <TrajectoryChart lines={[line('A Player', 1, 4), line('B Player', 2, 4)]} />,
    )
    for (const path of container.querySelectorAll('path')) {
      expect((path.getAttribute('d')?.match(/M/g) ?? []).length).toBe(1)
    }
    expect(screen.queryByText(/six months or more/)).not.toBeInTheDocument()
  })

  // A path of one point draws nothing, so a week with an absence either side
  // of it would silently disappear.
  it('marks a week stranded between two absences', () => {
    const stranded: TrajectoryLineData = {
      name: 'One Week',
      position: 1,
      points: [
        { date: '2019-01-07', elo: 2400 },
        { date: '2022-06-01', elo: 2450 },
        { date: '2025-12-29', elo: 2500 },
      ],
    }
    const { container } = render(<TrajectoryChart lines={[stranded, line('Present', 2, 4)]} />)
    // A zero-length path with a round cap, so the dot stays round under the
    // stretch that turns a <circle> into an ellipse.
    expect(container.querySelectorAll('path[d$="l0 0"]').length).toBe(3)
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
