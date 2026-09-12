# Frontend design handoff

> **Superseded in part, 2026-09-12.** The visual system described here (Archivo, hairlines
> on paper, the wash on the best surface) was replaced by the draw sheet:
> [`DESIGN.md`](../../DESIGN.md) is the design record and
> [ADR-0010](../decisions/0010-the-draw-sheet.md) the reasoning. The product rules below,
> the absence system above all, are unchanged and still bind. The mockups in
> [`mockups/`](mockups/) are the old world and stay as history.

This document answered the open questions from the frontend scaffold discussion and
defined the first design system. Where it and `DESIGN.md` disagree, `DESIGN.md` wins.

## Answers to the open questions

**Styling approach: CSS Modules + custom properties.** No Tailwind, no shadcn/Radix, no
component library. The design depends on a small, strict token system and ruled
structure; utility classes and pre-styled components fight both. Already an ADR in the
spec (§4) — this confirms it.

**Mobile: mobile-first.** Build every component to work at 360px, then widen: at ≥880px
the player page becomes two columns (figures left, tables right), the home page puts the
form lines beside the seeds, and the rankings table gains columns (peak Elo, age). Same
components, no separate mobile variants.

**Chart dependency: none.** Every chart is hand-rolled inline SVG: the sparkline, the
leaders' trajectories, the two-player pair, and the seeding sheet. Do not add a charting
library.

**EmptyState: it's three components, not one.** See "The absence system" below — this is
the most important section of this document.

## Rules that make the palette work

- **Hue means surface.** Clay/hard/grass/indoor colours appear only when the value is
  surface-scoped: surface squares, surface Elo, the surface filter, draw-sim bars for a
  grass tournament. Never as decoration.
- **Players are monochrome.** In any two-player comparison, player A is ink, player B is
  pencil. Never distinguish players by hue — it would collide with surface encoding.
- Win and loss colours are for W/L marks and rank deltas only.
- No other colours. No gradients. No shadows. No cards. No radius on data.
- Sentence case everywhere. No all-caps labels, no eyebrows.
- Every caption or footnote that explains a symbol lives directly under the element it
  explains.
- Meta strings are typed fields two spaces apart: `Spain  right-handed  23  turned pro
  2018`. Always through the `Meta` component, so the gap cannot drift between screens.

## The absence system

83% of match rows carry no serve statistics, so absence renders more often than presence.
Three distinct cases, three distinct components:

**1. `AbsentCell` — one missing value in a populated table.**
Typed `n/r` in pencil, aligned like the numbers around it. `title="Not recorded"` and an
`aria-label` to match. The column remains sortable; absent values sort after all present
values regardless of direction. Never `0`, never blank, never `N/A`. The caption under
the table says what `n/r` means there.

**2. `PartialAggregate` — a summary over a gappy column.**
Aggregates are computed **only over recorded rows** and must declare their denominator in
an adjacent caption: "These figures cover the 41 of 68 matches that did." An average that
silently skips gaps without saying so is a correctness bug wearing a design costume.

**3. `EmptyState` — a whole section with nothing to show.**
Not a panel, not an illustration, no icon. A heading naming what's absent, one or two
sentences saying *why* (era, tour level, tournament), then **a redirect to something
complete** — a button or link to data that exists. Emptiness is direction, not mood.

**4. The degenerate case — a player with zero matches.**
The identity header always renders (name, country, hand). The body is a single
EmptyState explaining what the dataset covers and linking to search. The page never 404s
for a player that exists in the players table.

Global rule behind all four: **NULL is never rendered as zero, and nothing that's absent
is hidden.** Hiding the column would misrepresent the dataset; showing `n/r` tells the
truth about it.

## Component inventory

| Component | Notes |
|---|---|
| `SeedingSheet` | The home page's first viewport: form lines, seeds, and a ruled step from each line end to its row. The site's one piece of ambient motion. |
| `SplitBar` | Centre-out paired bar. 6px tall, ink left / pencil right, 2px bond gap at centre. Numbers at outer edges, label centred. |
| `StatTable` | Ruled rows, head and total under and over ink rules, sortable headers, cells never wrap. Uses `AbsentCell` + `PartialAggregate`. |
| `AbsentCell` | As above. |
| `EmptyState` | As above. |
| `SurfaceToggle`, `TourFilter` | Typed filter cells; the active one is boxed in its hue (ink for a tour). Selection persists to the URL. |
| `SurfaceDot` | 8px square in the surface colour, always adjacent to a text label. |
| `WinLossMark` | Bold W/L in win/loss, always with the letter. |
| `RankDelta` | Signed integer in win/loss, `0` in pencil. |
| `Sparkline`, `TrajectoryChart`, `TrajectoryPair` | Hand-rolled SVG over the sheet's ruling; names at the ends of their lines, never a legend. |
| `RivalryStrip` | Row of 13px squares: filled = player A won, outlined = player B won; fill/stroke colour = surface. Legend caption underneath. |
| `Meta` | Typed fields two spaces apart. Empty and absent parts drop out. |
| `Button` | A label boxed in one rule. Says what happens: "Simulate this matchup", not "Go". |
| `PlayerSearch` | The typed field: prompt, entry, one rule; a combobox underneath. |

## Quality floor (unchanged from spec §10.4)

Responsive to 360px · visible keyboard focus · colour never the only encoding · 4.5:1
contrast · skeleton loading states matching final layout · every error names what
happened and what to do. `prefers-reduced-motion` collapses the seeding sheet's draw-in
to a static render — that draw is the site's entire ambient motion budget.

## One thing the old mockups asked for that the data cannot answer

`mockups/player-profile.png` includes a **rally-length distribution**. Nothing in the
database can produce it. The ingest reads match-level CSVs only, so rally length would
need the Match Charting Project or the slam point-by-point repository added as a source
first. Tracked on #47.
