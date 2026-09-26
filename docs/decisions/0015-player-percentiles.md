# ADR-0015: Player percentiles and the head-to-head radar

- **Status:** Accepted
- **Date:** 2026-09-26
- **Context:** `GET /players/{slug}/percentiles`, drawn as a radar on the head-to-head page.

## Context

A radar puts eight figures on one shape, and the figures do not share a unit: points won
are percentages, surface strength is Elo, form is a change in Elo. Raw values on shared
spokes distort the shape, and forehand, backhand or movement ratings would need
shot-level tracking the data does not have.

## Decision

**Every spoke is a percentile of the player's tour, 0 to 100.** Ties count half. The
dashed ring at 50 is the tour median.

**The window is the player's last tour-level year:** the 364 days up to their last
finished main-draw match at tour level. An active player is measured on the last year and
a retired one on their final year, against the tour of that year.

**The population is everyone on the same tour with ten or more of those matches in the
window,** the leaderboard floor. A player under the floor is still ranked against the
population, is not counted in it, and the page says so.

**Eight spokes, all computed:**

| Spoke | Figure |
|---|---|
| Serve | service points won |
| Return | return points won |
| Clutch | mean of the percentiles of break points saved, break points converted, tiebreaks won and deciding sets won; at least two of the four |
| Hard, Clay, Grass | the surface Elo blended with overall as the simulator blends it (`rating.Blend`) |
| Form | overall Elo change over the last 182 days of the window |
| Big matches | win % against top-ten opponents or at a Slam; at least five |

Clutch averages percentiles rather than rates, because deciding sets spread widest and
would otherwise decide the spoke. Surfaces use the blend so that three grass matches
cannot produce a spike.

**A missing figure is not a zero.** The API returns null, and the chart leaves out that
vertex, draws a dotted chord across the gap, and the table says "no figure".

## Consequences

The population is aggregated per request (about 250 ms on the full database) and then
held by the response cache. Nothing new is stored.

Percentiles saturate at the top. Two players in the top five sit near 100 on most spokes,
and their shapes differ mainly on form and serve. That is true of their standing on the
tour, but it makes an elite rivalry the least interesting radar. Narrowing the population
(the top 50 by Elo, say) would spread them out, at the cost of meaning something different
for everyone else.
