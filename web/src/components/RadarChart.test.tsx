import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { RadarChart, type RadarSeries } from './RadarChart'

const series: RadarSeries[] = [
  { name: 'Ann Able', tone: 'a', values: [90, 40, 70], details: ['72.1%', null, '2100 Elo'] },
  { name: 'Bea Baker', tone: 'b', values: [20, null, 55], details: ['60.4%', null, '1950 Elo'] },
]

function renderChart() {
  return render(<RadarChart label="Last year" axes={['Serve', 'Return', 'Clay']} series={series} />)
}

describe('RadarChart', () => {
  it('names both players in the key', () => {
    renderChart()
    expect(screen.getAllByText('Ann Able').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Bea Baker').length).toBeGreaterThan(0)
  })

  it('carries every figure in a table, a missing one said as missing', () => {
    renderChart()
    const table = screen.getByRole('table', { name: 'Last year' })
    const serve = within(table).getByRole('row', { name: /Serve/ })
    expect(serve).toHaveTextContent('90th · 72.1%')
    expect(serve).toHaveTextContent('20th · 60.4%')
    expect(within(table).getByRole('row', { name: /Return/ })).toHaveTextContent('no figure')
  })

  it('reads an axis from the keyboard', () => {
    renderChart()
    const plot = screen.getByRole('figure')
    fireEvent.keyDown(plot, { key: 'ArrowRight' })
    expect(screen.getByText(/^Serve: Ann Able 90th/)).toBeInTheDocument()
    fireEvent.keyDown(plot, { key: 'ArrowLeft' })
    expect(screen.getByText(/^Clay: Ann Able 70th · 2100 Elo, Bea Baker 55th/)).toBeInTheDocument()
  })

  it('does not draw a missing figure as a vertex', () => {
    const { container } = renderChart()
    expect(container.querySelectorAll('circle')).toHaveLength(5)
  })
})
