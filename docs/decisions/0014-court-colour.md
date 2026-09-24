# ADR-0014: Court colour — cards, two player colours, and motion

- **Status:** Accepted
- **Date:** 2026-09-23
- **Context:** the frontend refresh on `feat/colour-redesign`; supersedes the visual half of
  [ADR-0010](0010-the-draw-sheet.md). Its absence rules (`n/r`, typed scores, marks) stand.

## Context

The draw sheet read correctly and looked quiet: one ink, three pencils, hue only for
surface, no cards, one animation on the whole site. Mockups for the home, player and
head-to-head pages asked for the same data with more presence: a colour per side of a
comparison, figures set large, panels, and pages that move when they arrive.

## Decision

**Keep the ground, add colour and cards.** The cream `--bond` and Martian Mono stay. White
cards with an 8px radius and a faint shadow replace ruled sections.

**Two players, two colours, everywhere.** Violet (`--player-a`) and magenta
(`--player-b`) mark the two sides of every comparison, each with a wash for tinted panels.
Hue is no longer reserved for surface; surfaces keep their inks and gain washes for badges.

**Lime is the ball.** One accent (`--lime`) for the brand mark, the primary search action,
the leader and "best" tags, and button hovers. Never text on white.

**Motion on arrival.** Cards rise in, figures count up, bars grow, lines draw in, form
pills pop, and a court illustration loops a rally on the home page. All of it waits for
the element to scroll into view and all of it is removed under `prefers-reduced-motion`.
Figures always settle on their final value, and screen readers only ever get that value.

## Consequences

- `DESIGN.md` is rewritten from the shipped pages; ADR-0010 remains the record of why the
  absence system and typed marks exist.
- A chart can now carry player identity by colour, so every coloured value still needs its
  name or number beside it: colour is never the only encoding.
