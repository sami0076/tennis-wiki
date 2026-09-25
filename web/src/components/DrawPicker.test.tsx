import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { DrawPicker } from './DrawPicker'
import type { ReplayableDraw } from '../api/types.gen'

function draw(over: Partial<ReplayableDraw> = {}): ReplayableDraw {
  return {
    slug: 'wimbledon-atp',
    name: 'Wimbledon',
    season: 2019,
    tour: 'atp',
    tier: 'tour',
    level: 'G',
    surface: 'grass',
    draw_size: 128,
    matches: 127,
    start_date: '2019-07-01',
    ...over,
  }
}

const wimbledon = draw()
const roland = draw({ slug: 'roland-garros-atp', name: 'Roland Garros', surface: 'clay' })
const older = draw({ slug: 'wimbledon-atp', name: 'Wimbledon', season: 2015 })

describe('DrawPicker', () => {
  it('describes a draw by what a reader is choosing between', () => {
    render(<DrawPicker draws={[wimbledon]} value={{ event: 'wimbledon-atp', season: 2019 }} onChange={() => {}} />)
    expect(screen.getByRole('option', { name: 'Wimbledon · ATP · Grass · 128 draw' })).toBeInTheDocument()
  })

  it('groups the draws by season', () => {
    render(
      <DrawPicker draws={[wimbledon, older]} value={{ event: 'wimbledon-atp', season: 2019 }} onChange={() => {}} />,
    )
    const groups = screen.getAllByRole('group')
    expect(groups.map((g) => g.getAttribute('label'))).toEqual(['2019', '2015'])
  })

  it('hands back both halves of the address', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <DrawPicker
        draws={[wimbledon, roland]}
        value={{ event: 'wimbledon-atp', season: 2019 }}
        onChange={onChange}
      />,
    )
    await user.selectOptions(screen.getByRole('combobox'), 'roland-garros-atp\u00002019')
    // A slug alone names an event, not an edition of it.
    expect(onChange).toHaveBeenCalledWith({ event: 'roland-garros-atp', season: 2019 })
  })

  it('tells two seasons of one event apart', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <DrawPicker draws={[wimbledon, older]} value={{ event: 'wimbledon-atp', season: 2019 }} onChange={onChange} />,
    )
    await user.selectOptions(screen.getByRole('combobox'), 'wimbledon-atp\u00002015')
    expect(onChange).toHaveBeenCalledWith({ event: 'wimbledon-atp', season: 2015 })
  })

  it('keeps showing the draw on screen even when the list does not carry it', () => {
    // The featured draw is addressed directly and need not clear the
    // eligibility cut this list is built from. A blank field under a rendered
    // draw would be a lie about what is on screen.
    render(<DrawPicker draws={[roland]} value={{ event: 'some-other-atp', season: 2011 }} onChange={() => {}} />)
    const select = screen.getByRole('combobox') as HTMLSelectElement
    expect(select.value).toBe('some-other-atp\u00002011')
    expect(within(select).getByRole('option', { name: 'Showing this draw' })).toBeInTheDocument()
  })

  it('describes a draw with no recorded size by its matches', () => {
    render(
      <DrawPicker
        draws={[draw({ draw_size: null, matches: 31 })]}
        value={{ event: 'wimbledon-atp', season: 2019 }}
        onChange={() => {}}
      />,
    )
    expect(screen.getByRole('option', { name: /31 matches/ })).toBeInTheDocument()
  })

  it('leaves an unrecorded surface out rather than writing "unknown"', () => {
    render(
      <DrawPicker
        draws={[draw({ surface: 'unknown' })]}
        value={{ event: 'wimbledon-atp', season: 2019 }}
        onChange={() => {}}
      />,
    )
    expect(screen.getByRole('option', { name: 'Wimbledon · ATP · 128 draw' })).toBeInTheDocument()
  })

  it('does not offer an empty list to choose from', () => {
    render(<DrawPicker draws={[]} value={{ event: 'wimbledon-atp', season: 2019 }} onChange={() => {}} busy />)
    expect(screen.getByRole('combobox')).toBeDisabled()
  })
})
