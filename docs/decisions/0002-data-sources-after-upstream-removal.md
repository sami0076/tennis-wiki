# ADR-0002: Ingest from license-compliant mirrors after the upstream repositories were removed

- **Status:** Accepted
- **Date:** 2026-09-02
- **Supersedes:** build specification §3.1 and §3.2

## Context

The build specification names `JeffSackmann/tennis_atp` and `JeffSackmann/tennis_wta` as
the primary data sources for the entire project. Phase 1 cannot start without them.

**Both return HTTP 404.** Verified on 2 September 2026 by three independent methods:
authenticated GitHub API, unauthenticated GitHub API, and direct HTTP request. The account
reports `public_repos: 1`; the only surviving repository is `tennis_MatchChartingProject`.
No public statement explaining the removal was found on the author's blog or elsewhere.

That surviving repository's README warns that the author is "serious about the license"
and had considered ceasing updates because of violations. The removal of the match
repositories is plausibly the follow-through on that warning. This shapes the decision
below: the constraint is not only legal, it is a matter of not being the reason the
remaining data disappears too.

Survey of what is actually available. **All figures below were measured against the real
files, not assumed.** See [ADR-0003](0003-full-depth-player-coverage.md) for the decision to
cover every tier, which is what makes the lower rows matter.

| Layer | Source | Tours | Tiers | Coverage | Notes |
|---|---|---|---|---|---|
| **A** | Complete Sackmann snapshot | ATP + WTA | tour, qual+Challenger, Futures, qual+ITF, doubles, amateur | → **2022-01-10** | The **only** located source for Futures, WTA ITF, and doubles. WTA goes back to **1923**, not 1968 |
| **B** | Restructured ATP mirror | ATP | tour, qual+Challenger | → 2024 | Fresher than A for the tiers it carries |
| **C** | Vendored WTA snapshots | WTA | tour | → 2024 | Several candidates, none authoritative |
| **D** | `Tennismylife/TML-Database` | ATP | tour | 2025 → 2026-01-17 | Official ATP alphanumeric player IDs; extra `indoor` column |
| **E** | `JeffSackmann/tennis_MatchChartingProject` | ATP + WTA | charted matches only | → **2026-05-24** | **Still public and actively maintained.** The most current source available |

Layer A's headline files stop at 2022-01-10 — the 2022 files are partial, and 2021 is the
last complete season. Layers B, C and D are fresher but carry only the upper tiers, so
lower-tier data is complete through 2021 and absent after it.

## Decision

1. **Ingest from license-compliant mirrors** of the Sackmann data for 1968–2024, both
   tours. CC BY-NC-SA 4.0 permits redistribution, so mirrors are lawful sources; we
   attribute the originator, not the mirror.
2. **Make the source layer pluggable.** No mirror is authoritative and any of them may
   vanish exactly as the originals did. Ingest sources are declared in configuration —
   name, base URL, path template, schema profile — not hardcoded. Swapping a mirror must
   be a config change, not a code change.
3. **Vendor a pinned snapshot.** Once a full ingest succeeds, archive the raw CSVs with
   recorded checksums. The project must not be one repository deletion away from being
   unbuildable. This snapshot is a build input, not a public redistribution.
4. **Use TML-Database for ATP 2025–26**, with a schema profile handling its differing
   column order, its extra `indoor` column, and — the expensive part — its alphanumeric
   ATP player IDs, which require reconciliation against Sackmann numeric IDs by name plus
   date of birth.
5. **Layer sources by freshness, resolve by precedence.** For any (tour, tier, season)
   the registry picks the freshest source that carries it. Layer A supplies the depth,
   B/C/D supply currency for the upper tiers, E supplies shot-level detail. Provenance is
   recorded per row so any source can be re-evaluated without a full re-derivation.
6. **Ship the coverage gaps as visible product,** not as silent absence. The ATP gap after
   17 January 2026 and the WTA gap from 2025 onward are disclosed on `/methodology` and
   surfaced in the UI wherever a date range would otherwise mislead.

## Alternatives considered

**Use a results-only source for the recent seasons.** It covers the gap for match results,
Elo, and head-to-head, but has no serve statistics — which are the inputs to the entire
simulation engine (§8.2). Adopting it as primary would mean the simulator silently
degrades for current players, which is the worst failure mode this project has. Rejected
as a primary source; retained only as an explicitly-labelled results-only supplement.

**Scrape the ATP and WTA websites for the gap.** Rejected for v1. It is a terms-of-service
problem, an ongoing maintenance burden, and it is precisely the behaviour that makes data
publishers withdraw datasets.

**Wait for the repositories to return.** Rejected. They may not, and the project is not
blocked — 1968–2024 is 57 years of full-schema data for both tours, which is more than
enough to build and validate every phase.

## Consequences

- Phase 1 proceeds on schedule. The rating engine, its validation checks, and the
  simulator all have sufficient data.
- Rating validation (§7.5) uses "the most recent two seasons" as its held-out set. With
  full-schema data ending in 2024, that means **2023–2024**, not 2025–2026. `cmd/validate`
  takes the holdout window as a parameter rather than assuming "now".
- Player identity reconciliation across sources becomes a first-class Phase 1 task with
  its own acceptance criteria, not an afterthought. The `players` table already carries
  `source_id text` with `UNIQUE (source_id, tour)`, which accommodates alphanumeric IDs;
  an alias table handles the cross-source join.
- The methodology page gains a data-provenance section. Given the circumstances this is an
  asset: being straightforward about where the data came from, and what is missing, is
  exactly the credibility this project trades on.
