import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { useUrlParam, useUrlParams } from './useUrlParam'

function Here() {
  const { search } = useLocation()
  return <span data-testid="search">{search}</span>
}

function mount(ui: React.ReactNode, at = '/x') {
  return render(
    <MemoryRouter initialEntries={[at]}>
      {ui}
      <Routes>
        <Route path="*" element={<Here />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('useUrlParam', () => {
  it('reads a parameter out of the URL', () => {
    function Reader() {
      const [value] = useUrlParam('surface')
      return <span data-testid="value">{value ?? 'none'}</span>
    }
    mount(<Reader />, '/x?surface=clay')
    expect(screen.getByTestId('value')).toHaveTextContent('clay')
  })

  it('removes the parameter rather than writing an empty one', async () => {
    const user = userEvent.setup()
    function Clearer() {
      const [, set] = useUrlParam('surface')
      return <button onClick={() => set(null)}>clear</button>
    }
    mount(<Clearer />, '/x?surface=clay&tour=atp')
    await user.click(screen.getByRole('button'))
    expect(screen.getByTestId('search')).toHaveTextContent('?tour=atp')
  })
})

describe('useUrlParams', () => {
  // Two useUrlParam setters in one tick each resolve against the URL as it was
  // before either ran, so the last navigation wins and the first write is
  // lost. This is the case that found it: a draw is a slug AND a season, and
  // writing them separately left the simulator asking for a season with no
  // event at all.
  it('writes several parameters in one navigation', async () => {
    const user = userEvent.setup()
    function Chooser() {
      const set = useUrlParams()
      return (
        <button onClick={() => set({ event: 'roland-garros-atp', season: '2019' })}>choose</button>
      )
    }
    mount(<Chooser />)
    await user.click(screen.getByRole('button'))

    const search = screen.getByTestId('search')
    expect(search).toHaveTextContent('event=roland-garros-atp')
    expect(search).toHaveTextContent('season=2019')
  })

  it('is what one setter at a time is not', async () => {
    const user = userEvent.setup()
    function Broken() {
      const [, setEvent] = useUrlParam('event')
      const [, setSeason] = useUrlParam('season')
      return (
        <button
          onClick={() => {
            setEvent('roland-garros-atp')
            setSeason('2019')
          }}
        >
          choose
        </button>
      )
    }
    mount(<Broken />)
    await user.click(screen.getByRole('button'))

    // Pinning the behaviour that makes useUrlParams necessary. If a future
    // react-router makes sequential setters compose, this fails and the
    // batching helper can go.
    expect(screen.getByTestId('search')).not.toHaveTextContent('event=roland-garros-atp')
  })

  it('keeps the parameters it was not asked about', async () => {
    const user = userEvent.setup()
    function Chooser() {
      const set = useUrlParams()
      return <button onClick={() => set({ season: '2021' })}>choose</button>
    }
    mount(<Chooser />, '/x?tour=wta&season=2019')
    await user.click(screen.getByRole('button'))

    const search = screen.getByTestId('search')
    expect(search).toHaveTextContent('tour=wta')
    expect(search).toHaveTextContent('season=2021')
  })

  it('removes a parameter set to null', async () => {
    const user = userEvent.setup()
    function Chooser() {
      const set = useUrlParams()
      return <button onClick={() => set({ event: null, season: '2021' })}>choose</button>
    }
    mount(<Chooser />, '/x?event=wimbledon-atp&season=2019')
    await user.click(screen.getByRole('button'))
    expect(screen.getByTestId('search')).toHaveTextContent('?season=2021')
  })
})
