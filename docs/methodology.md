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
- **The simulator's inputs do not end where the serve statistics do**, but its anchor does.
  Point-win probabilities are derived from ratings rather than from per-player serve rates
  ([ADR-0007](decisions/0007-elo-derived-point-probability.md)), so any rated player can be
  simulated; the tour-and-surface average that anchors the derivation is still measured from
  the 3% of matches that recorded serve statistics.

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
- **Rows repeating a match already read** are collapsed rather than counted twice. The WTA
  qualifying files carry 2,700 byte-identical repeats across three seasons alone; a match is
  identified by its draw, its number and the pair who played it, so a repeat updates the
  match instead of inventing a second one.
- **Team events** (Davis Cup, Billie Jean King Cup) are flagged and excluded from rating
  calculations by default.
- **Walkovers** are excluded from ratings, because no tennis was played and rating one would
  move two ratings on no evidence. A retirement is rated: it was played, and it has a
  winner.

## The tier weights, and the evidence for them

A result at Futures level counts for less than a Grand Slam final. How much less is a
judgement, and [ADR-0004](decisions/0004-tier-taxonomy-and-elo-pool.md) picked numbers
before there was any data to check them against. `cmd/validate` is what checks them.

| Level | Weight |
|---|---|
| Grand Slam final | 1.20 |
| Grand Slam, other rounds | 1.10 |
| Tour Finals | 1.10 |
| Masters 1000 / WTA 1000 | 1.05 |
| Other tour-level | 1.00 |
| Davis Cup and other team events | 0.80 (excluded from ratings by default) |
| Challenger | 0.80 |
| Futures / ITF | 0.60 |
| Qualifying | multiply by 0.90 |

**What the evidence says.** Tour-level predictive accuracy is 69.9%, inside the 68-72% the
specification asks for, and calibration is within 1.6 percentage points at every tier. On
those two measures the weights are fine.

The third measure is less comfortable. A player's rating should carry across a promotion
from Challenger to tour without a step in it, and it does not: across 7,050 promotions,
players won 33,120 of their first tour-level matches against 31,323 expected. Promoted
players arrive underrated.

**Raising the lower tiers is not the fix, and the numbers say so.** Moving Challenger to
0.90 and Futures to 0.75 removes about a fifth of that surplus and makes Challenger
calibration worse; going further to 1.00 and 0.90 removes a little more and costs a little
more. The lever does not fit the problem.

The likeliest explanation is that promotion selects for players who are improving, whose
rating is an average over a period when they were genuinely worse. No fixed weight corrects
for someone being better than their own record. That is a hypothesis rather than a finding,
and it is stated here rather than quietly assumed, because the alternative is presenting a
number as settled when the check that would settle it says otherwise.

So the weights stay as ADR-0004 set them, and the reason is written down: the available
change trades a measurable calibration cost for a partial fix to something the weights do
not control. `make validate` reruns all of this, and `--weights` tries other numbers without
a rebuild.

## Tiers

`tier` is a competitive standard, deliberately distinct from `tournaments.level`, which
records event prestige. A Grand Slam qualifying draw and a Futures qualifying draw are both
"qualifying" and nothing alike. Qualifying is a boolean beside the tier, not a tier of its
own, because a Challenger qualifier is still Challenger standard. The reasoning, and the
Elo pool design that depends on it, is in
[ADR-0004](decisions/0004-tier-taxonomy-and-elo-pool.md).
