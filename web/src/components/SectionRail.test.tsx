import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SectionRail } from './SectionRail'

const items = [
  { id: 'rating', label: 'Rating' },
  { id: 'serve', label: 'Serve and return' },
]

describe('SectionRail', () => {
  it('makes every section a link somebody can send', () => {
    render(<SectionRail items={items} />)
    expect(screen.getByRole('link', { name: 'Rating' })).toHaveAttribute('href', '#rating')
    expect(screen.getByRole('link', { name: 'Serve and return' })).toHaveAttribute('href', '#serve')
  })

  it('is a named navigation, so it is not a second unlabelled nav on the page', () => {
    render(<SectionRail items={items} label="This career" />)
    expect(screen.getByRole('navigation', { name: 'This career' })).toBeInTheDocument()
  })

  it('marks the first section without an observer to tell it otherwise', () => {
    // jsdom has no IntersectionObserver. The rail is a table of contents
    // first: every link still works and the first entry stays marked.
    render(<SectionRail items={items} />)
    expect(screen.getByRole('link', { name: 'Rating' })).toHaveAttribute('aria-current', 'true')
    expect(screen.getByRole('link', { name: 'Serve and return' })).not.toHaveAttribute(
      'aria-current',
    )
  })
})
