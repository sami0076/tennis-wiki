# ADR-0013: No flags; the sheet writes the code

- **Status:** Accepted
- **Date:** 2026-09-20
- **Context:** issue #129, in Phase 5 (#131); ADR-0010 chose the draw sheet, PRODUCT.md
  forbids fabricated imagery

## Context

The site carries an IOC code for nearly every player and renders it as three letters in
parentheses, `(ESP)`, on the seeding sheet, the search results, the leaderboards and the
draw sheet's entrants column: 245 codes across 125,756 players, 1,532 of them without one.
A flag is the one image the site could legitimately show, since no photographs exist and
none may be fabricated. Every tennis site shows them, a visitor expects them, and on a table
of forty names a flag is the only thing that makes the column scannable at a glance. The
question is whether to add them, and it deserved an answer on the record either way.

## Decision

**No.** The site writes the code, in pencil, in parentheses, where the sheet writes it.

Three reasons, in the order they weigh.

**The object does not have them.** ADR-0010 chose the typed sheet from the tournament
office as the site's world, deliberately and against the category's defaults, and a typed
sheet writes `(ESP)`. The world was chosen for carrying this product's mechanism without
translation; a raster of colour in every name cell is a translation. It also breaks the one
rule the palette has: hue is spent on surface and on the two sides of a comparison, and
nothing else. A flag column puts twenty hues that mean nationality next to the four that
mean surface, on the same row, and the reader has to learn which colours carry information
and which are decoration. The seeding sheet's one coloured cell per row is the best-surface
figure; it would stop being the one.

**A code is the more honest claim.** The IOC code is what the source recorded, and it is
recorded as of the player's career, not as of today. 145 players are `URS`, 120 `TCH`, 304
`YUG`, 180 `FRG`, 29 `GDR`, 30 `SCG`: nations that no longer exist and have no current
flag. Drawing a neighbour's flag for them would be an invention, and drawing none would be
an absence rendered as a gap, which this site does not do. `UNK` is 168 players whose
nationality the source did not know. The code carries all of this exactly; a flag would
carry it wrongly or not at all.

**The mechanics are a dependency for nothing.** Unicode flag glyphs do not render on
Windows. The alternative is a vendored sprite of some 250 SVGs (Twemoji, CC-BY 4.0) under
a CSP that admits no external image host, kept in step with a code list that includes
countries that no longer exist. That is a maintained asset for a feature the world does
not want.

## What was considered

- **Flags in the compact rows only** (search results, leaderboards), where scanning
  matters most and the sheet's vocabulary is thinnest. Rejected because it makes the site
  inconsistent with itself: the same player would be a flag in one list and a code in the
  next, and the leaderboards are ruled tables in the same world as everything else.
- **Flags on hover or as a title.** No harm, and no point: a reader who wants to know
  what `TCH` stood for wants a word, not a picture.
- **A country name in full**, `(Spain)`, as the compromise. Rejected for width: the
  entrants column of a 128 draw is already the widest thing on the site, and `(Czechoslovakia)`
  is a third again. The code is the sheet's own abbreviation and needs no explaining to
  the audience that reads draw sheets.

## Consequences

- Nothing changes. Every place that writes the code keeps writing it, in pencil, in
  parentheses, after the name.
- The name cell in `/_components` shows the code and nothing else; there is no flag
  variant to keep in the gallery.
- If the decision is revisited, the historical codes are the first thing the reopened
  issue has to answer, and "a code, not a flag" stays the answer for them regardless.
