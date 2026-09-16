# Mockups

The mobile layouts the first design system was written from, drawn in September 2026 in
the hairlines-on-cream world that the redesign of 12 September replaced. **The site no
longer looks like these.** What shipped is the draw sheet — one monospaced face, ruled
lines, no cards — and [`DESIGN.md`](../../../DESIGN.md) at the repository root records it,
with the reasoning in [ADR-0010](../../decisions/0010-the-draw-sheet.md). The images stay
because the absence system (`n/r`, the declared denominator, the empty state as a note)
and the screen inventory were drawn here first and carried over unchanged; for the look,
open the live site.

They are the primary design, not a degraded desktop, and
[`../design-system.md`](../design-system.md) wins wherever an image and the document
disagree — for the product rules; for the visuals, `DESIGN.md` wins over both.

Expected files, per the screen index in that document:

| File | Screen |
|---|---|
| `home.png` | Nav, hero trajectory chart, Elo leader strip, attribution footer |
| `player-profile.png` | Identity header, surface Elo strip, clutch vs tour average, recent results |
| `head-to-head.png` | Career score, surface filter, SplitBars, RivalryStrip, simulate button |
| `simulator.png` | Win split, stage-by-stage breakdown, draw-sim odds with CI whiskers |
| `rankings.png` | Elo against official rank, with the RankDelta column |
| `empty-state-borg.png` | The pre-1991 EmptyState pattern |
| `match-log-absence.png` | AbsentCell and PartialAggregate in one table |

This file exists so the directory survives a clone: git does not track an empty one.
