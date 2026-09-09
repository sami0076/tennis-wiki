# ADR-0007: Derive the point-win probability from the rating, not from serve statistics

- **Status:** Accepted
- **Date:** 2026-09-09
- **Context:** [ADR-0004](0004-tier-taxonomy-and-elo-pool.md), issue #76

## Context

Everything in Phase 3 rests on one number: the probability that a player wins a point on
their own serve against a specific opponent. The chain in `simulator.png` — point, hold,
set, match — is closed form once that number exists, so where it comes from is the only
real modelling decision in the phase.

The obvious source is the serve statistics already ingested. Counted over the full
database:

| | Players |
|---|---|
| Have **any** match with serve statistics | 8,153 |
| Have **10+** such matches | 3,362 |
| Have **30+** such matches | 2,299 |
| **Are rated** — have an Elo in any series | **75,021** |
| Exist in the database | 125,719 |

A simulator fed only by serve statistics would work for about **2.7%** of the players on
this site: the ones every other tennis site already covers. The README leads with "every
player, not just the famous ones" and [ADR-0003](0003-full-depth-player-coverage.md)
committed to full-depth coverage as the project's differentiator. A headline feature that
excludes 97% of the players contradicts both.

Three options were on the table in #76: measured rates only, ratings only, or both with
measured preferred.

## Decision

**Derive the point-win probability from the Elo rating, for every pair. Do not use
per-player serve statistics as a simulator input.**

The chain is inverted rather than fed forward. Given two players:

1. Take the blended rating for the surface — the formula in the README, implemented in #77.
2. `rating.Expected` turns the rating difference into a match-win probability. That number
   already exists, is already validated (69.9% tour-level accuracy, calibration reported
   per tier by `cmd/validate`), and already covers every rated player.
3. Solve for the pair of serve probabilities whose closed-form match probability equals it.

Two unknowns and one equation, so the second degree of freedom is pinned by an anchor: the
two probabilities average to the tour-and-surface serve-point-win rate for the era. That
anchor is an aggregate, not a per-player figure, so it is available wherever serve
statistics exist at all — and it varies enough to be worth measuring rather than assuming:

| Tour, tier | Serve points won |
|---|---|
| ATP tour | 62.6% |
| ATP Challenger | 61.2% |
| WTA tour | **55.9%** |
| WTA Challenger | 54.8% |

| ATP tour, by surface | 1990s | 2000s | 2010s | 2020s |
|---|---|---|---|---|
| Grass | 63.4% | 65.0% | 65.6% | 65.4% |
| Hard | 62.0% | 63.2% | 63.3% | 63.8% |
| Clay | 59.4% | 60.5% | 61.4% | 61.3% |

The 6.7-point gap between the ATP and WTA tours is the reason the anchor cannot be one
constant. Issue #79 builds the table.

## Consequences

**Every rated player can be simulated** — 75,021 of them, 22 times the number the measured
path would have reached. A player with no rating still cannot be, and gets an explanation
rather than a 1500-versus-1500 coin flip.

**The match probability is the rating's, by construction.** This is the honest cost of the
decision and it has to be stated plainly, because it changes what the simulator can claim.
The chain is solved so that its match-level answer *equals* `rating.Expected`. So:

- The simulator does not improve on the Elo prediction at match level. It cannot. Any
  calibration check at match level is a check of the rating engine, which `cmd/validate`
  already performs.
- What the simulator adds is **shape, not accuracy**: the decomposition into per-point,
  per-game and per-set probabilities, which is what makes the amplification visible and
  what the draw simulator needs to play a bracket forward.
- #84 must therefore validate the rungs the chain introduces — set scores, the distribution
  of match lengths, draw outcomes — and must not report "the simulator matches Elo at match
  level" as a finding. It is an identity, not a result.

**The anchor is a real input and carries real uncertainty.** It is measured from the 3% of
matches that recorded serve statistics and applied to pairs from the other 97%. For a
Futures match in 1994 the anchor is an extrapolation, and the response says which anchor it
used so a reader can judge it.

**Serve statistics keep their existing jobs.** They still drive the profile's serve panel,
the head-to-head comparison and the clutch figures from #66. This decision removes them
from one input path, not from the site.

**The measured path is not foreclosed.** If it is ever revisited, it arrives as a second
model with its own calibration and its own row in the validation report, not as a silent
blend with this one. A blended figure would hide whichever half is worse.

## What the response must carry

The same rule the clutch baseline established: a number that will not name its source is a
number pretending to be a fact. A simulation response states the ratings it used and the
week they are as of, the anchor and the population it came from, and that the point
probabilities are derived rather than observed. A reader who wants the observed serve rates
can find them on the player page, where they are what they claim to be.
