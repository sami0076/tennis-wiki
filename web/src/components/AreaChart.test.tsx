import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { AreaChart } from './AreaChart'

const points = [
  { date: '2019-01-07', elo: 1800 },
  { date: '2020-01-06', elo: 1950 },
  { date: '2021-01-04', elo: 1875 },
]

describe('AreaChart', () => {
  it('draws nothing from a single point, which is not a line', () => {
    const { container } = render(<AreaChart points={points.slice(0, 1)} label="One week" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('reads out a week when the plot takes focus, so the line is not mouse-only', async () => {
    const user = userEvent.setup()
    render(<AreaChart points={points} label="Elo over three seasons" />)

    const plot = screen.getByRole('figure')
    await user.tab()
    expect(plot).toHaveFocus()
    // Focus lands on the latest week, which is the one the end marker labels.
    expect(screen.getByText('2021-01-04')).toBeInTheDocument()
  })

  it('steps through the weeks with the arrow keys', async () => {
    const user = userEvent.setup()
    render(<AreaChart points={points} label="Elo over three seasons" />)
    await user.tab()

    await user.keyboard('{ArrowLeft}')
    expect(screen.getByText('2020-01-06')).toBeInTheDocument()
    expect(screen.getByText('Elo 1950')).toBeInTheDocument()

    await user.keyboard('{Home}')
    expect(screen.getByText('2019-01-07')).toBeInTheDocument()
    expect(screen.getByText('Elo 1800')).toBeInTheDocument()
  })

  it('announces the reading as well as drawing it', async () => {
    const user = userEvent.setup()
    const { container } = render(<AreaChart points={points} label="Elo over three seasons" />)
    await user.tab()
    await user.keyboard('{Home}')
    // The drawn readout is hidden from assistive technology by being a
    // picture; this is the same fact in words.
    const live = container.querySelector('[aria-live="polite"]')
    expect(live).toHaveTextContent('2019-01-07: Elo 1800')
  })

  it('keeps the peak marker out of the way while a week is being read', async () => {
    const user = userEvent.setup()
    render(<AreaChart points={points} label="Elo over three seasons" />)
    expect(screen.getByText('Peak 1950')).toBeInTheDocument()

    await user.tab()
    // Two labels over the same line would overlap; the one being asked for wins.
    expect(screen.queryByText('Peak 1950')).not.toBeInTheDocument()
  })
})
