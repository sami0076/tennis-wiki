import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Meta } from './Meta'

describe('Meta', () => {
  it('joins the parts with a middle dot', () => {
    render(<Meta parts={['Spain', 'right-handed', 23, 'turned pro 2018']} />)
    const line = screen.getByText(/Spain/)
    expect(line.textContent).toBe('Spain \u00b7 right-handed \u00b7 23 \u00b7 turned pro 2018')
  })

  // A player with no recorded hand must not leave a dot with nothing after it.
  it('drops absent parts rather than stranding a separator', () => {
    render(<Meta parts={['Sweden', null, undefined, '', '1973-1983']} />)
    expect(screen.getByText(/Sweden/).textContent).toBe('Sweden \u00b7 1973-1983')
  })

  it('renders nothing at all when every part is absent', () => {
    const { container } = render(<Meta parts={[null, '']} />)
    expect(container).toBeEmptyDOMElement()
  })
})
