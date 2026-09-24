---
name: Deucepoint
description: Cream, ink and white, with white cards and pages that move when they arrive; colour only where it tells two players or four surfaces apart.
colors:
  bond: "#f6f6f2"
  highlight: "#ecece6"
  card: "#ffffff"
  card-line: "#e8e7e0"
  ink: "#17181a"
  pencil: "#66655f"
  pencil-mid: "#8a8881"
  pencil-light: "#cfcdc5"
  ball: "#d4f53c"
  player-a: "#5b3ae6"
  player-a-wash: "#ece8f8"
  player-b: "#d0186b"
  player-b-wash: "#f5e6ec"
  third: "#d9820f"
  clay: "#a84f1f"
  clay-wash: "#f8e7dd"
  hard: "#2e6fa8"
  hard-wash: "#e2ecf7"
  grass: "#3f6b35"
  grass-wash: "#e2eddc"
  carpet: "#4f6f7a"
  win: "#3f6b35"
  loss: "#c23a2b"
typography:
  family: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
  display: "clamp(40px, 7vw, 76px) / 700 / -0.035em"
  figure: "clamp(40px, 6vw, 64px) / 700 / -0.04em"
  headline: "20px / 700"
  title: "16px / 600"
  body: "14px / 400 / 1.55"
  data: "13px / 400"
  caption: "12px / 400 / pencil"
  kicker: "11-12px / 600 / uppercase / 0.08em"
rounded:
  card: "8px"
  badge: "4px"
  pill: "999px"
spacing: [4, 8, 12, 16, 24, 32, 48, 64]
---

# Design System: Deucepoint

Reasoning is in [ADR-0014](docs/decisions/0014-court-colour.md), which supersedes the visual half of [ADR-0010](docs/decisions/0010-the-draw-sheet.md). This file records what shipped in `web/src`. The absence system and the typed marks from the draw sheet stand unchanged.

## Overview

**Creative North Star: "Cream, ink and white, in motion."** A cream ground, white cards, black type and one monospaced face. Colour is kept for the places it does a job: violet is player A and magenta player B wherever two players are compared (head to head, odds, the simulator, chart lines), and surfaces keep their colours. Everything else is ink. Pages arrive rather than appear: headlines rise word by word, cards rise in, figures roll up, bars grow, lines draw in, and the home page loops a rally over a black-and-white court.

**Key characteristics**
- Cream `--bond` ground; white cards (8px radius, faint shadow) hold every section.
- Violet against magenta in every comparison, on names, figures, bars and lines; panels stay white.
- No decorative colour: accents, badges, buttons and hovers are ink on cream or white.
- Surfaces keep their ink and gain a wash for badges (`HARD`, `CLAY`, `GRASS`).
- Martian Mono for everything, with fluid display and figure sizes for names and headline numbers.
- Motion on arrival, triggered by scrolling into view, removed under reduced motion.

## Colors

Every colour is a custom property in `web/src/styles/tokens.css`.

- **Ground:** `--bond` for the page, `--card` for panels, `--card-line` for rules inside a card, `--highlight` for empty tracks and quiet chips.
- **Ink and pencils:** ink for entries and figures; pencil for captions, kickers and meta; pencil-mid and pencil-light for fields and ruling.
- **Sides:** `--player-a` (violet) and `--player-b` (magenta) for names, tallies, bars, chart lines and form pills in a comparison. A single player's chart and form are ink. In a ranked chart, the top three lines are violet, magenta and `--third` (amber).
- **Accent:** ink. The `--lime*` token names survive but point at ink, dark grey and white; `--ball` (real lime) is kept for the scoreboard's serve light only.
- **Surfaces:** ink for text, bars and courts; wash for badge grounds (`surfaceVar`, `surfaceWash` in `lib/surface.ts`).
- **Outcomes:** win and loss on rank deltas; W/L marks are ink (won) and grey (lost) squares.

**The name-beside-colour rule.** Colour is never the only encoding: a side's colour always sits beside its name or figure, a surface colour beside its word, a W/L colour inside its letter.

## Typography

One family, Martian Mono, self-hosted. Names and headline numbers use the fluid `--t-display` and `--t-figure`; everything else uses the fixed steps `--t-11` to `--t-32`. Kickers (small uppercase, letterspaced, pencil) label figures and sections. Scores, spans and records are still typed as the source writes them (`6-4 7-6(3)`, `380-89`), and a set never wraps.

## Layout

A single column up to 1248px, padded 16px on a phone and 32px from 880px, with one breakpoint at 880px. The header is sticky and frosted: brand ball, tabs (active one in brackets with an ink underline), and search.

- **Page opener:** `PageHeader`: a kicker with an accent dash (or a pulsing ink dot), the display title, a standfirst, and optional art on the right (`CourtArt`).
- **Home:** hero with the black-and-white court, a large search with an ink button and "Try" chips; then three columns (Elo top 5, the rivalry at the top, last week's finals); then the leaders chart, the coverage table and the replayed draw as cards.
- **Player:** name at the left, a white Elo card at the right (count-up rating, peak, last-10 form pills); four career tiles; rating history (2fr) beside Elo by surface (1fr); the remaining sections as cards.
- **Head to head:** two white player panels, each with its colour dash, around a large counting tally, the split bar under all three; the filters in one card; the match simulator beside the by-surface splits; finals, slam finals, deciding sets and tiebreak tiles; recent meetings as two-column cards; the full sheet below.
- Everything is one column below 880px, and a table wider than a phone scrolls inside its card.

## Shapes & Depth

Cards have an 8px radius and `--shadow`. On hover a card lifts to `--shadow-lift` and moves 2-4px. Badges have a 4px radius. Toggles and filter cells are pills inside a pill tray: ink when active, or the surface colour for a surface filter. Buttons are ink with an 8px radius; on hover white slides in from the left. Empty states are dashed cards.

## Motion

All motion lives in `base.css` keyframes (`dp-rise`, `dp-fade`, `dp-grow-x`, `dp-pop`, `dp-draw`, `dp-pulse`) and a few components:

- **Reveal / Card:** rise 14px and fade in over 700ms, staggered by `delay`, the first time the element is in view (`lib/useInView.ts`). Anything already on screen when it mounts reveals at once.
- **Odometer:** big whole figures roll up digit by digit like a scoreboard.
- **CountUp:** runs a figure up from zero over about 1.1s with an ease-out; screen readers get only the final value, and a timer guarantees it lands.
- **Bars:** split bars, surface bars, odds bars and tally bars grow from their origin.
- **Charts:** lines draw in (`stroke-dasharray` with `pathLength=1`); area washes fade in after them; peak and end markers pop.
- **FormPills:** pop in one after another.
- **CourtArt:** court lines draw in; a white ball loops along a dashed flight with a bounce ring; the court is ink with white lines.
- **Ambient:** the brand ball and kicker dot pulse.

Under `prefers-reduced-motion: reduce`, `base.css` collapses every animation. `CountUp` and the ball's `animateMotion` check `matchMedia` themselves.

## Components

New in this system (`web/src/components`, all shown at `/_components`): `Card`/`Kicker`, `Reveal`, `CountUp`, `PageHeader`, `CourtArt`, `FormPills`, `SurfaceBadge`, `AreaChart`. Restyled: `Button`, `PlayerSearch`, `StatTable` (a card with uppercase heads and a grey row hover), `StatRow`, `SplitBar`, `WinSplit`, `SurfaceToggle`/`TourFilter` (pills), `SurfaceEloStrip` (bars), `RecentFinals` (cards), `SeedingSheet` and `TrajectoryChart` (coloured leaders), `WinLossMark`, `Skeleton` (shimmer), `EmptyState` (dashed card), `Scoreboard` (ink board, lime serve light), `DrawSheet`, `RoundList`.

### The absence system (unchanged)
- **AbsentCell:** `n/r` in pencil, aligned like the numbers around it. Never `0`, blank or a dash.
- **PartialAggregate:** a caption declaring the denominator ("covers the 41 of 68 matches that did").
- **EmptyState:** names what is absent, says why, and offers somewhere complete to go.

### The charted sheet (unchanged in behaviour)
The `charted` mark after a score opens the per-set sheet under its row; nothing charted feeds an aggregate.

## Do's and Don'ts

**Do**
- Take every colour from `tokens.css`; violet is A and magenta is B everywhere.
- Put each section on a card, and open each page with `PageHeader`.
- Gate every animation on scroll-into-view and reduced motion; let figures settle on the API's value.
- Keep a side's name beside its colour and a surface's word beside its colour.
- Type `n/r` for an unrecorded value and explain it in the caption.

**Don't**
- Add colour for decoration; ask where a colour tells something apart first.
- Loop motion other than the court rally and the brand pulse.
- Render an absence as `0`, a blank or a dash.
- Add photography or a second type family.
