import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { StatTable, type Column } from './StatTable'

interface Row {
  name: string
  aces: number | null
}

const rows: ReadonlyArray<Row> = [
  { name: 'Recent', aces: 12 },
  { name: 'Older', aces: 3 },
  { name: 'Unrecorded', aces: null },
]

const columns: ReadonlyArray<Column<Row>> = [
  { key: 'name', header: 'Match', value: (row) => row.name },
  { key: 'aces', header: 'Aces', align: 'right', value: (row) => row.aces },
]

function bodyOrder(): string[] {
  const table = screen.getByRole('table')
  return within(table)
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[0]?.textContent ?? '')
}

describe('StatTable', () => {
  it('renders an absent value as n/r rather than a zero', () => {
    render(<StatTable caption="Aces" columns={columns} rows={rows} rowKey={(r) => r.name} />)
    expect(screen.getByLabelText('Aces: not recorded')).toBeInTheDocument()
  })

  // Sorting an absent value as zero would collect every unrecorded match at one
  // end and read as the worst performances in the table.
  it('sorts absent values last in both directions', async () => {
    const user = userEvent.setup()
    render(<StatTable caption="Aces" columns={columns} rows={rows} rowKey={(r) => r.name} />)

    const header = screen.getByRole('button', { name: /Aces/ })

    await user.click(header)
    expect(bodyOrder()).toEqual(['Recent', 'Older', 'Unrecorded'])

    await user.click(header)
    expect(bodyOrder()).toEqual(['Older', 'Recent', 'Unrecorded'])
  })

  // The column is in the markup either way, so it stays sortable and reaches a
  // screen reader; only the 360px layout drops it, and that is a CSS decision.
  it('marks a wide-only column rather than omitting it', () => {
    const wide: ReadonlyArray<Column<Row>> = [
      ...columns,
      { key: 'peak', header: 'Peak', align: 'right', wide: true, value: (row) => row.aces },
    ]
    const { container } = render(
      <StatTable caption="Aces" columns={wide} rows={rows} rowKey={(r) => r.name} />,
    )

    const headers = [...container.querySelectorAll('th')]
    expect(headers.map((th) => th.textContent)).toEqual(['Match', 'Aces', 'Peak'])
    expect(headers[2]?.className).toContain('wide')
    expect(headers[0]?.className).not.toContain('wide')
    // Every row hides the same column, or the header and its cells drift apart.
    expect(container.querySelectorAll('td[class*="wide"]')).toHaveLength(rows.length)
  })

  it('reports the sorted column to assistive technology', async () => {
    const user = userEvent.setup()
    render(<StatTable caption="Aces" columns={columns} rows={rows} rowKey={(r) => r.name} />)

    await user.click(screen.getByRole('button', { name: /Aces/ }))
    expect(screen.getByRole('columnheader', { name: /Aces/ })).toHaveAttribute(
      'aria-sort',
      'descending',
    )
  })
})
