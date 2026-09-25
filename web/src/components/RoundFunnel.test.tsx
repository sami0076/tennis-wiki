import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RoundFunnel } from './RoundFunnel'

const rounds = [
  { round: 'R32', matches: 40, wins: 34 },
  { round: 'QF', matches: 20, wins: 12 },
  { round: 'F', matches: 5, wins: 3 },
]

describe('RoundFunnel', () => {
  it('writes the record beside every bar, so colour is never the only encoding', () => {
    render(<RoundFunnel rounds={rounds} />)
    // A non-breaking hyphen keeps a record from wrapping mid-figure, so the
    // assertion has to look for that character rather than a plain dash.
    expect(screen.getByText('34‑6')).toBeInTheDocument()
    expect(screen.getByText('12‑8')).toBeInTheDocument()
    expect(screen.getByText('3‑2')).toBeInTheDocument()
  })

  it("spells the round out and keeps the source's code beside it", () => {
    render(<RoundFunnel rounds={rounds} />)
    expect(screen.getByText('Third round')).toBeInTheDocument()
    expect(screen.getByText('R32')).toBeInTheDocument()
    expect(screen.getByText('Quarter-final')).toBeInTheDocument()
  })

  it('renders nothing rather than an empty frame for a career with no rounds', () => {
    const { container } = render(<RoundFunnel rounds={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when every round has no matches', () => {
    // A funnel scaled against a busiest round of zero would divide by zero.
    const { container } = render(<RoundFunnel rounds={[{ round: 'F', matches: 0, wins: 0 }]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('names an unknown round code rather than dropping it', () => {
    render(<RoundFunnel rounds={[{ round: 'ER', matches: 3, wins: 2 }]} />)
    expect(screen.getAllByText('ER').length).toBeGreaterThan(0)
  })
})
