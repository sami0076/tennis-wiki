# Methodology

How the numbers on this site are produced, and — as much as anything — where they stop.

> Sections on the rating engine and the simulator arrive with Phases 2 and 3. What follows
> is coverage, which is settled.

## What the data is

Every match, player, and ranking here descends from [Jeff Sackmann's Tennis
Abstract](https://github.com/JeffSackmann) datasets, licensed CC BY-NC-SA 4.0. The original
repositories were withdrawn during this project; the current sources are mirrors and
derivatives, recorded in [ADR-0002](decisions/0002-data-sources-after-upstream-removal.md)
and configured in [`configs/sources.json`](../configs/sources.json). Full attribution is in
[`DATA_LICENSE.md`](../DATA_LICENSE.md).

Every match row records which source produced it, so any claim below can be checked against
the database rather than taken on trust.

## Coverage, and the gap

**Ask the site, not this page.** `GET /api/v1/coverage` reports matches, date range, and
the share carrying serve statistics per tour and tier, queried live. This page describes
the shape; the endpoint has the current numbers.

Full-schema data — the layout carrying serve statistics — runs out before the present:

| Tour | Full schema through |
|---|---|
| ATP | 2026-01-17 |
| WTA | 2024-12-31 |

[ADR-0006](decisions/0006-accept-and-disclose-the-coverage-gap.md) decided to accept that
gap rather than fill it with results-only data or by scraping. The reasoning is there in
full; the short version is that mixing two data regimes in one database produces
plausible-looking wrong numbers, and this project would rather be visibly behind than
quietly wrong.

What the gap means in practice:

- **Recent form is incomplete.** A "last 12 months" window covers less than it says for the
  WTA, and the site should not pretend otherwise.
- **Ratings for active players stop short of the present.** A "current top eight" is
  current as of the coverage date, not as of today.
- **The simulator's inputs end where the serve statistics do.**

## Where statistics do not exist

Absence is not zero, and the API keeps three kinds of absence apart rather than collapsing
them into one null. `serve.availability` on a player profile is one of:

| Value | Meaning |
|---|---|
| `recorded` | every eligible match carried statistics |
| `partial` | some did, some did not |
| `never_recorded_for_tier` | Futures and ITF, where nothing has ever been recorded in any year |
| `never_recorded_in_era` | before 1991 anywhere; before roughly 2010 at Challenger level |
| `not_recorded` | absent, reason unknown — the honest fallback |

When `availability` is not `recorded` or `partial`, `rates` is `null` rather than a set of
zeroes. Within `rates`, an individual rate with no denominator is also `null`: a player who
never faced a break point has not saved 0% of them.

This matters more at full depth than it sounds. Futures and ITF are the majority of the
1.6 million matches, and no Futures match in any year has ever recorded a serve statistic.
For most of the 115,000 players on this site, "we do not have this" is the accurate answer,
and the interesting engineering problem is saying *why* rather than showing a wall of
zeroes.

## What is excluded from statistics

- **Retirements and walkovers** count in win/loss records and are reported separately in
  `incomplete_matches`, but are excluded from every rate. A match abandoned at 2-1 in the
  first set is a real result and a meaningless serve sample.
- **Rows with no `serve_points`** are excluded from rate denominators. They are never
  treated as zero: a zeroed ace count for a 1970s match is wrong but plausible-looking,
  which is the worst failure mode available.
- **Stat lines that contradict themselves** are dropped at ingest, keeping the match. More
  first serves in than points served, more won than made, more second serves won than were
  played, more break points saved than faced: each is arithmetically impossible, so the row
  is corrupt rather than surprising. `cmd/dataqual` counts any that predate the check, and
  `ingest --stage prune` clears them.
- **Team events** (Davis Cup, Billie Jean King Cup) are flagged and excluded from rating
  calculations by default.

## Tiers

`tier` is a competitive standard, deliberately distinct from `tournaments.level`, which
records event prestige. A Grand Slam qualifying draw and a Futures qualifying draw are both
"qualifying" and nothing alike. Qualifying is a boolean beside the tier, not a tier of its
own, because a Challenger qualifier is still Challenger standard. The reasoning, and the
Elo pool design that depends on it, is in
[ADR-0004](decisions/0004-tier-taxonomy-and-elo-pool.md).
