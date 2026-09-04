# ADR-0006: Accept the recent-coverage gap, and disclose it structurally

- **Status:** Accepted
- **Date:** 2026-09-04
- **Context:** [ADR-0002](0002-data-sources-after-upstream-removal.md), issue #15

## Context

After the upstream repositories were removed (ADR-0002), full-schema match data — the
49-column layout carrying serve statistics — reaches:

| Tour | Full schema through | Gap as of today |
|---|---|---|
| ATP | 2026-01-17 | ~7.5 months |
| WTA | 2024-12-31 | ~20 months |

Results-only data (13 columns: names, score, round, surface, ranks — no serve statistics
and no player IDs) exists for both tours through 2026.

This is not cosmetic. Serve and return percentages are the direct inputs to the simulator,
so a gap means current-season form windows are incomplete, ratings for active players stop
short of the present, and the WTA — the project's headline differentiator — is thinnest
exactly where a visitor looks first.

Issue #15 laid out four options: accept the gap, supplement with results-only data, scrape
the official sites, or wait for a full-schema mirror. A fifth emerged later: the Match
Charting Project, which is public, maintained, and charted through 2026-05-24, but covers
under 4% of matches and skews hard toward well-known players.

## Decision

**Accept the gap. Do not supplement, do not scrape. Disclose the true coverage everywhere
it matters, and make that disclosure derived rather than written down.**

Concretely:

1. `GET /api/v1/coverage` reports matches, date range, and the share carrying serve
   statistics, per tour and tier, **queried from the data**. Nothing hardcodes a "current
   through" date, so the claim cannot drift from the database. When the gap closes, the
   number changes on its own.
2. [`docs/methodology.md`](../methodology.md) states the actual coverage, the gap, and its
   consequences for ratings and simulation.
3. The README coverage table carries the same dates.

## Why not the alternatives

**Supplementing with results-only data (B)** buys currency for results, head-to-head and
Elo, but not for the simulator, whose inputs would still end in 2024. The cost is two
classes of match in one database, distinguishable only if every future aggregate query
remembers to filter — and the failure mode when one forgets is a plausible-looking wrong
number, which this project has already produced twice. It also needs name-based player
matching, with no IDs in that source.

**Scraping the official sites (C)** is a terms-of-service problem, an ongoing maintenance
burden, and precisely the behaviour that leads publishers to withdraw datasets. Given how
this project lost its original source, doing it would be hard to defend.

**Waiting for a mirror (D)** is not a decision, and cannot be relied on.

**The Match Charting Project (E)** is genuinely interesting: it is the most current source
available and its per-set serve and return columns are a superset of what the tour CSVs
carry. But it covers roughly 4% of matches and is concentrated on elite players, so
adopting it now would make coverage *less* uniform, not more. It is the natural first
supplement if this decision is revisited.

## Consequences

Currency is the thing visitors notice fastest, and this decision accepts being visibly
behind on it. That is the cost, and it is the right one: an honest gap is recoverable,
whereas silently mixing two data regimes is the kind of error that survives review and
surfaces as a wrong number a year later.

**Revisiting stays cheap.** `matches.source` already records row-level provenance, so
adopting option B later means adding a source to `configs/sources.json` and a filter to the
rate statistics — not a migration. That was the expensive part, and it is already done.

**Revisit when** the site is real enough to judge whether staleness actually undermines it.
That is a better basis for the call than a prediction made now.
