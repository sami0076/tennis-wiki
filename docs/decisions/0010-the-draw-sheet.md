# ADR-0010: The draw sheet, one monospaced face, and absence typed n/r

- **Status:** Accepted
- **Date:** 2026-09-12
- **Context:** the frontend redesign on `feat/redesign`; supersedes the visual half of
  `docs/design/design-system.md`

## Context

The first frontend was hairlines on paper: Archivo, a small token set, hue reserved for
surfaces, and an absence system that renders "not recorded" more often than it renders a
number. The rules were right and are kept. The look was the category's predictable
opposite, the broadsheet-on-cream that a stats site reaches for when it does not want to
be a dashboard, and it could have been guessed from the category alone.

The redesign asked a different question: what does this thing look like as an object in
its own audience's world? A tennis fan already reads a draw sheet, the typed page pinned
in the players' lounge with seeds in brackets, countries in parentheses, scores written
in after each round, and ruled lines stepping a name into the next round. It carries this
product's mechanism without translation: a seeding is a ranking, a draw replayed is the
simulator, a score is data, and the marks a sheet uses for a retirement or a walkover are
the marks a match log needs.

## Decision

**Every page is a sheet from the tournament office.** The design record is `DESIGN.md`
at the root, written from the built pages; this ADR records the three decisions a reader
of the code would otherwise have to guess.

**One monospaced family for everything typed.** Martian Mono, self-hosted from
`@fontsource-variable/martian-mono`, carries names, seeds, scores, figures, headings,
captions, buttons and the methodology's prose. A draw sheet is a typed document, so the
mono is the material rather than a costume for "technical"; and a mono makes tabular
figures true by construction, which the old system called its most load-bearing line of
CSS. The methodology page, the one long read, stays in the mono at 14px on a 72-character
measure; it is a typed document too, and a second family would be a second world.

**Absence is typed `n/r`.** The old em-dash read as a blank in a mono column and collided
with the rule that scores and spans are typed with hyphens, the way the source and the
sheet write them (`6-4 7-6(3)`, `1973-1983`, `335-83`). `n/r` is how a sheet abbreviates
a figure nobody recorded, it sorts and aligns like the numbers around it, and every table
that carries it explains it in the caption under the table. Nothing else about the
absence system changed: never a zero, never a blank, never hidden, and an aggregate
still declares its denominator.

**Colour only where a value is surface-scoped.** The surface hues were darkened so they
read as text on the bond at 4.5:1 (clay `#a84f1f`, hard `#2e6fa8`, grass `#4a7a3f`,
indoor `#6b5b95`). The active surface filter is a cell boxed in its hue; the best surface
on a player page is boxed in its hue. A two-player comparison was ink against pencil until
2026-09-12, when the user asked for colour that tells the players apart: player A is now teal
(`#0f6e78`) and player B rose (`#a8285f`), two hues no surface uses, so the surface rule holds.

## Consequences

- `web/src/styles/tokens.css` is the whole palette: bond, highlight, ink, pencil,
  pencil-mid, pencil-light, four surfaces, win, loss. Rules are `--rule` (pencil-light)
  between rows and `--rule-ink` for a section head, a table head and a total.
- The home page's first viewport is `SeedingSheet`: the leaders' form lines, the seeds
  beside them, and a ruled step from each line end to its row. It is the site's one piece
  of ambient motion and it is static under `prefers-reduced-motion`.
- The Figma file `Deucepoint redesign` holds the tokens, the atoms, and the Home and
  Player frames the world was approved on. The remaining screens were built in code
  because the Figma plan's tool-call limit stopped the frames; the code is the record.
- `docs/design/mockups/` are the old world and stay as history.
