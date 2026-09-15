# ADR-0011: The Match Charting Project attaches to matches it can name, and feeds nothing else

- **Status:** Accepted
- **Date:** 2026-09-15
- **Context:** issue #103, the revisit [ADR-0006](0006-accept-and-disclose-the-coverage-gap.md)
  asked for; amends ADR-0006

## Context

ADR-0006 named the Match Charting Project the natural first supplement and deferred it
until "the site is real enough to judge whether staleness actually undermines it". The
site has been live since 14 September (#101). This is the revisit, and the question is
not whether to ingest the MCP but what each layer of the site may take from it.

Everything below was counted from the files as they stood on 15 September 2026
(`charting-{m,w}-matches.csv`, `charting-{m,w}-stats-Overview.csv`), not quoted from
`DATA_LICENSE.md`. Where the two agree, that is now a checked fact rather than a copied
one.

### What the files hold

| | ATP | WTA |
|---|---|---|
| Charted matches | 7,566 | 4,080 |
| Distinct player-name strings | 1,003 | 732 |
| Players appearing exactly once | 395 | 272 |
| Matches with a `Total` stats row | 7,545 | 4,070 |
| Charted since 1 September 2024 | 924 | 849 |
| Charted in 2026 | 185 | 213 |
| Last charted match | 2026-05-21 | 2026-05-24 |
| Share from the 2020s | 44% | 59% |
| Share from before 2000 | 13% | 4% |
| Grand Slam share | 28% | 33% |
| Rows missing the player-name columns | 1 | 18 |
| Rows two columns short | 2 | 2 |

The 19 rows with no player names are team-event ties (Davis Cup, Billie Jean King Cup)
where the charter left the name fields out and every later column shifted left; two more
rows carry an umpire's name in `Best of`. They are 0.2% of the files and are noted because
a reader that trusts column position will silently mis-date them.

The names are internally consistent: across 1,735 distinct strings there is not one pair
that differs only by accent, case or punctuation. Volunteers chart against a shared
player list, and it shows.

### The premise that no longer holds

#103 and ADR-0006 both call the MCP "the most current source available — charted
through 2026-05-24, ahead of every other source". That was true when they were written.
Since #120 the Tennismylife site supplies full-schema tour matches for both tours to late
August 2026; the MCP's last charted match is three months older. **Currency is no longer
a reason to ingest it.** What the MCP has that nothing else public has is per-set serve
and return figures, winners and unforced errors by wing, and shot-by-shot data — for the
matches it covers.

### Identity, measured

The MCP carries names, not ids. The 1,732 names from well-formed rows were joined to the
live `players` table by exact case-insensitive full name within the tour:

| | ATP | WTA |
|---|---|---|
| Names in the MCP | 1,002 | 730 |
| Exactly one player row | 884 (88%) | 635 (87%) |
| No player row | 10 (1%) | 46 (6%) |
| More than one player row | 108 (11%) | 49 (7%) |

The misses are name forms — `Alison Riske Amritraj`, `Storm Hunter`, `Edouard Roger
Vasselin` — and a surname-plus-initial fallback recovers only 10 of the 56, so a name
lookup is not going to close that gap on its own.

The multiplicity is the finding. Of the 25 most-duplicated ATP names, 23 are **one person
held twice in our own table**: a Sackmann numeric id with a birth date beside an
alphanumeric id without one (`Carlos Alcaraz` is `207989` and `A0E2`). Those are the
reconcile stage working as designed — a name-and-country match with no date of birth
scores 0.60, under the 0.90 auto-link line, and waits in `identity_reviews` for a human —
on the alphanumeric ids the live source introduced in #120. The queue has not been
processed. Two of the 25 are different people (`Chris Lewis`, NZL 1957 and GBR 1982).

So a player-level name join against 125,719 rows has three failure modes at once: a
name form the MCP spells differently, two people with one name, and one person we hold
twice. The last is ours to fix regardless of this decision, and is filed separately.

## Decision

**The MCP is a layer of annotation on matches the database already has. It creates no
matches, no players, no ratings, and no coverage claim.** Per layer:

| Layer | May the MCP feed it? | |
|---|---|---|
| Ratings | **No.** | 11,646 matches against 1.6 million rated ones, concentrated on players whose ratings are already the best-supported in the database. It would add noise where the ratings are strongest and nothing where they are weakest. |
| Match records | **No new rows.** | A charted match attaches to an existing `matches` row or is not stored. `matches.source` is untouched; there is no third regime of match. |
| Player records | **No.** | No player is created from a charted name. A name that does not resolve leaves its match unattached. |
| Statistics | **Yes, beside the existing ones, never in their place.** | Per-set and per-match figures from `stats-Overview` go in their own table keyed by our `match_id`, carrying the MCP `match_id` as provenance. The serve columns on `match_players` are not filled, corrected or overridden from it. The site may show a charted match's figures on the pages that have one (#105), labelled as charted. |
| Simulation inputs | **No.** | The simulator reads `match_players`. Charted matches are 0.7% of the tour-level population and skewed toward finals and famous players; feeding them in would make the model's inputs less uniform, which is the objection ADR-0006 raised to the source and it still holds. |
| Coverage claims | **No.** | `/api/v1/coverage` does not read the charted table. Charted coverage, if it is ever shown, is its own line with its own numerator and denominator, and it never moves a "through" date. |

**Identity is resolved at the match, not the player.** An MCP row names the tour, the
date, the tournament, the round and two players. That is enough to find one row in
`matches` — a surname is nearly unique inside one edition of one event — and the row
found supplies both player ids from data the site already trusts. There is no name
lookup against the players table at all. A charted match that finds no `matches` row, or
more than one, is not attached; it is counted, and `cmd/dataqual` reports the count, so
the shortfall is a number rather than a guess. Exhibition matches, 1960s matches the
sources do not hold, and the malformed 23 all land in that count.

**Corrections flow one way.** Where a charted match and our row disagree on a total —
aces, serve points — the difference is reported by `dataqual` and neither side is
edited. The MCP is charted by hand, one match per volunteer, and the tour figures are
official; both are wrong sometimes, and a site that quietly picks one has produced a
wrong number twice already.

## Why not the alternatives

**Rule it out.** The recorded no #103 asked for as a fallback. It would be defensible —
the currency argument is gone and the coverage is 0.7% — but the per-set figures and the
shot data are real, nothing else public has them, and attaching them to matches we
already hold costs no identity risk and no second match regime. The reasons ADR-0006 gave
for deferring were about what the source would do to *uniformity*; as annotation it does
nothing to it.

**Use it to fill serve statistics where the tour files have none.** The tempting one:
Futures and ITF have no serve figures in any year, and some charted matches are from
those tiers. But the population would then be "matches somebody chose to chart", which
is not a sample of anything, and the simulator would carry two regimes in one column —
exactly the mixing ADR-0006 refused.

**Name-match players, with the overrides file for the residue.** This is how the live
source's ids are handled, so the machinery exists. Against it: 12% of the names are
ambiguous or missing today, the residue is hundreds of hand decisions, and every one is a
guess about a person when the match row already answers the question. Match-level
resolution makes the player-level problem disappear rather than manage it.

## Consequences

- #104 ingests `stats-Overview` into a table of its own, resolved through `matches`, with
  a reader that reads by header and rejects a row whose `Date` is not eight digits. The
  point-by-point files are out of scope until a page needs them.
- #105 decides what a charted match shows. Whatever it is, it is labelled charted and
  carries the MCP attribution; the license notice in `DATA_LICENSE.md` already covers it.
- `DATA_LICENSE.md`'s description of the MCP as the most current source is corrected in
  the same change as this ADR.
- The unprocessed `identity_reviews` queue from #120 is a live defect — the most-searched
  active players exist twice — and gets its own issue. This ADR does not depend on it,
  because it never joins on a player name.
- ADR-0006's finding stands: no results-only supplement, no scraping, the coverage claim
  stays derived. Its list of sources gains a layer that is not a source of matches.
