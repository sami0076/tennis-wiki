import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SplitBar } from './SplitBar'

function fills(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('[style*="width"]')).map(
    (el) => (el as HTMLElement).style.width,
  )
}

describe('SplitBar', () => {
  // Centre-out, not a stacked bar split by share: 43 against 45 has to read as
  // two short bars, not as a near-even split of something large.
  it('scales each half by its own value, not by its share of the pair', () => {
    const { container } = render(<SplitBar label="Return points won" left={43} right={45} />)
    const [left, right] = fills(container)
    expect(left).toBe(`${(43 / 45) * 100}%`)
    expect(right).toBe('100%')
  })

  it('measures a group against a shared max when given one', () => {
    const { container } = render(<SplitBar label="Clay Elo" left={2214} right={2189} max={2500} />)
    const [left, right] = fills(container)
    expect(left).toBe(`${(2214 / 2500) * 100}%`)
    expect(right).toBe(`${(2189 / 2500) * 100}%`)
  })

  it('survives a pair of zeroes without dividing by zero', () => {
    const { container } = render(<SplitBar label="Meetings" left={0} right={0} />)
    expect(fills(container)).toEqual(['0%', '0%'])
    expect(screen.getByRole('img', { name: 'Meetings: 0 to 0' })).toBeInTheDocument()
  })
})
