import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  AvailabilityNeverForTier,
  AvailabilityRecorded,
  type HeadToHead as Comparison,
  type Meeting,
  type ServeStats,
} from '../api/client'
import { HeadToHead } from './HeadToHead'

const borg = { slug: 'bjorn-borg', name: 'Bjorn Borg', tour: 'atp', country: 'Sweden' }
const mcenroe = {
  slug: 'john-mcenroe',
  name: 'John McEnroe',
  tour: 'atp',
  country: 'United States',
}

function serve(matches: number): ServeStats {
  return {
    availability: AvailabilityRecorded,
    matches_with_data: matches,
    rates: {
      aces_per_match: 8.4,
      double_faults_per_match: 2.1,
      first_serve_in_percentage: 61,
      first_serve_won_percentage: 74,
      second_serve_won_percentage: 52,
      break_points_saved_percentage: 66,
    },
  }
}

const meetings: Meeting[] = [
  {
    date: '1980-07-05',
    tournament: 'Wimbledon',
    event_slug: 'wimbledon-atp',
    tier: 'tour',
    level: 'G',
    season: 1980,
    round: 'F',
    qualifying: false,
    surface: 'grass',
    winner_index: 0,
    score: '1-6 7-5 6-3 6-7 8-6',
    incomplete: false,
    charting_id: null,
    best_of: 5,
    deciding_set: false,
    tiebreaks: [0, 0],
  },
  {
    date: '1981-01-18',
    tournament: 'Masters',
    event_slug: null,
    tier: 'tour',
    level: 'M',
    season: 1981,
    round: 'F',
    qualifying: false,
    surface: 'carpet',
    winner_index: 1,
    score: '6-4 6-2 6-4',
    incomplete: false,
    charting_id: null,
    best_of: 5,
    deciding_set: false,
    tiebreaks: [0, 0],
  },
  {
    date: '1981-09-09',
    tournament: 'US Open',
    event_slug: 'us-open-atp',
    tier: 'tour',
    level: 'G',
    season: 1981,
    round: 'F',
    qualifying: false,
    surface: 'hard',
    winner_index: 1,
    score: '4-6 6-2 6-4 6-3',
    incomplete: false,
    charting_id: null,
    best_of: 5,
    deciding_set: false,
    tiebreaks: [0, 0],
  },
]

const rivalry: Comparison = {
  players: [borg, mcenroe],
  record: { matches: 3, wins: [1, 2], incomplete: 0 },
  surfaces: [
    { name: 'grass', matches: 1, wins: [1, 0] },
    { name: 'carpet', matches: 1, wins: [0, 1] },
    { name: 'hard', matches: 1, wins: [0, 1] },
  ],
  tiers: [{ name: 'tour', matches: 3, wins: [1, 2] }],
  serve: [serve(3), serve(3)],
  meetings,
  filters: { level: null, round: null, best_of: null, surface: null, deciders: false, tiebreaks: false, from: null, to: null },
  total_meetings: 3,
  closeness: {
    deciders: { matches: 2, wins: [1, 1], incomplete: 0 },
    scored: 3,
    tiebreaks: { name: 'tiebreaks', matches: 3, wins: [2, 1] },
  },
}

/** The same rivalry with the URL asking for it the other way round. */
function mirrored(comparison: Comparison): Comparison {
  const flip = <T,>(pair: [T, T]): [T, T] => [pair[1], pair[0]]
  return {
    players: flip(comparison.players),
    record: { ...comparison.record, wins: flip(comparison.record.wins) },
    surfaces: comparison.surfaces.map((split) => ({ ...split, wins: flip(split.wins) })),
    tiers: comparison.tiers.map((split) => ({ ...split, wins: flip(split.wins) })),
    serve: flip(comparison.serve),
    meetings: comparison.meetings.map((meeting) => ({
      ...meeting,
      winner_index: meeting.winner_index === 0 ? 1 : 0,
      tiebreaks: meeting.tiebreaks === null ? null : flip(meeting.tiebreaks),
    })),
    filters: comparison.filters,
    total_meetings: comparison.total_meetings,
    closeness: {
      ...comparison.closeness,
      deciders: { ...comparison.closeness.deciders, wins: flip(comparison.closeness.deciders.wins) },
      tiebreaks: { ...comparison.closeness.tiebreaks, wins: flip(comparison.closeness.tiebreaks.wins) },
    },
  }
}

const profile = {
  slug: 'bjorn-borg',
  name: 'Bjorn Borg',
  tour: 'atp',
  country: 'Sweden',
  hand: 'R',
  height_cm: 180,
  birth_date: '1956-06-06',
  pro_since: 1973,
  career: null,
  serve: { availability: AvailabilityRecorded, matches_with_data: 0, rates: null },
  ratings: [
    {
      surface: 'overall',
      matches: 807,
      current: { elo: 2210, as_of: '1983-01-03' },
      peak: { elo: 2350, as_of: '1980-06-30' },
    },
  ],
}

const emptySeries = { surface: 'overall', from: '', to: '', points: [] }

/** What the search endpoint answers a picker with. */
const searchResults = {
  data: [
    { slug: 'bjorn-borg', name: 'Bjorn Borg', tour: 'atp', country: 'SWE', matches: 764, best_tier: 'tour', score: 1 },
    { slug: 'john-mcenroe', name: 'John McEnroe', tour: 'atp', country: 'USA', matches: 1102, best_tier: 'tour', score: 0.9 },
  ],
  next_cursor: null,
}

const simulation = {
  best_of: 3,
  surface: 'overall',
  chain: { point: [0.64, 0.62], hold: [0.8, 0.78], set: [0.55, 0.45], match: [0.564, 0.436] },
  availability: 'available',
}

/** Answers each endpoint the page asks for, and nothing else. */
let requested: string[] = []

function stub(comparison: Comparison, charted: unknown = null) {
  requested = []
  vi.stubGlobal('fetch', (input: string) => {
    requested.push(String(input))
    const path = new URL(String(input), 'http://localhost').pathname
    const body = path.startsWith('/api/v1/h2h/')
      ? comparison
      : path.startsWith('/api/v1/charted/')
        ? charted
        : path === '/api/v1/players'
          ? searchResults
          : path.endsWith('/ratings')
            ? emptySeries
            : path.endsWith('/matches')
              ? { data: [], next_cursor: null }
              : path === '/api/v1/simulate/match'
                ? simulation
                : profile
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/h2h" element={<HeadToHead />} />
        <Route path="/h2h/:a/:b" element={<HeadToHead />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('HeadToHead', () => {
  it('renders the whole comparison for two tour players', async () => {
    stub(rivalry)
    renderAt('/h2h/bjorn-borg/john-mcenroe')

    expect(await screen.findByRole('link', { name: 'Bjorn Borg' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'John McEnroe' })).toBeInTheDocument()
    expect(screen.getByText('1-2')).toBeInTheDocument()
    // The event is a link to its sheet, the round typed after it.
    expect(screen.getByRole('link', { name: 'Wimbledon' })).toHaveAttribute('href', '/tournaments/wimbledon-atp/1980')
    expect(screen.getByText('Break points saved')).toBeInTheDocument()
  })

  // One of the three meetings is charted: one mark, and the sheet opens under
  // it with player A on the A side whoever won.
  it('opens the charted meeting under its row and marks no other', async () => {
    const first = meetings[0] as Meeting
    const withChart: Comparison = {
      ...rivalry,
      meetings: [
        { ...first, charting_id: '19800705-M-Wimbledon-F-Bjorn_Borg-John_Mcenroe' },
        ...meetings.slice(1),
      ],
    }
    stub(mirrored(withChart), {
      charting_id: '19800705-M-Wimbledon-F-Bjorn_Borg-John_Mcenroe',
      played_on: '1980-07-05',
      charted_by: null,
      tournament: 'Wimbledon',
    event_slug: 'wimbledon-atp',
      season: 1980,
      round: 'F',
      score: first.score,
      players: [
        { slug: 'bjorn-borg', name: 'Bjorn Borg' },
        { slug: 'john-mcenroe', name: 'John McEnroe' },
      ],
      sets: [
        {
          set: 0,
          lines: [
            { serve_points: 150, aces: 5, double_faults: 3, first_in: 90, first_won: 70, second_in: 57, second_won: 30, bp_faced: 8, bp_saved: 5, return_points: 140, return_points_won: 60, winners: 30, winners_fh: 18, winners_bh: 12, unforced: 25, unforced_fh: 15, unforced_bh: 10 },
            { serve_points: 140, aces: 8, double_faults: 6, first_in: 85, first_won: 66, second_in: 49, second_won: 25, bp_faced: 10, bp_saved: 7, return_points: 150, return_points_won: 65, winners: 40, winners_fh: 22, winners_bh: 18, unforced: 35, unforced_fh: 20, unforced_bh: 15 },
          ],
        },
      ],
    })
    renderAt('/h2h/john-mcenroe/bjorn-borg')

    await screen.findByRole('link', { name: 'Wimbledon' })
    const marks = screen.getAllByRole('button', { name: 'charted' })
    expect(marks).toHaveLength(1)
    await userEvent.click(marks[0] as HTMLElement)

    // McEnroe is player A on this page, so his figures come first though he lost.
    const aces = await screen.findByRole('row', { name: /^Aces/ })
    expect(aces.textContent?.replace(/\s+/g, ' ')).toContain('8 5')
    expect(screen.getByText(/Charted point by point/)).toBeInTheDocument()
  })

  // The URL is the state, so the surface filter has to survive being pasted
  // into another browser.
  // The filters live in the URL and go to the API, which cuts the record,
  // the strip and the serve figures; the page writes the cut in words.
  it('round-trips the filters through the URL and writes the cut in words', async () => {
    stub({
      ...rivalry,
      record: { matches: 1, wins: [1, 0], incomplete: 0 },
      meetings: [meetings[0]!],
      filters: { ...rivalry.filters, surface: 'grass', round: 'F', from: 1980, to: null },
    })
    renderAt('/h2h/bjorn-borg/john-mcenroe?surface=grass&round=F&from=1980')

    expect(await screen.findByText('1-0')).toBeInTheDocument()
    expect(screen.getByText('in finals, on grass, since 1980, of 3 meetings')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Wimbledon' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'US Open' })).not.toBeInTheDocument()
    const url = new URL(requested.find((u) => u.includes('/h2h/')) as string, 'http://localhost')
    expect(url.searchParams.get('surface')).toBe('grass')
    expect(url.searchParams.get('round')).toBe('F')
    expect(url.searchParams.get('from')).toBe('1980')
    // The closeness summary is the rivalry's, not the cut's.
    expect(screen.getByText(/2 of the 3 meetings whose score could be read went to a deciding set/)).toBeInTheDocument()
  })

  it('reads an empty cut as an answer naming the filter', async () => {
    stub({
      ...rivalry,
      record: { matches: 0, wins: [0, 0], incomplete: 0 },
      meetings: [],
      filters: { ...rivalry.filters, level: 'challenger' },
    })
    renderAt('/h2h/bjorn-borg/john-mcenroe?level=challenger')
    expect(await screen.findByText('They never met at Challenger level')).toBeInTheDocument()
    // One on the controls and one on the empty state: both clear the cut.
    expect(screen.getAllByRole('button', { name: 'Show every meeting' })).toHaveLength(2)
  })

  // Asking the other way round is the same rivalry read from the other end:
  // the same three matches, the same winners, the sides swapped.
  it('is the same comparison with the players swapped', async () => {
    stub(mirrored(rivalry))
    renderAt('/h2h/john-mcenroe/bjorn-borg')

    expect(await screen.findByText('2-1')).toBeInTheDocument()
    const rows = screen.getAllByRole('row')
    const wimbledon = rows.find((row) => row.textContent?.includes('Wimbledon'))
    expect(wimbledon).toHaveTextContent('Bjorn Borg')
  })

  it('reads as an answer, not an error, when they never met', async () => {
    stub({
      ...rivalry,
      record: { matches: 0, wins: [0, 0], incomplete: 0 },
      surfaces: [],
      tiers: [],
      meetings: [],
      serve: [
        { availability: AvailabilityNeverForTier, matches_with_data: 0, rates: null },
        { availability: AvailabilityNeverForTier, matches_with_data: 0, rates: null },
      ],
      total_meetings: 0,
      closeness: { deciders: { matches: 0, wins: [0, 0], incomplete: 0 }, scored: 0, tiebreaks: { name: 'tiebreaks', matches: 0, wins: [0, 0] } },
    })
    renderAt('/h2h/bjorn-borg/john-mcenroe')

    expect(await screen.findByText('These two have never met')).toBeInTheDocument()
    expect(screen.getByText('0-0')).toBeInTheDocument()
    // Still a comparison: the ratings section is a real answer for two players
    // who never played each other.
    expect(screen.getByText('Overall Elo')).toBeInTheDocument()
  })

  it('explains a rivalry played entirely at a tier with no statistics', async () => {
    stub({
      ...rivalry,
      serve: [
        { availability: AvailabilityNeverForTier, matches_with_data: 0, rates: null },
        { availability: AvailabilityNeverForTier, matches_with_data: 0, rates: null },
      ],
    })
    renderAt('/h2h/bjorn-borg/john-mcenroe')

    expect(await screen.findByText('No serve statistics for these meetings')).toBeInTheDocument()
    expect(screen.getByText(/No Futures or ITF match has ever recorded/)).toBeInTheDocument()
    // The record is still there. Only the statistics are missing.
    expect(screen.getByText('1-2')).toBeInTheDocument()
  })

  // The bug this test exists for: choosing on an empty page cleared the box and
  // remembered nothing, so no pair could ever be built from /h2h.
  it('remembers the first pick until the second one arrives', async () => {
    stub(rivalry)
    const user = userEvent.setup()
    renderAt('/h2h')

    const first = await screen.findByRole('combobox', { name: 'First player' })
    await user.type(first, 'borg')
    await user.click(await screen.findByText('Bjorn Borg'))

    // Still on the picker, with the choice visible rather than thrown away.
    expect(first).toHaveValue('Bjorn Borg')
    expect(screen.getByText('Pick two players')).toBeInTheDocument()

    const second = screen.getByRole('combobox', { name: 'Second player' })
    await user.type(second, 'mcenroe')
    await user.click(await screen.findByText('John McEnroe'))

    // Both known, so the comparison is a page now.
    expect(await screen.findByText('1-2')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Bjorn Borg' })).toBeInTheDocument()
  })

  // Once a comparison exists, the boxes name the players it is actually of --
  // not the last thing typed into them.
  it('shows who is being compared', async () => {
    stub(rivalry)
    renderAt('/h2h/bjorn-borg/john-mcenroe')

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: 'First player' })).toHaveValue('Bjorn Borg')
    })
    expect(screen.getByRole('combobox', { name: 'Second player' })).toHaveValue('John McEnroe')
  })

  it('asks for two players before it compares anything', async () => {
    stub(rivalry)
    renderAt('/h2h')

    expect(await screen.findByText('Pick two players')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'First player' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Second player' })).toBeInTheDocument()
  })
})
