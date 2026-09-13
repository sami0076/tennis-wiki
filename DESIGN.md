---
name: Deucepoint
description: Every page is a sheet from the tournament office - bond paper, one ink, one pencil, ruled lines, and colour only where a value is surface-scoped.
colors:
  bond: "#f6f6f2"
  highlight: "#ecece6"
  ink: "#17181a"
  pencil: "#66655f"
  pencil-mid: "#8a8881"
  pencil-light: "#cfcdc5"
  clay: "#a84f1f"
  hard: "#2e6fa8"
  grass: "#4a7a3f"
  indoor: "#6b5b95"
  win: "#4a7a3f"
  loss: "#b0483a"
  player-a: "#0f6e78"
  player-b: "#a8285f"
typography:
  display:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "28px"
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  title:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "normal"
  body:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: "normal"
  data:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.3
    letterSpacing: "normal"
  caption:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: "normal"
  figure:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "28px"
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.015em"
  small:
    fontFamily: "'Martian Mono Variable', 'Martian Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.3
    letterSpacing: "normal"
rounded:
  none: "0"
  square: "1px"
spacing:
  s-1: "4px"
  s-2: "8px"
  s-3: "12px"
  s-4: "16px"
  s-5: "24px"
  s-6: "32px"
  s-7: "48px"
  s-8: "64px"
components:
  button:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.data}"
    rounded: "{rounded.none}"
    padding: "8px 14px"
  button-hover:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.bond}"
  button-disabled:
    backgroundColor: "transparent"
    textColor: "{colors.pencil-mid}"
  field-search:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "5px 0"
  filter-cell:
    backgroundColor: "transparent"
    textColor: "{colors.pencil}"
    typography: "{typography.data}"
    rounded: "{rounded.none}"
    padding: "4px 10px"
  filter-cell-active:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
  nav-tab:
    textColor: "{colors.pencil}"
    typography: "{typography.data}"
  nav-tab-active:
    textColor: "{colors.ink}"
  table-head:
    backgroundColor: "{colors.bond}"
    textColor: "{colors.pencil}"
    typography: "{typography.caption}"
    padding: "6px 8px"
  table-cell:
    textColor: "{colors.ink}"
    typography: "{typography.data}"
    padding: "9px 8px"
  surface-cell-best:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "8px 12px"
  empty-state:
    textColor: "{colors.pencil}"
    typography: "{typography.data}"
    padding: "16px 0 24px"
---

# Design System: Deucepoint

Reasoning for the world is in [ADR-0010](docs/decisions/0010-the-draw-sheet.md). This file records what shipped in `web/src`. Where the superseded handoff at `docs/design/design-system.md` disagrees, this file wins; its absence system and component inventory still hold.

## Overview

**Creative North Star: "The Draw Sheet"**

Every page is a sheet from the tournament office: the typed draw pinned in the players' lounge, with seeds in brackets `[1]`, countries in parentheses `(ESP)`, scores written in as the source writes them `6-4 3-6 6-2`, the marks a sheet uses `ret.` `w/o` `Q`, and ruled lines that step a name into the next round. One monospaced typewriter-grade family carries everything typed, headings and buttons included. Structure is ruled lines, not boxes: a pencil rule under every row, an ink rule at a section head, a table head and a total, and an L-shaped bracket stub where a line steps across to its row.

The sheet is bond, not white. Two greys and one black do all the work of hierarchy: ink for the entry, pencil for the caption and the second player, pencil-mid for the third line of a chart, pencil-light for the ruling. Hue is spent on surface and cannot be borrowed for anything else. Strip the content and what remains is a ruled sheet with bracket stubs.

It refuses the sports-stats dashboard (tiles, one accent, a big chart) and the broadsheet-hairlines-on-cream it replaced. Confirmed visual rejections: cards, shadows, radii, icon sets, gradients, a second type family, photography.

**Key Characteristics:**
- One monospaced family, tabular by construction, at eight fixed sizes from 11px to 32px
- Bond ground, ink and three pencils; four surface hues and two outcome hues, each scoped to one meaning
- Ruled lines at 1px are the entire structural vocabulary; ink for a head or total, pencil-light between rows
- No cards, no shadows, no radius, no icons; selection is a box drawn in one rule
- One breakpoint at 880px; below it the sheet is one column
- One piece of ambient motion, the seeding sheet drawing in, retired after 1400ms and static under reduced motion; everything else that moves over time answers a choice the reader made (the simulator's result revealing, a match they asked to watch), finishes on its own, and renders finished under reduced motion

## Colors

Bond paper, one ink, three pencils, and hue only where a value is surface-scoped. Every colour is a custom property in `web/src/styles/tokens.css`; nothing else in the artifact declares a hex.

### Primary
- **Ink** (`{colors.ink}`): the entry. Body text, names, figures, headings, the active tab, the box around a selected cell, the leader's line, the sort chevron, focus outline, selection background. Fills a hovered button.
- **Bond** (`{colors.bond}`): the ground on every page, the table head background, the text of a hovered button, the 2px gap in a split bar, the background of the search combobox panel.

### Secondary
Four surface hues, darkened to read as text on bond at 4.5:1. A surface hue appears only in a cell whose value is surface-scoped: the 8px square before a surface word, the Elo figure in that surface's cell, the box around the best surface, the box around the active surface filter, the fill of a draw-simulation bar and the fill or stroke of a rivalry square.
- **Clay** (`{colors.clay}`)
- **Hard** (`{colors.hard}`)
- **Grass** (`{colors.grass}`)
- **Indoor** (`{colors.indoor}`): carpet is shown as indoor and shares the value.

### Tertiary
Two outcome hues, for W/L marks, rank deltas, the champion mark, and the one error line a page can carry.
- **Win** (`{colors.win}`): the `W` mark, an upward rank delta, `won` beside a replayed draw's champion. Same value as grass; the letter or sign is always present, so the coincidence never has to be resolved by colour.
- **Loss** (`{colors.loss}`): the `L` mark, a downward rank delta, and a fetch error's sentence.

### Neutral
- **Pencil** (`{colors.pencil}`): every caption, standfirst and footer line; table heads; the seed number and country; a set the side lost, on the board; `n/r`; a rank delta of 0; placeholder text; inactive tabs and filter cells.
- **Pencil-mid** (`{colors.pencil-mid}`): the third line of a chart, the field behind the named lines at 0.6 opacity, the dotted leader between a name and its figure, a disabled button's border and label, the scrollbar thumb.
- **Pencil-light** (`{colors.pencil-light}`): the row rule, chart ruling and baselines, the interval whisker on an odds bar, a skeleton block, the dimmed lines when a seed row is hovered.
- **Highlight** (`{colors.highlight}`): the hovered seed row, the keyboard-active combobox option, inline code in the methodology.

### Named Rules
**The Hue Means Surface Rule.** A colour other than ink, pencil, win, loss, teal or rose appears only on an element whose value belongs to one surface, and the surface word or square is always beside it. Never as decoration.

**The Two Sides Rule.** In any two-player comparison player A is teal (`#0f6e78`) and player B is rose (`#a8285f`): names, chart lines, bar halves, tags, the board's rows, the side named in a line of commentary. Two hues no surface uses, each 5.5:1 on the bond, so a side and a court are never the same colour. Amended 2026-09-12 from ink and pencil, at the user's request for colour that tells the players apart.

**The Outcome Rule.** Win and loss colour appears only on a W/L mark, a rank delta and the champion mark, and always with a letter or sign, so removing colour loses nothing.

## Typography

**Display Font:** Martian Mono Variable (with Martian Mono, ui-monospace, SF Mono, Menlo, Consolas, monospace)
**Body Font:** the same
**Label/Mono Font:** the same

Self-hosted from `@fontsource-variable/martian-mono/wdth.css`, imported in `web/src/main.tsx`. One family for everything typed on the sheet; a second family would be a second world. The stack is `--font` in `tokens.css`.

**Character:** a typewriter-grade mono at fixed pixel sizes. It reads as a document produced by an office, not as code. Tabular figures are true by construction, and `font-variant-numeric: tabular-nums` is set on `body`, on every `table`, and on anything carrying `data-numeric` or `.numeric` so a fallback face keeps the columns.

### Hierarchy
The ramp is eight fixed steps, `--t-11` to `--t-32`, all in px. Nothing is fluid; the only size that changes with the viewport is the page title, which steps from 28px to 32px at 880px.

- **Display** (700, 28px on a phone and 32px from 880px, 1.15, -0.02em): the page title (`h1` on every route, the player's name). Max 24ch on the home headline.
- **Headline** (600, 20px, 1.25, -0.01em): the brand `Deucepoint` in the sheet head, the methodology's `h2` (each one above an ink rule), the head-to-head's name pair.
- **Title** (600, 16px, 1.3): a section heading, the methodology's `h3`, the two names over a win split.
- **Body** (400, 14px, 1.55): base text, the standfirst (1.6, pencil, max 62ch), the methodology's paragraphs (1.65 on a 72ch measure), stat-row labels, the search field.
- **Data** (400 or 500, 13px, 1.3): table cells, seed rows, split-bar numbers, buttons, filter cells, tabs from 880px, meta lines, empty-state reasons (1.6, max 60ch). A cell never wraps.
- **Caption** (400, 12px, 1.55, pencil): every caption under a table or chart, table heads (500), the footer, tabs on a phone, chart tags (600), the contents title on the methodology. Max 72ch, or 62ch on the home page.
- **Figure** (700, 28px, 1.15, -0.015em): the big numbers only - a surface Elo in its cell, a win-share percentage.
- **Small** (400, 11px, 1.3): chart axis figures, the `±` interval beside a probability, the match count under a surface Elo, the event line in a match log, a score's mark (`ret.`).

Weights in use: 400 for prose and captions, 500 for a label or control, 600 for a heading or the brand, 700 for a name, a figure, a total row, a W/L mark.

### Named Rules
**The One Family Rule.** Everything typed on the sheet is Martian Mono, including headings, buttons, and the methodology's prose. There is no display face and no second family.

**The Typed Score Rule.** Scores, spans and records are typed with hyphens the way the source writes them (`6-4 7-6(3)`, `1973-1983`, `335-83`). A set is one unbreakable word; a cell never wraps; a table wider than the phone scrolls inside its wrapper and its caption does not.

**The Sentence Case Rule.** Every heading, label, tab and button is sentence case. No uppercase labels, no letterspaced eyebrows, no kickers.

## Layout

The shell is a single column of max-width 1248px centred, padded 16px on a phone and 24px from 880px. The sheet head is one flex row: brand, tabs, then the `Player:` search field; on a phone the search takes the whole row under the brand and the tabs, and the tabs scroll sideways with their short labels. Main content is padded 24px above and 64px below; the footer sits under an ink rule with the methodology link and the data attribution on every page.

There is one breakpoint, 880px, declared as `--wide` in `tokens.css` and written literally in every media query (a custom property cannot be used there). Below it every page is one column, in source order. At and above it:

- Home: headline over the left 7 of 12, the tour toggle and `Elo leaders, as of <week>` over the right 5; then the seeding sheet as a `7fr / 40px / minmax(360px, 5fr)` grid, the 40px column being the gutter where the ruled steps join each line's end to its seed row.
- Player: figures in the narrow left column (5fr), serve and splits in the wide right (7fr), a 48px gutter; the match log runs full width under both, and columns marked wide appear.
- Methodology: an 18rem sticky contents column with a pencil rule on its right, a 48px gap, and the article on a 72ch measure.
- Tables: the head row becomes sticky, and cell side padding goes from 5px to 8px.

Spacing is the `--s-1` to `--s-8` scale (4, 8, 12, 16, 24, 32, 48, 64). Section rhythm: a section is padded 24px or 32px top and bottom and closed by an ink rule. A caption sits 8px under the element it explains. Rows are 9px or 10px tall in padding on a pencil rule, so a seed row lands at exactly 40px, the unit the chart steps are drawn to. Three controls carry their own padding off the scale: the button (8px 14px), the filter cell (4px 10px), the table head (6px 8px).

Density is that of a typed sheet: no whitespace panels, no card gutters, one column of ruled entries per region. Everything works at 360px; the mobile investment is the single column, the scrolling table wrapper, and the short tab labels.

## Elevation & Depth

Flat. There are no shadows and no tonal layering; the sheet is one surface and depth is conveyed by the weight of a rule and the darkness of the grey. An ink rule is a head or a total; a pencil-light rule is a row. Selection is a box drawn in one ink rule, not a lifted surface. The one element that sits above the sheet, the search combobox panel, is boxed in ink on bond with no shadow. The two `box-shadow` declarations in the artifact (`0 1px 0 var(--ink)` on a focused field) thicken the field's rule to 2px; they are a rule, not a shadow.

### Named Rules
**The Ruled Structure Rule.** Structure is a 1px rule: pencil-light between rows, ink at a section head, above a total, under a table head, and around anything selected. Nothing is a card and nothing casts a shadow.

## Shapes

Square. `border-radius: 0` on the button; every other box inherits none. The only radius in the artifact is 1px on the three small colour squares (the 8px surface square, the 8px square in a surface cell, the 13px rivalry square), enough to keep them from aliasing and too little to read as rounded. Borders are 1px solid in ink or pencil-light, and a leader between a name and its figure is a 1px dotted pencil-mid rule. Selection and emphasis are drawn by boxing a cell in one rule; in a surface's own hue when the cell is surface-scoped, in ink otherwise. The bracket stub in the seeding gutter is an L drawn in the line's own grey.

## Components

Every component lives in `web/src/components` as a `.tsx` with a `.module.css`, and every one renders in every state at `/_components` (`web/src/routes/Gallery.tsx`). All charts are inline SVG drawn by hand; there is no charting library, no icon set, no component library.

### Buttons
A typed label boxed in one rule. Pressing it is the box filling.
- **Shape:** square (`border-radius: 0`), a 1px ink border, transparent ground.
- **Default:** ink text at 13px/500, padded 8px 14px.
- **Hover:** fills ink, text turns bond, 120ms ease-out. **Active:** translates down 1px over 80ms.
- **Disabled:** border and label go pencil-mid; hover does nothing.
- **ButtonLink:** the same box when the action is navigation; it keeps its own border rather than the anchor's underline.
- The label says what happens: "Simulate this matchup", "See the full rankings", never "Go", never with an arrow.

### Inputs / Fields
The typed field: a prompt, the entry, one rule under both.
- **Style:** the label `Player` is 14px/500 in ink and gains its colon in CSS; the input is transparent, borderless, 14px, padded 5px 0; the pair share one ink rule underneath.
- **Focus:** the rule thickens to 2px (`box-shadow: 0 1px 0 var(--ink)` on `:focus-within`); the input's own outline is off because the field draws the focus.
- **Combobox panel:** absolutely positioned under the field, 4px down, boxed in ink on bond, max 320px tall and scrolling; each option 8px 12px on a pencil rule, the keyboard-active one on highlight; every row carries tour, country, career matches and best tier. Messages in the panel are 12px pencil.
- **Placeholder:** pencil at full opacity.
- The date field on rankings uses the same rule-under-entry treatment.

### Navigation
The sheet head, typed.
- **Brand:** `Deucepoint`, 20px/600, no underline.
- **Tabs:** pencil, 12px/500 on a phone (short labels, horizontally scrollable) and 13px from 880px (long labels); hover to ink in 120ms.
- **Active tab:** ink at 600, set in brackets `[Rankings]` the way a seed is. Inactive tabs keep a non-breaking space where each bracket would be so nothing moves.
- **Search:** in the chrome on every page; 320px wide from 880px, full width on a phone. The head closes with an ink rule.

### Filter cells (TourFilter, SurfaceToggle)
Typed cells in a row, 8px apart. Each is pencil 13px/500 padded 4px 10px with a transparent 1px border; hover to ink. The active one is boxed in one rule: ink for a tour (a tour is not a surface), the surface's own hue for a surface. Selection persists to the URL.

### Tables (StatTable)
A sheet table: head under an ink rule, rows under pencil rules, the total above an ink rule and in 700, and one more ink rule to close the sheet. Heads are 12px/500 pencil and sticky from 880px; cells 13px, padded 9px 8px, never wrap; first and last cells flush to the edges. Sort direction is a drawn 8px chevron in ink, not a hue. Columns the phone has no room for are not rendered below 880px. The caption sits under the table, 12px pencil on a 72ch measure, and explains any `n/r` it contains. The methodology's markdown tables are drawn to the same rules.

### Stat rows (StatRow)
One ruled line of the sheet: label at 14px left, the figure at 14px/700 on the right edge, 9px padding on a pencil rule; the last row closes on an ink rule.

### The absence system (AbsentCell, PartialAggregate, EmptyState)
Three distinct cases, unchanged from the handoff.
- **AbsentCell:** `n/r` in pencil, aligned like the numbers around it, titled "Not recorded". Never `0`, never blank, never a dash.
- **PartialAggregate:** a 12px pencil caption under the aggregate declaring its denominator ("covers the 41 of 68 matches that did").
- **EmptyState:** a note on the sheet, not a panel: an ink rule above, a 14px/600 heading naming what is absent, a 13px pencil reason on 60ch, then a button to something complete, 16px below. No icon, no illustration.

### Marks (WinLossMark, RankDelta, SurfaceDot, Score, Meta)
- **WinLossMark:** `W` or `L` at 13px/700 in win or loss.
- **RankDelta:** a signed integer at 500, win when up, loss when down, pencil at 0.
- **SurfaceDot:** an 8px square (1px radius) in the surface hue, 6px before its word, never alone.
- **Score:** each set an unbreakable word; the mark (`ret.`, `w/o`) at 11px pencil.
- **Meta:** typed fields two real spaces apart (`Spain  right-handed  23  turned pro 2018`), 13px pencil; empty parts drop out.

### Surface Elo cells (SurfaceEloStrip)
Typed cells in a row, 12px apart, each flex 128px to 200px wide and padded 8px 12px with a transparent 1px border. The label is 12px/500 with the 8px square, the figure 28px/700 in the surface's hue, the match count 11px pencil. The best surface is boxed in its hue: the one place a box carries meaning on the player page. The overall cell is ink with a pencil label.

### Charts (Sparkline, TrajectoryChart, TrajectoryPair, SplitBar, WinSplit, OddsBar, RivalryStrip)
Hand-rolled SVG over the sheet's ruling (pencil-light, 1px). Names sit at the ends of their lines, never in a legend; the two axis figures are 11px pencil.
- **TrajectoryChart:** 140px tall; the ink ramp in ranking order (ink 1.5px, pencil 1.5px, pencil-mid 1.25px), the field at pencil-light 1px.
- **TrajectoryPair:** 120px tall; A ink, B pencil, both 1.5px.
- **Sparkline:** one ink line at 1.25px with round joins.
- **SplitBar:** two 6px halves growing outward from the centre, ink left, pencil right, a 2px bond gap; numbers at the outer edges in 700, the label centred in 12px pencil; each on a pencil rule, the last on ink.
- **WinSplit:** one 10px bar divided ink against pencil with a 2px bond edge; the share at 28px/700. With `animate`, the figures count up from 50.0 over 700ms on an ease-out cubic while the fill eases with them, and the last frame writes the API's own figure.
- **Scorelines:** the chance of each set score, most likely first, A's sets written first: a 4ch score, a 6px bar scaled to the likeliest (ink for A's wins, pencil for B's), the percentage right-aligned in 700; rows on pencil rules between ink rules; the bars grow over 550ms, 300ms after the result lands.
- **OddsBar:** a 6px fill in the tournament's surface hue inside a 10px track, the 95% interval as a pencil-light whisker around the bar's end; the figure in 700 with its `±` at 11px pencil. Name, track and figure sit in one row from 880px and the track drops to its own line on a phone.
- **RivalryStrip:** 13px squares (1px radius) 3px apart; filled when player A won, outlined when B did, fill or stroke in the match's surface; the legend is a caption underneath.

### The seeding sheet (signature)
The home page's first viewport and the site's one piece of ambient motion. Left, the eight leaders' form lines over 24 months on the ruling, the top three in the ink ramp and the field in pencil-mid at 0.6 opacity; a dotted lead (`1 3`) carries a line that stopped early to the edge. A 40px gutter holds an L-shaped ruled step from each line's end to its row. Right, the seed rows: `[seed] Name (CTY) ····· best-surface Elo in its hue ····· Elo delta`, 40px tall on pencil rules, a dotted leader between name and figures, the surface figure (12px, the surface word and the raw series rating from `best_surface` on the rankings response) the one coloured cell on the row, the list opened and closed by ink rules. Hover on a row lights its line to ink and dims the others to pencil-light and their rows to 0.55 opacity; the row itself goes highlight. On a phone the two stack, the steps are not drawn, the surface figure steps aside as the rankings table's column does, and the top three lines carry `[1]` `[2]` `[3]` at their ends instead.

**Motion.** Under `prefers-reduced-motion: no-preference` only: lines draw in over 900ms on `cubic-bezier(0.16, 1, 0.3, 1)` via `stroke-dasharray` with `pathLength="1"`; the dotted leads fade in over 300ms from 700ms; the gutter unclips left to right over 420ms from 700ms; each seed row slides 6px in and fades over 480ms, delayed `120ms + index * 45ms`. After 1400ms the component drops its animate class and what remains is the finished sheet, so a resize or capture has nothing to trip over. Under reduced motion the sheet is static; `base.css` also collapses every animation and transition to 0.01ms. The TrajectoryChart on the rankings page draws in the same way (900ms named, 700ms field) and is otherwise still. Everything else on the site moves only as a 120ms to 160ms ease-out on colour, border or stroke.

### The simulated match (Scoreboard, Tracker, Playback)
One draw from the odds above, played out at the prototype's pace once the reader presses `Watch a simulated match`, never on its own. The section is at most 560px wide, a board a phone could hold. **Scoreboard:** a table between two ink rules, `table-layout: fixed`; a header row of set numbers (11px/500 pencil); one row per player with the name at 14px/700 (the leader ink, the trailer pencil), a column per finished set at 16px/700 (the set's winner ink, the loser pencil, the loser's tiebreak points as a 10px superscript on their 6), and the set in progress boxed in a 1px rule in the match's surface hue, its header number in that hue. An 8px square in the surface hue stands before whoever serves and pulses (1 to 0.3 opacity, 1.2s) only while the match plays. A broken player's row fades from highlight to transparent over 900ms; a moving figure pops from `scale(1.35)` over 350ms. Set columns are 6ch, 5ch and 14px figures below 880px. **Commentary:** one 13px pencil line with a 20px floor under the board, fading out 200ms and in 220ms between beats; its words follow the sample (`Sinner holds comfortably`, `Break point, Alcaraz`, `Alcaraz breaks`, `Tiebreak at 6-6`, `Sinner takes the tiebreak 7-5`, `Set 2 to Alcaraz`, `Game, set, match: Alcaraz wins 3-1`); a visually hidden polite live region carries set and match lines only. **Tracker:** `Points won` as a SplitBar whose fills ease 500ms, then `Break points won` (`2 of 5` against `1 of 3`) and `Service holds` on ruled lines, A in ink at the left, B in pencil at the right, the label pencil between; every figure counted from the points the sample played.

**Beats** (the prototype's): a routine hold 480ms, a hold after a break point 800ms, the break-point beat 850ms, a break 950ms, `Tiebreak at 6-6` 1000ms then its result 1100ms, a set 1300ms; the match line ends the run. The button reads `Playing…` and is disabled while it runs, then `Watch another`. `matchMedia` is asked in JS: under reduced motion the finished match renders at once.

### Skeleton
A line of the sheet not yet typed: a pencil-light block, 12px tall with 10px under it, matching the final layout; it pulses between 1 and 0.45 opacity over 1.6s only when motion is allowed.

## Do's and Don'ts

### Do:
- **Do** set every colour from the twelve tokens in `tokens.css`; a new hue needs a new surface, not a new element.
- **Do** put a surface hue only on a cell whose value is that surface's, and put the surface word or 8px square beside it.
- **Do** make player A ink and player B pencil in every comparison, and put each name at the end of its own line.
- **Do** use `--rule` (pencil-light) between rows and `--rule-ink` at a head, above a total, at a section break, and around anything selected.
- **Do** type `n/r` in pencil for a value nobody recorded, explain it in the caption under the table, and declare an aggregate's denominator beside it.
- **Do** keep every caption directly under the element it explains, 12px pencil, on a 72ch measure or less.
- **Do** keep tabular numerals on every element carrying data and never let a cell wrap; scroll the wrapper instead.
- **Do** write buttons as what happens, in sentence case, boxed in one ink rule.
- **Do** keep one column below 880px and prove every component at 360px.
- **Do** gate any motion on `prefers-reduced-motion: no-preference` (and ask `matchMedia` from any timer), keep the seeding sheet's draw-in the only ambient motion, and let anything else that moves over time answer a choice the reader made, finish on its own, and render finished under reduced motion.

### Don't:
- **Don't** add a card, a panel background, a shadow, a gradient, or a radius above 1px.
- **Don't** add an icon set or a glyph icon; the sort chevron and the colour squares are drawn, and every other mark is typed.
- **Don't** add a second type family, a display face, or a fluid size; the ramp is the eight fixed px steps.
- **Don't** set anything in uppercase, letterspace a label, or add an eyebrow or kicker above a heading.
- **Don't** use hue to tell two players apart, to decorate a heading, or as a status colour outside W/L marks and rank deltas.
- **Don't** render an absence as `0`, a blank, a dash or `N/A`, and don't hide a column because it is mostly `n/r`.
- **Don't** put a chart's names in a legend, or a chart's key in a hue.
- **Don't** add a second breakpoint; 880px is the one.
- **Don't** invent photography, logos or marks; none exist.
