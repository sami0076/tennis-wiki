import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { checks, headings } from '../generated/methodology'
import { Methodology } from './Methodology'

function renderPage() {
  return render(
    <MemoryRouter>
      <Methodology />
    </MemoryRouter>,
  )
}

describe('Methodology', () => {
  // The page is the document, not a second copy of it. If the generated module
  // were empty this is what would notice.
  it('renders the document it is generated from', () => {
    const { container } = renderPage()

    expect(screen.getByRole('heading', { level: 1, name: 'Methodology' })).toBeInTheDocument()
    expect(container.querySelectorAll('h2').length).toBeGreaterThan(5)
    expect(screen.getByText(/Absence is not zero/)).toBeInTheDocument()
  })

  // Every contents entry has to land somewhere. A table of contents pointing at
  // an id that no longer exists is worse than none.
  it('links its contents to sections that exist', () => {
    const { container } = renderPage()

    const links = [...container.querySelectorAll('nav a')]
    expect(links.length).toBe(headings.filter((h) => h.level > 1).length)
    for (const link of links) {
      const id = link.getAttribute('href')?.slice(1) ?? ''
      expect(container.querySelector(`article #${CSS.escape(id)}`)).not.toBeNull()
    }
  })

  // The point of publishing the methodology is the parts that are not
  // flattering. This is the one the simulator would rather not mention.
  it('carries the results that do not flatter the model', () => {
    renderPage()

    expect(
      screen.getByRole('heading', { name: /chain expects too many deciding sets/ }),
    ).toBeInTheDocument()
    expect(screen.getByText(/Promoted/)).toBeInTheDocument()
    expect(screen.getAllByText(/34\.1%/).length).toBeGreaterThan(0)
  })

  // Not retyped: the figures beside the title come from the committed run, and
  // the generator refuses to build a page whose prose disagrees with it.
  it('names the run its figures come from', () => {
    renderPage()

    // Twice over: once beside the title from the run, once in the prose that
    // the generator checked against it.
    expect(screen.getAllByText(new RegExp(checks.tourAccuracy.toFixed(1))).length).toBe(2)
    // The document points at the run too, so this link is on the page twice.
    for (const link of screen.getAllByRole('link', { name: 'validation.json' })) {
      expect(link).toHaveAttribute('href', expect.stringContaining('docs/validation.json'))
    }
  })

  // MathML, rendered by the browser. No font files, no runtime library. The TeX
  // survives inside the annotation element, which is where a screen reader and
  // a copy-paste both look for it.
  it('renders the formulas as maths rather than as source', () => {
    const { container } = renderPage()

    expect(container.querySelectorAll('math').length).toBeGreaterThan(1)
    const annotations = [...container.querySelectorAll('math annotation')].map(
      (node) => node.textContent ?? '',
    )
    expect(annotations.some((tex) => tex.includes('frac{250}'))).toBe(true)
    expect(container.querySelector('article')?.textContent).not.toContain('$$')
  })

  // The document links to ADRs and configuration by relative path, which only
  // resolves in the repository.
  it('points its repository links at the repository', () => {
    renderPage()

    const [adr] = screen.getAllByRole('link', { name: 'ADR-0004' })
    expect(adr).toHaveAttribute(
      'href',
      'https://github.com/sami0076/tennis-wiki/blob/main/docs/decisions/0004-tier-taxonomy-and-elo-pool.md',
    )
  })
})
