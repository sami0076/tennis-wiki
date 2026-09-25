import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ThemeToggle } from './ThemeToggle'

function systemPrefers(dark: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: dark && query.includes('dark'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
  systemPrefers(false)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ThemeToggle', () => {
  it('starts on the system theme, with no attribute to override the media query', async () => {
    render(<ThemeToggle />)
    // No data-theme is how "follow the system" is expressed: the stylesheet's
    // own prefers-color-scheme rule is left to answer.
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
    expect(await screen.findByRole('button', { name: /match my system/i })).toBeInTheDocument()
  })

  it('cycles system, light, dark and back', async () => {
    const user = userEvent.setup()
    render(<ThemeToggle />)
    const button = screen.getByRole('button')

    await user.click(button)
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')

    await user.click(button)
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')

    await user.click(button)
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
  })

  it('remembers an explicit choice and forgets the system one', async () => {
    const user = userEvent.setup()
    render(<ThemeToggle />)
    const button = screen.getByRole('button')

    await user.click(button)
    expect(window.localStorage.getItem('deucepoint:theme')).toBe('light')

    // Back to following the system is an absence, not a stored "system": a key
    // that says system would outlive a later change of mind about the default.
    await user.click(button)
    await user.click(button)
    expect(window.localStorage.getItem('deucepoint:theme')).toBeNull()
  })

  it('names the resolved theme while following the system', async () => {
    systemPrefers(true)
    render(<ThemeToggle />)
    // "System" alone does not tell a reader what they are looking at.
    expect(screen.getByRole('button', { name: /match my system \(dark\)/i })).toBeInTheDocument()
  })

  it('says what pressing it will do', async () => {
    render(<ThemeToggle />)
    expect(screen.getByRole('button', { name: /switch to light/i })).toBeInTheDocument()
  })
})
