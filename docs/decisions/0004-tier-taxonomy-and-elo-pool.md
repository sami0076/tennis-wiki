# ADR-0004: Tier taxonomy and a single tier-weighted Elo pool

- **Status:** Accepted
- **Date:** 2026-09-02
- **Amends:** build specification §7.2 and §7.5

## Context

[ADR-0003](0003-full-depth-player-coverage.md) committed the project to every tier, taking
the database from ~445,000 tour-level matches to ~1.63 million. The rating engine in spec
§7 was written for a tour-only pool and does not survive that change unmodified.

Two problems to settle before the schema is written, because both determine columns that
every downstream query reads.

**Tier is not the same thing as level.** The spec's `tournaments.level` (`G, M, A, D, F, C,
S`) conflates event prestige with competitive standard. A Grand Slam qualifying draw and a
Futures qualifying draw are both "qualifying" and nothing alike. The source files bundle
qualifying with the tier below it — `atp_matches_qual_chall`, `wta_matches_qual_itf` — so
the distinction cannot be recovered from the filename either.

**One pool or several?** Rating players from four tiers on one scale, or keeping separate
pools per tier.

## Decision

### Tier taxonomy

`tier` becomes a first-class column on `tournaments`, distinct from `level`:

| Tier | Contents |
|---|---|
| `tour` | ATP / WTA main draw |
| `challenger` | ATP Challenger main draw |
| `futures` | ITF Men's / Futures |
| `itf` | ITF Women's |

Qualifying is a **boolean alongside tier**, not a tier of its own — a Challenger qualifier
is still Challenger standard. It must be derived during parsing, since the source files
bundle it with the main draw.

### One pool, tier-weighted

A single global rating pool spanning every tier. Separate per-tier pools were rejected
because a player's rating would be discontinuous at promotion, and a rising player's
trajectory across tiers is precisely what this dataset is uniquely able to show.

The §7.2 importance weight extends downward:

| Level | Weight |
|---|---|
| Grand Slam final | 1.20 |
| Grand Slam, other rounds | 1.10 |
| Tour Finals | 1.10 |
| Masters 1000 / WTA 1000 | 1.05 |
| Other tour-level | 1.00 |
| Davis Cup / team events | 0.80 |
| **Challenger** | **0.80** |
| **Futures / ITF** | **0.60** |
| **Qualifying** | **×0.90** (multiplier) |

The lower-tier numbers are a starting point, not a result. Weight them too low and an
improving player cannot climb fast enough, so their rating lags reality on promotion.
Weight them too high and Futures noise leaks into tour-level ratings. Which way we have
erred is measurable, which is the point of the next section.

### Restated §7.5 validation

The spec's criteria assume a tour-only pool:

- **Mean reversion** applies to the whole pool. Tour players sitting well above 1500 is
  correct — the pool is now much larger and much weaker. Report the tour-level mean
  separately for reference.
- **Predictive accuracy** reported per tier. The 68–72% target is tour-level; expect lower
  at Futures.
- **Calibration** bucketed per tier.
- **New: promotion continuity.** A player's trajectory should be continuous across a
  Challenger-to-tour promotion. A systematic jump or drop at that boundary means the tier
  weights are wrong. This is the most sensitive test available for them.

## A correction to §7.2

The spec states that `K(n) = 250 / (n + 5)^0.4` yields "K ≈ 130 for a debutant and K ≈ 25
for a veteran with 500 matches". The first figure is right (131.33). **The second is not:
K(500) = 20.73.** K reaches 25 at roughly 311 matches.

Per spec §0.2, where an exact formula is given it is implemented as given. The formula
stands; the prose gloss is wrong and is not reproduced anywhere in the code or on the
methodology page.

## Consequences

- `tournaments.tier` and an `is_qualifying` flag are required in #3, and #6 must derive
  both during parsing rather than inferring them from filenames.
- Weights are configuration, not constants, so the promotion-continuity check can be run
  against alternatives without a rebuild.
- This ADR cannot be fully closed until there is data. The taxonomy is settled now; the
  weight values stay provisional until #20 and the Phase 2 validation run confirm them.
