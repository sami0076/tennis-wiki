import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CommandPalette } from './CommandPalette'

const federer = {
  slug: 'roger-federer',
  name: 'Roger Federer',
  tour: 'atp',
  country: 'SUI',
  matches: 1526,
  best_tier: 'tour',
  score: 0.9,
}

const nadal = {
  slug: 'rafael-nadal',
  name: 'Rafael Nadal',
  tour: 'atp',
  country: 'ESP',
  matches: 1300,
  best_tier: 'tour',
  score: 0.9,
}

/** Answers the search endpoint from the query, so a pairing step can pick a
 *  different player from the one the first step found. */
function stubSearch() {
  vi.stubGlobal('fetch', (input: RequestInfo | URL) => {
    const url = String(input)
    const q = new URL(url, 'http://localhost').searchParams.get('q') ?? ''
    const matches = [federer, nadal].filter((p) => p.name.toLowerCase().includes(q.toLowerCase()))
    return Promise.resolve(
      new Response(JSON.stringify({ data: matches }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
}

/** Renders the palette open, with the current path on screen to assert on. */
function show() {
  function Here() {
    const { pathname, search } = useLocation()
    return <span data-testid="here">{pathname + search}</span>
  }
  return render(
    <MemoryRouter initialEntries={['/']}>
      <CommandPalette open onClose={() => {}} />
      <Routes>
        <Route path="*" element={<Here />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  window.localStorage.clear()
  stubSearch()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('CommandPalette', () => {
  it('offers the pages with nothing typed', async () => {
    show()
    expect(await screen.findByRole('option', { name: /Rankings/ })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /Leaderboards/ })).toBeInTheDocument()
  })

  it('lists the recently read players first, before anything is typed', async () => {
    window.localStorage.setItem(
      'deucepoint:recent-players',
      JSON.stringify([{ slug: 'stan-wawrinka', name: 'Stan Wawrinka', tour: 'atp' }]),
    )
    show()
    const options = await screen.findAllByRole('option')
    expect(options[0]).toHaveTextContent('Stan Wawrinka')
  })

  it('searches players and opens the one chosen', async () => {
    const user = userEvent.setup()
    show()
    await user.type(screen.getByRole('textbox'), 'federer')

    const option = await screen.findByRole('option', { name: /Roger Federer/ })
    expect(option).toBeInTheDocument()
    await user.click(option)

    await waitFor(() => expect(screen.getByTestId('here')).toHaveTextContent('/players/roger-federer'))
  })

  it('turns a found player into a comparison in two more keystrokes', async () => {
    const user = userEvent.setup()
    show()
    const field = screen.getByRole('textbox')
    await user.type(field, 'federer')
    await screen.findByRole('option', { name: /Roger Federer/ })

    // Tab makes the highlighted player the subject rather than moving focus:
    // the palette is one field and one list, so there is nowhere else to go.
    await user.keyboard('{Tab}')
    await user.click(await screen.findByRole('option', { name: /Head to head against/ }))

    // Now the field searches for the other half of the pair.
    await user.type(field, 'nadal')
    await user.click(await screen.findByRole('option', { name: /Rafael Nadal/ }))

    await waitFor(() =>
      expect(screen.getByTestId('here')).toHaveTextContent('/h2h/roger-federer/rafael-nadal'),
    )
  })

  it('never offers a player as their own opponent', async () => {
    const user = userEvent.setup()
    show()
    const field = screen.getByRole('textbox')
    await user.type(field, 'federer')
    await screen.findByRole('option', { name: /Roger Federer/ })
    await user.keyboard('{Tab}')
    await user.click(await screen.findByRole('option', { name: /Head to head against/ }))

    await user.type(field, 'federer')
    // The API answers a self-comparison with a 400; not offering it is faster
    // and costs no request.
    await waitFor(() => expect(screen.queryByRole('option')).not.toBeInTheDocument())
  })

  it('steps back out of a subject on Escape rather than closing', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <MemoryRouter>
        <CommandPalette open onClose={onClose} />
      </MemoryRouter>,
    )
    await user.type(screen.getByRole('textbox'), 'federer')
    await screen.findByRole('option', { name: /Roger Federer/ })
    await user.keyboard('{Tab}')
    await screen.findByRole('option', { name: /Head to head against/ })

    await user.keyboard('{Escape}')
    expect(onClose).not.toHaveBeenCalled()
    // Back at the top level, where Escape does close it.
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })

  it('says why a one-letter query found nothing', async () => {
    const user = userEvent.setup()
    show()
    await user.type(screen.getByRole('textbox'), 'z')
    expect(await screen.findByText(/two letters or more/i)).toBeInTheDocument()
  })

  it('renders nothing at all when closed', () => {
    render(
      <MemoryRouter>
        <CommandPalette open={false} onClose={() => {}} />
      </MemoryRouter>,
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
