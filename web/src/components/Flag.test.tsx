import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Flag } from './Flag'
import { flagCode, isHistorical } from '../lib/country'

describe('flagCode', () => {
  // These are the whole reason the map exists. IOC and ISO agree often enough
  // that a wrong assumption survives casual testing and then flies Germany's
  // flag for Georgia.
  it.each([
    ['GER', 'de'],
    ['SUI', 'ch'],
    ['NED', 'nl'],
    ['DEN', 'dk'],
    ['CRO', 'hr'],
    ['GRE', 'gr'],
    ['POR', 'pt'],
    ['RSA', 'za'],
    ['TPE', 'tw'],
    ['KSA', 'sa'],
    ['IRI', 'ir'],
    ['INA', 'id'],
  ])('maps the IOC code %s to %s, which ISO spells differently', (ioc, iso) => {
    expect(flagCode(ioc)).toBe(iso)
  })

  it('maps the codes where the two standards happen to agree', () => {
    expect(flagCode('USA')).toBe('us')
    expect(flagCode('FRA')).toBe('fr')
    expect(flagCode('ESP')).toBe('es')
  })

  it('gives a country that no longer exists no flag at all', () => {
    // A Soviet player did not play for Russia and a Yugoslav one did not play
    // for Serbia. Flying a successor's flag would invent a fact.
    for (const code of ['URS', 'YUG', 'TCH', 'SCG', 'FRG', 'GDR', 'RHO']) {
      expect(flagCode(code)).toBeNull()
      expect(isHistorical(code)).toBe(true)
    }
  })

  it('gives an unknown or empty code no flag rather than a guess', () => {
    expect(flagCode('ZZZ')).toBeNull()
    expect(flagCode('')).toBeNull()
    expect(flagCode(null)).toBeNull()
    expect(flagCode(undefined)).toBeNull()
  })

  it('does not care about the case it is handed', () => {
    expect(flagCode('sui')).toBe('ch')
    expect(flagCode(' Sui ')).toBe('ch')
  })
})

describe('Flag', () => {
  it('points at the file named by the ISO code, not the IOC one', () => {
    const { container } = render(<Flag country="SUI" />)
    expect(container.querySelector('img')).toHaveAttribute('src', '/flags/ch.svg')
  })

  it('keeps the code beside the picture, so colour is never the only encoding', () => {
    render(<Flag country="SUI" />)
    expect(screen.getByText('SUI')).toBeInTheDocument()
  })

  it('is decoration, not content, when it carries no code', () => {
    // These sit against a name that already identifies the player. Announcing
    // the country too would bury the name inside its own link.
    const { container } = render(<Flag country="SUI" code={false} />)
    expect(screen.queryByText('SUI')).not.toBeInTheDocument()
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true')
    expect(container.querySelector('img')).toHaveAttribute('src', '/flags/ch.svg')
  })

  it('keeps the letters and drops the picture for a country with no flag', () => {
    const { container } = render(<Flag country="URS" />)
    expect(screen.getByText('URS')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
  })

  it('renders nothing for a player with no country recorded', () => {
    const { container } = render(<Flag country={null} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('prints the code as the API gave it', () => {
    // The database stores upper-case IOC codes; this should not be re-casing
    // anything, because anything that is not one is not a code to normalise.
    render(<Flag country="Austria" />)
    expect(screen.getByText('Austria')).toBeInTheDocument()
  })
})
