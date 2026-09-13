---
version: 1
slug: "web-src-routes-home-tsx"
primary_target: "web/src/routes/Home.tsx"
related_targets: ["web/src/layout/Layout.tsx","web/src/routes/Player.tsx","web/src/routes/Players.tsx","web/src/routes/HeadToHead.tsx","web/src/routes/Rankings.tsx","web/src/routes/Simulator.tsx","web/src/routes/Methodology.tsx","web/src/routes/Gallery.tsx"]
---

# Surface brief: the site, entered through `/`

Scope: full visual redesign of every route (`/`, `/players`, `/players/:slug`, `/h2h`,
`/rankings`, `/simulator`, `/methodology`, `/_components`) and the chrome in
`web/src/layout/Layout.tsx`. Home is Persuade; player, players, h2h, rankings and simulator
are Operate; methodology is Read. One world owns all of them.

Audience: tennis fans and analysts first, arriving with a name or matchup, often on a
phone. Second, a hiring manager opening the link from a CV: the first viewport has to land
as design work. Job on `/`: see the state of the tour and get to a player in one move.

Proof and content: every number is queried; the coverage table; Wimbledon 2019 replayed
(Djokovic 40.1% ±1.0, and he won it). No photography exists and none may be invented.

Constraints that bind every page: hue means surface only; two players are ink and pencil,
never two hues; win/loss colour only on W/L marks and deltas; absence is never a zero and
never hidden; tabular numerals on all data; colour never the only encoding; 360px;
attribution footer on every page; sentence case, plain sentences; copy is preserved.

Decided at build: `/methodology` prose stays in the mono at 14px on a 72ch measure; a
methodology is a typed document too, and a second family would be a second world.

Review medium: Figma frames first (user's choice, 2026-09-11), then code. Code-led by
contract: no image generation in this session.

## Direction contract

THESIS: Every page is a sheet from the tournament office: the typed draw sheet pinned in
the players' lounge, with seeds in brackets, scores written in, and ruled lines that join
winners. It refuses the sports-stats dashboard (tiles, one accent, a big chart) and the
broadsheet-hairlines-on-cream it replaces.

OWN-WORLD: Bond paper, one ink, one pencil. One monospaced typewriter-grade family carries
everything typed: names, seeds `[1]`, countries `(ESP)`, scores `6-4 3-6 6-2`, marks
`ret.` `w/o` `Q`, buttons, headings. Ruled ink lines at 1px do the structure: a row rule
under every entry, an L-shaped bracket stub where a line steps to the next round, a double
rule above totals. Pencil grey for captions and for player B. Surface colour only in the
cell that is surface-scoped: a filled square before the surface word, the Elo figure in
that colour, the active surface filter as an ink-bordered cell in its hue. No cards, no
shadows, no radius, no icon set. With the content removed: a ruled sheet with bracket
stubs on white.

STORY: A fan reads it as a draw sheet and trusts it the way they trust one: the seeds,
the countries, the scores, the marks. They see the top eight seeded by Elo, the eight
form lines, and the last real draw with the model's odds written in the round columns and
the champion marked. They search a name and land on that player's sheet.

FIRST VIEWPORT: Sheet head as a typed header: `Deucepoint`, the nav as typed tabs, the
search as the sheet's `Player:` field. Below, the headline in the mono at display size,
two lines, the standfirst in pencil, spanning the left 7 of 12; the ATP | WTA toggle and
`Elo leaders, as of <week>` heading over the right 5. Then the sheet: left, the ruled
trajectory chart of the eight leaders over 24 months, ink ramp for the top three, the
field in pencil-mid at reduced weight; a 40px gutter in which a ruled step joins each
line's end to its seed's row; right, the seeding list `[1]` to `[8]`, each row `[seed]
Name (CTY) ····· best-surface Elo in its hue ····· Elo delta`, ruled; the surface figure
comes from `best_surface` on the rankings response and steps aside on a phone. Mobile: header, headline, toggle, chart with `[1]` `[2]`
`[3]` at the line ends, seeds, stacked. The primary action is the search field; the
second is `See the full rankings` as a typed button under the seeds.

SIGNATURE INTERACTION: the sheet's rules and bracket stubs draw in on load, left to right,
280ms staggered by row, and the trajectory lines draw with them; the only ambient motion on
the site, static under reduced motion. Hover on a seed row pulls its chart line to ink and
the others to pencil, the way a finger on a draw sheet follows one name.

FORM: The draw sheet, position 1 on the grounded list (Impeccable's pick, chosen over the
assigned Ceefax results page). Seed key 9ef26072.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review,
the verdict, DESIGN.md, and every shipping raster carrying its provenance.
