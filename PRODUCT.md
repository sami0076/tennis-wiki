# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Tennis fans and analysts who come for the numbers: checking a player's form on one
surface before a match, settling a rivalry argument, reading how a Futures or ITF player
rates against the tour, replaying a draw. They arrive with a name or a matchup in mind,
often on a phone during a tournament week. Confirmed 2026-09-11: fans first.

Second audience, also confirmed: hiring managers and portfolio reviewers who open the
link from a CV. The first viewport has to land as design work; the data pages prove
depth. They never outrank the fan.

## Product Purpose

Deep per-player statistics, head-to-head comparison, and first-principles match and
draw simulation for both the ATP and WTA tours, built from raw match data with the
working shown. Around 1.6 million matches across tour, Challenger, Futures and ITF, and
over 115,000 players, rated on one Elo scale. Success is a fan trusting a number because
the site said where it came from and what it does not cover.

## Positioning

Three things no free tennis site does together: the women's tour on the same footing as
the men's; every player, not just the famous ones (a player ranked 400 gets a real page);
and simulation that shows every rung from point-win probability to match probability.
The closest prior art, Ultimate Tennis Statistics, is ATP-only.

The mechanism a neighbour could not truthfully copy: absence is disclosed, never hidden
or zeroed. 83% of match rows carry no serve statistics, so the site renders "not
recorded, and here is why" more often than it renders a number, and every aggregate
declares its denominator.

## Operating Context

Routes: `/` (Elo leaders, trajectory chart, what the database holds), `/players`
(search, tour filter), `/players/:slug` (identity, surface Elo, clutch vs tour average,
career, official ranking, serve, splits by surface and tier, match log), `/h2h` and
`/h2h/:a/:b` (career score, surface filter, split bars, rivalry strip, every meeting,
simulate button), `/rankings` (Elo vs official, rank delta), `/simulator` (two pickers,
the chain from point to match, the chance of each set score, one match played out from the
chain on request and labelled a sample of the model, a draw played ten thousand times), `/methodology`
(generated from `docs/methodology.md`), `/_components` (every component in every state).

Player search lives in the header on every page: a combobox, debounced, every row
carrying tour, country, career match count and best tier, because at 115,000 players a
name is not an identifier. Query and filters live in the URL so any view is a link.

Frontend: React 18, TypeScript, Vite, CSS Modules and custom properties, `web/`. Types
generated from the Go API structs. Dev: `make api` on :8080, `make web` on :5173.
Seed fixture of ~4,100 real matches covers every data regime. Served as a static SPA.

## Capabilities and Constraints

- Read-only API. No accounts, comments, or social features. No live scores. No betting
  odds, tipping, or predictions framed as picks; the simulator reports probabilities.
- Data through 2026-01-17 (ATP) and 2024-12-31 (WTA); the gap is disclosed, not filled.
  `/api/v1/coverage` reports the real dates from the database.
- A ranking is always as of the last week that exists, never today, and says which week.
- Product rules the redesign keeps (confirmed 2026-09-11): hue means surface and is
  never decorative; the two sides of any comparison are orange (A) and turquoise (B), never named by
  colour alone (amended 2026-09-12 from monochrome, at the user's request); win/loss colour
  only on W/L marks and rank deltas; absence is never rendered as zero and nothing
  absent is hidden (AbsentCell, PartialAggregate, EmptyState are three distinct cases);
  tabular numerals on every element carrying data; colour is never the only encoding.
- Surfaces: hard, clay, grass, carpet (shown as indoor). Tours: ATP, WTA. Tiers: tour,
  Challenger, Futures, ITF, qualifying.
- Doubles exist in the data but are not modelled (v1). No mobile apps; responsive to
  360px is the mobile investment.
- Styling stack is CSS Modules plus custom properties with no Tailwind, no component
  library, no charting library, no icon set (spec §4). Changing that needs an ADR.
- Undecided: whether a dark theme ships. Not required by the product; open to the
  visual direction.

## Brand Commitments

- Name: **Deucepoint**. The repository stays `tennis-wiki`. No logo or mark exists yet.
- Non-commercial, forever: no ads, no payment flows, no paid tier (CC BY-NC-SA 4.0).
- Attribution to Jeff Sackmann / Tennis Abstract in the footer of every page. Required
  by the data licence, not optional.
- Voice: plain sentences that say what happened and what to do, in sentence case.
  Captions explain the symbol directly under the element. Buttons say what happens
  ("Simulate this matchup", not "Go"). Nothing in the copy pretends a gap is a zero.
- The incumbent look (Archivo, hairlines on paper, `docs/design/design-system.md`,
  `docs/design/mockups/`) is evidence of the subject, not authority over the redesign.
  Full replacement of the visual world confirmed 2026-09-11.

## Evidence on Hand

- Real data through the API and the seed fixture; every number on the site is queried.
- Validation: tour-level predictive accuracy 69.9%; Wimbledon 2019 draw gives Djokovic
  40.1% ±1.0 and he won it; 296 real draws score 0.855 Brier. All in
  `docs/methodology.md` and `docs/validation.json`.
- Nine ADRs in `docs/decisions/`, `docs/performance.md` with measured query costs.
- No player photographs, and none may be fabricated or scraped. No customer quotes,
  press, or testimonials exist; none may be invented.
- No screenshots of the current site exist yet (README placeholder).

## Product Principles

1. Never render an absence as a zero, and never hide it. Say which kind of never.
2. Show the working. Every figure names its inputs, denominator and population.
3. Both tours on one footing, every player at full depth.
4. The URL is the state; any view is a link somebody can send.
5. Colour and emphasis carry meaning only. Nothing decorative claims to be data.

## Accessibility & Inclusion

Quality floor from the build spec: responsive to 360px, visible keyboard focus, colour
never the only encoding, 4.5:1 contrast, skeleton loading states matching the final
layout, every error names what happened and what to do, `prefers-reduced-motion`
collapses all ambient motion to a static render.
