# Frontend design handoff

This document answers the open questions from the frontend scaffold discussion and
defines the design system. It is the source of truth for visual decisions; the mockup
images in `docs/design/mockups/` show the intended result. Where an image and this
document disagree, this document wins.

## Answers to the open questions

**Styling approach: CSS Modules + custom properties.** No Tailwind, no shadcn/Radix, no
component library. The design depends on a small, strict token system and hairline-based
structure; utility classes and pre-styled components fight both. Already an ADR in the
spec (§4) — this confirms it.

**Mobile: mobile-first.** All mockups are 360–400px layouts and they are the primary
design, not a degraded desktop. Build every component to work at 360px, then widen:
at ≥880px the player page becomes two columns (identity + ratings left, tables right)
and the rankings table gains columns (peak Elo, age). Same components, no separate
mobile variants.

**Chart dependency: none in the scaffold.** The player page uses a hand-rolled inline
SVG sparkline — a pure presentational component taking `{date, elo}[]` and nothing else,
~40 lines. The full TrajectoryChart (multi-series, hover readout, 60 years of data)
arrives with the ratings endpoint work and gets its own ADR if it needs visx. Do not add
a charting library now.

**EmptyState: it's three components, not one.** See "The absence system" below — this is
the most important section of this document.

## Tokens

```css
:root {
  /* ground */
  --paper:      #F7F7F5;   /* page background */
  --chalk:      #E2E2DD;   /* hairlines. 1px, always */
  --chalk-hard: #14161A;   /* aggregate/total rules in tables */

  /* ink ramp — also the player-pair palette */
  --ink:        #14161A;   /* primary text; player A in comparisons */
  --ink-2:      #5A5D61;   /* secondary chart series */
  --ink-3:      #9A9DA1;   /* player B in comparisons; absent-value dashes */
  --ink-4:      #C9CAC6;   /* background chart series, "the field" */
  --muted:      #6E7175;   /* secondary text, captions, labels */

  /* surface semantics — never decorative */
  --clay:       #C4622D;
  --hard:       #2E6FA8;
  --grass:      #4A7A3F;
  --indoor:     #6B5B95;
  --clay-wash:  #FCF6F1;   /* highlight fill, e.g. best-surface cell */

  /* outcomes */
  --win:        #4A7A3F;
  --loss:       #B0483A;
}
```

Rules that make the palette work:

- **Hue means surface.** Clay/hard/grass/indoor colors appear only when the value is
  surface-scoped: surface dots, surface Elo, surface filter states, draw-sim bars for a
  clay tournament. Never as decoration.
- **Players are monochrome.** In any two-player comparison, player A is `--ink`, player B
  is `--ink-3`. Never distinguish players by hue — it would collide with surface encoding.
- `--win`/`--loss` are for W/L markers and ranking deltas only.
- No other colors. No gradients. No shadows anywhere.

## Typography

- **Archivo** (Google Fonts), weights 400 / 600 / 700. Display and scoreline numerals use
  700; the variable width axis (expanded) is welcome where supported, but 700 alone is the
  fallback and the mockups use it.
- Scale (1.25): 11 / 12 / 13 / 14 / 16 / 20 / 24px in current mockups; don't invent sizes
  between steps.
- `font-variant-numeric: tabular-nums` on **every** element containing data. This is the
  most load-bearing single line of CSS in the project.
- Sentence case everywhere. No all-caps labels, no letter-spaced eyebrows.
- Body/caption line-height 1.5–1.55; data rows 1.2.

## Structure

- Hairlines (`1px solid var(--chalk)`) separate data regions — this replaces cards.
  Content sits directly on `--paper`.
- Table aggregate rows get a `--chalk-hard` top rule (see the match-log mockup) — the
  heavier line is the "double rule" of a ledger total.
- Border-radius: 4px on interactive controls (buttons, inputs), 2–3px on tiny color
  chips, **0 on all data regions and tables**.
- Buttons: 1px `--ink` border, transparent background, 600 weight, 8px 14px padding.
  Active filter states: 2px bottom border in `--ink` (or the surface color when the
  filter IS a surface).
- Every caption or footnote that explains a symbol lives directly under the element it
  explains, 11px `--muted`.
- Meta strings join their parts with a middle dot and hair-thin spacing: "Spain ·
  right-handed · 23 · turned pro 2018", "Cincinnati QF · 6–4 7–5", "ATP · all surfaces".
  Always through the `Meta` component, so the separator and its spacing cannot drift
  between screens, and never as a decorative flourish anywhere else.

## The absence system

83% of match rows carry no serve statistics, so absence renders more often than presence.
Three distinct cases, three distinct components:

**1. `AbsentCell` — one missing value in a populated table.**
An em-dash (`—`) in `--ink-3`, right-aligned like the numbers around it. `title="Not
recorded"` and an `aria-label` to match. The column remains sortable; absent values sort
after all present values regardless of direction. Never `0`, never blank, never `N/A`.

**2. `PartialAggregate` — a summary over a gappy column.**
Aggregates are computed **only over recorded rows** and must declare their denominator in
an adjacent caption: "Averages cover the 41 of 68 matches that did." An average that
silently skips gaps without saying so is a correctness bug wearing a design costume.

**3. `EmptyState` — a whole section with nothing to show.**
Not a panel, not an illustration, no icon. A muted heading naming what's absent, one or
two sentences saying *why* (era, tour level, tournament), then **a redirect to something
complete** — a button or link to data that exists. Emptiness is direction, not mood. See
the Borg mockup: peak Elo, career record, and rivalries render fully; only the stats
section explains itself.

**4. The degenerate case — a player with zero matches.**
The identity header always renders (name, country, hand). The body is a single
EmptyState explaining what the dataset covers and linking to search. The page never 404s
for a player that exists in the players table.

Global rule behind all four: **NULL is never rendered as zero, and nothing that's absent
is hidden.** Hiding the column would misrepresent the dataset; showing the dash tells the
truth about it.

## Component inventory for the scaffold

| Component | Notes |
|---|---|
| `SplitBar` | Center-out paired bar. 6px tall, `--ink` left / `--ink-3` right, 1px `--paper` gap at center. Numbers at outer edges, label centered, 13px. |
| `StatTable` | Hairline rows, tabular nums, sortable headers, sticky header on desktop. Uses `AbsentCell` + `PartialAggregate`. |
| `AbsentCell` | As above. |
| `EmptyState` | As above. |
| `SurfaceToggle` | Text filter row; active item gets a 2px underline in its surface color. Selection persists to the URL. |
| `SurfaceDot` | 7–8px rounded square in the surface color, always adjacent to a text label (color is never the only encoding). |
| `WinLossMark` | Bold W/L in `--win`/`--loss`, always with the letter (again: not color-only). |
| `RankDelta` | Signed integer in `--win`/`--loss`, `0` in `--muted`. |
| `Sparkline` | Hand-rolled SVG, no axes, no labels, single `--ink` line, baseline hairline. |
| `RivalryStrip` | Row of 13px squares: filled = player A won, outlined = player B won; fill/stroke color = surface. Legend caption underneath. |
| `Meta` | Joins meta parts with a middle dot. Empty and absent parts drop out rather than leaving a stranded separator. |
| `Button` | As in Structure above. Label says what happens: "Simulate this matchup", not "Go". |

## Screen index

Mockup images in `docs/design/mockups/` (mobile layouts, the primary design):

1. `home.png` — nav, hero trajectory chart (top 8, top 3 labeled, rest `--ink-4`),
   Elo leader strip with best-surface annotation, attribution footer.
2. `player-profile.png` — identity header, surface Elo strip (best surface gets
   `--clay-wash`-style highlight in its own surface tint), clutch vs tour average,
   rally-length distribution, recent results.
3. `head-to-head.png` — career score, surface filter, SplitBars, RivalryStrip, simulate
   button.
4. `simulator.png` — win split, stage-by-stage breakdown with the amplification caption,
   draw-sim odds with ±CI whiskers.
5. `rankings.png` — Elo vs official with RankDelta column and explanatory caption.
6. `empty-state-borg.png` — the pre-1991 EmptyState pattern.
7. `match-log-absence.png` — AbsentCell + PartialAggregate in one table.

> The image files are not in the repository yet. This document is written to stand
> without them, and it wins over them where they disagree.

## Quality floor (unchanged from spec §10.4)

Responsive to 360px · visible keyboard focus · color never the only encoding · 4.5:1
contrast · skeleton loading states matching final layout · every error names what
happened and what to do. `prefers-reduced-motion` collapses the hero line-draw to a
static render — that draw is the site's entire ambient motion budget.

## Do not

- No shadows, cards, gradients, or icon sets.
- No all-caps, no arrows appended to button text.
- No hue for anything except surfaces and win/loss.
- No charting library in the scaffold.
- No component library. The inventory above is the component library.

## One thing the mockups ask for that the data cannot answer

`player-profile.png` includes a **rally-length distribution**. Nothing in the database can
produce it. The ingest reads match-level CSVs only — `configs/sources.json` lists six
match sources and none of them is point-by-point — so rally length would need the Match
Charting Project or the slam point-by-point repository added as a source first. The
README's "Futures and ITF carry no point-level data" understates it: no tier has any.

**Clutch vs tour average is unaffected** and stays. Break points saved and faced are
recorded per match, so both a player's figure and the tour baseline are computable from
what is already ingested.

Tracked on #47.
