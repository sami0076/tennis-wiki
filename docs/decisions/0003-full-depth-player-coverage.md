# ADR-0003: Cover every player at every tier, not just tour level

- **Status:** Accepted
- **Date:** 2026-09-02
- **Amends:** build specification §1 (non-goals), §2 (Phase 1 scope), §7 (rating engine)

## Context

The build specification scopes v1 to **tour-level matches only**. The README drafted from
it listed "Challengers and Futures" under *deliberately not built*, on the reasoning that a
tour-only pool keeps the rating population coherent.

That reasoning is defensible but it makes the product's headline claim narrower than it
sounds. "Deep per-player statistics" over a tour-only database means deep statistics for
roughly the top few hundred players of each era and **nothing at all** for everyone else —
including every player on the way up, every career journeyman, and every player whose
career peaked at Challenger level. A user searching for a player ranked 400 gets an empty
page.

The project owner's direction is explicit: cover as many players, in as much depth, as the
data allows.

## What the data actually supports

Measured against real files, not assumed:

| Tier | Matches (est.) | Serve statistics available |
|---|---|---|
| ATP tour | 194,996 (exact, 1968–2024) | 89% (2005) → 99% (2022) |
| ATP qualifying + Challenger | ~223,000 (exact, 1978–2024) | **0% (2005) → 84% (2015) → 99.7% (2022)** |
| ATP Futures | ~447,000 (1991–2021) | **0% in every year sampled** |
| WTA tour | ~250,000 (1923–2024) | 19% (2005) → 73% (2015) → 89% (2022) |
| WTA qualifying + ITF | ~488,000 (1924–2021) | ~0% (13% by 2022) |
| ATP doubles | ~26,000 (2000–2020) | partial |
| **Total** | **~1.63 million matches** | |

Player tables hold **55,487 ATP and 59,675 WTA** players — 115,162 in total, against the
few thousand who ever appear in a tour-level draw.

Two findings shape everything below:

1. **Challenger-level data became statistically rich around 2010.** From roughly 2015 the
   ATP Challenger tour carries serve statistics at near tour-level completeness. This is
   the single highest-value tier to add: hundreds of thousands of matches, full statistics,
   and it covers exactly the players a tour-only database misses.
2. **Futures and ITF have no serve statistics at all — ever.** ~935,000 matches of results,
   scores, and rankings, with no point-level data. They support win/loss records,
   head-to-head, and Elo, but they cannot feed the simulator.

WTA data is also materially poorer than ATP historically (19% serve-stat coverage in 2005
against ATP's 89%). The "WTA parity" claim has to mean *parity of treatment*, not parity of
available data, and the site must say so rather than let the gap read as neglect.

## Decision

**Ingest every tier, both tours, and make data availability an explicit dimension of the
model rather than an accident of which rows happen to be populated.**

1. **Tier is first-class.** `tournaments` carries a `tier` (`tour`, `challenger`,
   `futures`, `itf`, `qualifying`) alongside the existing `level`. Every query, aggregate,
   and UI surface can filter on it. Default views are tour-level so the site does not
   drown a casual visitor in Futures results; the depth is there when asked for.
2. **One Elo pool, tier-weighted.** A single global rating pool covering all tiers, with
   the §7.2 importance weight extended downward. Separate per-tier pools were rejected:
   they make a rising player's rating discontinuous at promotion, which is precisely the
   thing this data is uniquely able to show. See "Consequences" for what this does to the
   §7.5 validation criteria.
3. **Statistical depth degrades by tier, and the model says so.** A `has_detailed_stats`
   determination per match, derived at ingest from whether `serve_points` is populated —
   never inferred later from a zero. Rate statistics and the simulator consume only matches
   where it is true.
4. **Every player gets a page.** A player with eleven career Futures matches gets a real
   page showing those eleven matches, their Elo, and an honest explanation of which
   statistics do not exist for them. This is the `EmptyState` component from §10.3 becoming
   a primary surface rather than an edge case.
5. **Doubles deferred, not discarded.** ~26,000 doubles matches exist. They need a
   different match model (four players per match) and a separate rating treatment. Out of
   scope for Phase 1; the schema should not actively preclude it.

## Consequences

### The §7.5 validation criteria have to be restated

They were written for a tour-level pool and do not survive contact with a 1.6M-match,
all-tier pool unchanged:

- **Mean reversion (1500 ± 30)** now describes a much larger and much weaker population.
  Tour-level players will sit well above 1500 by construction, and that is correct. The
  check applies to the *whole pool*; a separate tour-level mean is reported for reference.
- **Predictive accuracy (68–72%)** is a tour-level figure. It must be reported **per tier**.
  Expect lower accuracy at Futures — more volatility, less reliable form, more retirements.
  A single blended number would hide exactly the thing worth knowing.
- **Calibration** buckets should be computed per tier for the same reason.
- **Face validity** gains a new and useful check: a player's rating trajectory should be
  continuous across a promotion from Challenger to tour. A discontinuity there means the
  tier weighting is wrong.

### Scale

~1.63M matches and ~3.3M `match_players` rows. Comfortable for PostgreSQL, but it makes
several things that were optional now mandatory: genuinely streaming ingest, considered
index design, and pagination everywhere. Full-ingest and rating-recompute runtimes need
measuring rather than assuming, since §7.6 requires recomputing ratings from scratch on
every ingest.

### Identity reconciliation gets much harder

115,000 players instead of a few thousand, and the lower tiers are exactly where name data
is worst — transliteration variants, inconsistent diacritics, junior-to-pro name changes,
and genuine duplicate records in the source. Issue #7 grows accordingly. The
conservative rule stands and matters more: **a wrong automatic merge is worse than an
unmerged pair, because it is invisible.**

### The "deliberately not built" section changes

Challengers, Futures, and ITF move out of it. What remains deliberately excluded: live
scores, betting products, user accounts, mobile apps, and — for now — doubles.
