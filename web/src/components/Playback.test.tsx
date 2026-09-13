import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Playback } from './Playback'

const chain = {
  point: [0.621, 0.606] as [number, number],
  hold: [0.777, 0.748] as [number, number],
  set: [0.552, 0.448] as [number, number],
  match: [0.596, 0.404] as [number, number],
}
const players = [{ name: 'Carlos Alcaraz' }, { name: 'Jannik Sinner' }] as const

function motion(reduced: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: reduced,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

function show() {
  return render(<Playback chain={chain} bestOf={3} players={players} surface="clay" />)
}

/** Lines name a side in its own span, so a line is matched on its paragraph. */
function lines(pattern: RegExp): HTMLElement[] {
  return screen.queryAllByText(
    (_, element) => element?.tagName === 'P' && pattern.test(element.textContent ?? ''),
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('Playback', () => {
  it('waits to be asked', () => {
    motion(false)
    show()
    expect(screen.getByRole('button', { name: 'Watch a simulated match' })).toBeEnabled()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  // Anyone who asked for less motion gets the finished match at once.
  it('renders the finished match at once under reduced motion', async () => {
    motion(true)
    const user = userEvent.setup()
    show()
    await user.click(screen.getByRole('button', { name: 'Watch a simulated match' }))

    // The line under the board, and the same line announced to a screen reader.
    expect(lines(/^Game, set, match: (Alcaraz|Sinner) wins 2-[01]$/)).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Watch another' })).toBeEnabled()
    expect(screen.getByText('Service holds')).toBeInTheDocument()
  })

  it('plays beat by beat and ends on the result', async () => {
    motion(false)
    vi.useFakeTimers()
    show()
    fireEvent.click(screen.getByRole('button', { name: 'Watch a simulated match' }))

    expect(screen.getByRole('button', { name: 'Playing…' })).toBeDisabled()
    expect(screen.getByRole('table')).toBeInTheDocument()

    // The first beat lands after the fade.
    await act(async () => {
      vi.advanceTimersByTime(250)
    })
    expect(lines(/^(Alcaraz|Sinner) (holds|breaks)|^Break point, /)).toHaveLength(1)

    // Each beat schedules the next once React has committed it, so the match
    // is played through a beat at a time rather than in one jump.
    for (let beat = 0; beat < 400 && lines(/^Game, set, match: /).length === 0; beat++) {
      await act(async () => {
        vi.advanceTimersByTime(1500)
      })
    }
    expect(lines(/^Game, set, match: /)).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Watch another' })).toBeEnabled()
  }, 20000)

  it('leaves nothing ticking when it is taken off the page mid-match', async () => {
    motion(false)
    vi.useFakeTimers()
    const { unmount } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Watch a simulated match' }))
    await act(async () => {
      vi.advanceTimersByTime(1000)
    })
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
