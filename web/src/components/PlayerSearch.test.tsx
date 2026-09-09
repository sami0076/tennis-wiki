import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PlayerSearchResult } from '../api/types.gen'
import { PlayerSearch } from './PlayerSearch'

const djokovic: PlayerSearchResult = {
  slug: 'novak-djokovic',
  name: 'Novak Djokovic',
  tour: 'atp',
  country: 'Serbia',
  matches: 1444,
  best_tier: 'tour',
  score: 0.9,
}

const erler: PlayerSearchResult = {
  slug: 'alexander-erler',
  name: 'Alexander Erler',
  tour: 'atp',
  country: 'Austria',
  matches: 31,
  best_tier: 'challenger',
  score: 0.6,
}

/** Indexing under noUncheckedIndexedAccess, without an assertion on every line. */
function at<T>(items: ArrayLike<T>, index: number): T {
  const item = items[index]
  if (item === undefined) throw new Error(`nothing at index ${index} of ${items.length}`)
  return item
}

function page(data: PlayerSearchResult[]) {
  return { data, next_cursor: null }
}

/** Records every request and hands back control of when each one answers. */
function stubFetch() {
  const calls: { q: string; signal: AbortSignal; settle: (body: unknown) => void }[] = []
  vi.stubGlobal('fetch', (input: string, init: RequestInit) => {
    return new Promise((resolve) => {
      calls.push({
        q: new URL(String(input), 'http://localhost').searchParams.get('q') ?? '',
        signal: init.signal as AbortSignal,
        settle: (body) =>
          resolve(
            new Response(JSON.stringify(body), {
              status: 200,
              headers: { 'Content-Type': 'application/json' },
            }),
          ),
      })
    })
  })
  return calls
}

function Harness({ onSelect = () => {} }: { onSelect?: (p: PlayerSearchResult) => void }) {
  const [value, setValue] = useState('')
  return (
    <PlayerSearch label="Search players" value={value} onChange={setValue} onSelect={onSelect} />
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('PlayerSearch', () => {
  it('asks once for a name typed a letter at a time', async () => {
    const calls = stubFetch()
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'novak')
    await waitFor(() => expect(calls).toHaveLength(1))

    at(calls, 0).settle(page([djokovic]))
    expect(await screen.findByText('Novak Djokovic')).toBeInTheDocument()
    expect(calls).toHaveLength(1)
  })

  // The API answers 400 below two characters, so asking would turn the first
  // keystroke of every search into an error.
  it('does not ask below two characters', async () => {
    const calls = stubFetch()
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'n')
    expect(await screen.findByText(/at least 2 characters/)).toBeInTheDocument()
    expect(calls).toHaveLength(0)
  })

  it('lets a superseded response lose to the newer one', async () => {
    const calls = stubFetch()
    const user = userEvent.setup()
    render(<Harness />)

    const input = screen.getByRole('combobox')
    await user.type(input, 'ale')
    await waitFor(() => expect(calls).toHaveLength(1))

    await user.type(input, 'xander')
    await waitFor(() => expect(calls).toHaveLength(2))

    at(calls, 1).settle(page([erler]))
    expect(await screen.findByText('Alexander Erler')).toBeInTheDocument()

    // The first request lands last, and carries the answer to a query nobody is
    // asking any more.
    expect(at(calls, 0).signal.aborted).toBe(true)
    at(calls, 0).settle(page([djokovic]))
    await waitFor(() => expect(screen.queryByText('Novak Djokovic')).not.toBeInTheDocument())
  })

  it('moves with the arrow keys and picks with Enter', async () => {
    const calls = stubFetch()
    const chosen: PlayerSearchResult[] = []
    const user = userEvent.setup()
    render(<Harness onSelect={(player) => chosen.push(player)} />)

    const input = screen.getByRole('combobox')
    await user.type(input, 'a')
    await user.type(input, 'l')
    await waitFor(() => expect(calls).toHaveLength(1))
    at(calls, 0).settle(page([erler, djokovic]))
    await screen.findByText('Alexander Erler')

    await user.keyboard('{ArrowDown}{ArrowDown}')
    const options = screen.getAllByRole('option')
    expect(at(options, 1)).toHaveAttribute('aria-selected', 'true')
    // Focus stays in the input; the highlight is named rather than moved to.
    expect(input).toHaveFocus()
    expect(input).toHaveAttribute('aria-activedescendant', at(options, 1).id)

    await user.keyboard('{Enter}')
    expect(chosen).toEqual([djokovic])
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })

  it('says nothing matched, and what to try instead', async () => {
    const calls = stubFetch()
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'zzzz')
    await waitFor(() => expect(calls).toHaveLength(1))
    at(calls, 0).settle(page([]))

    const messages = await screen.findAllByText(/No player matches "zzzz"/)
    expect(messages.length).toBeGreaterThan(0)
    expect(at(messages, 0)).toHaveTextContent('surname on its own')
  })

  it('shows enough beside a name to tell two players apart', async () => {
    const calls = stubFetch()
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'alexander')
    await waitFor(() => expect(calls).toHaveLength(1))
    at(calls, 0).settle(page([erler]))

    await screen.findByText('Alexander Erler')
    const option = screen.getByRole('option')
    expect(option).toHaveTextContent('ATP')
    expect(option).toHaveTextContent('Austria')
    expect(option).toHaveTextContent('31 matches')
    expect(option).toHaveTextContent('Challenger')
  })
})
