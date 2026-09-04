# Tennis Wiki

[![CI](https://github.com/sami0076/tennis-wiki/actions/workflows/ci.yml/badge.svg)](https://github.com/sami0076/tennis-wiki/actions/workflows/ci.yml)

Deep per-player statistics, head-to-head comparison, and first-principles match and draw
simulation for **both the ATP and WTA tours** — built from raw match data, with the
working shown.

> **Status: Phase 1 (data foundation) in progress.** Nothing below the Roadmap is live
> yet. This README is written to the target shape so it fills in as phases land; sections
> marked _(pending)_ are placeholders, not claims. See [Roadmap](#roadmap) for what
> actually exists today.

<!-- SCREENSHOT: head-to-head page. Required by the build spec §13.2 — first thing after
     the title once Phase 2 ships. -->
_Screenshot of the head-to-head page — pending Phase 2._

**Live URL:** _pending Phase 4._

---

## Why this exists

Three things no free tennis site does well together:

- Deep per-player statistics for **ATP and WTA on the same footing**. Most sites treat the
  women's tour as an afterthought or omit it entirely.
- **Every player, not just the famous ones.** Around 1.6 million matches across tour,
  Challenger, Futures and ITF, and over 115,000 players. A player ranked 400 gets a real
  page, not an empty one — see [ADR-0003](docs/decisions/0003-full-depth-player-coverage.md).
- Head-to-head comparison with **surface, era, and form context** — not just a win-loss
  tally.
- **Match and draw simulation from first principles**, showing every intermediate step
  from point-win probability up to match probability.

The closest prior art, [Ultimate Tennis Statistics](https://github.com/mcekovic/tennis-crystal-ball),
is ATP-only. This project's differentiation is WTA parity, the simulators, and an
interface that is designed rather than assembled.

## Architecture

```mermaid
flowchart LR
    subgraph sources["Data sources (CC BY-NC-SA 4.0)"]
        A["Sackmann-lineage mirrors<br/>ATP + WTA, all tiers<br/>tour / Challenger / Futures / ITF"]
        B["TML-Database<br/>ATP 2025-26"]
        C["Match Charting Project<br/>shot-by-shot, to May 2026"]
    end

    A --> I["cmd/ingest<br/>streaming, idempotent"]
    B --> I
    C --> I

    I --> PG[("PostgreSQL 16")]
    PG --> R["cmd/rate<br/>Elo engine"]
    R --> PG
    PG --> API["cmd/api<br/>Go + chi"]
    RD[("Redis 7")] <--> API
    API --> SIM["internal/simulate<br/>closed form + Monte Carlo"]
    SIM --> API
    API --> WEB["web/<br/>React + TS + Vite"]

    V["cmd/validate"] -.reads.-> PG
    DQ["cmd/dataqual"] -.reads.-> PG
```

Go handles ingestion, rating, simulation, and the API. PostgreSQL does the statistical
heavy lifting through window functions. The frontend is a static SPA served from a CDN —
deliberately **not** in the Kubernetes cluster.

## Coverage

| Tier | Matches (approx.) | Serve statistics |
|---|---|---|
| ATP tour | 195,000 | 89% (2005) → 99% (2022) |
| ATP qualifying + Challenger | 223,000 | 0% (2005) → 99.7% (2022) |
| ATP Futures | 447,000 | none, in any year |
| WTA tour | 250,000 | 19% (2005) → 89% (2022) |
| WTA qualifying + ITF | 488,000 | effectively none |
| **Total** | **~1.63 million** | |

Over **115,000 players** across both tours. WTA records reach back to 1923.

Depth is uneven and the site is explicit about it rather than hiding it: Futures and ITF
matches support win/loss, head-to-head, and Elo but carry no point-level data, so they
cannot feed the simulator. Where a statistic does not exist, the site explains why instead
of showing a zero. Full detail in [`DATA_LICENSE.md`](DATA_LICENSE.md).

## Run it locally

Needs Docker and Go 1.23+.

```bash
docker compose up -d      # Postgres 16 + Redis 7
make migrate-up           # apply the schema
```

Postgres is published on **5433** and Redis on **6380**, not the defaults, because a local
install very often already holds 5432 and 6379.

Useful targets — `make help` lists them all:

| | |
|---|---|
| `make up` | start the stack and wait until healthy |
| `make down` | stop it, keeping data |
| `make reset` | stop it and delete all data |
| `make psql` | open a shell on the database |
| `make testdb` | create the disposable database the integration tests need |
| `make api` | run the HTTP API on port 8080 |
| `make test` | run the test suite |
| `make lint` | run golangci-lint |

### The API

`make ingest` then `make api`, and the read-only API is up:

```
GET /api/v1/health                        readiness, including a database round trip
GET /api/v1/players?q=&tour=&limit=       fuzzy search, diacritic-insensitive
GET /api/v1/players/:slug                 profile and career summary
```

```bash
curl 'localhost:8080/api/v1/players?q=Djokovi%C4%87'
curl localhost:8080/api/v1/players/novak-djokovic
```

Statistics that were never recorded are reported as absent with a reason, never as zero —
see [Coverage](#coverage). Errors are RFC 7807 `problem+json`, and list responses are
cursor-paginated.

The frontend is not built yet — see the
[Phase 1 tracking issue](https://github.com/sami0076/tennis-wiki/issues/16). The eventual
one-command target is `docker compose up` producing a working, seeded site, with
`make ingest-full` for the complete dataset.

## Rating methodology

Ratings are computed from scratch over every match in chronological order — no ratings
are imported from any API. Standard Elo, with a decaying K-factor so that a player's early
matches move their rating far more than their five-hundredth:

$$K(n) = \frac{250}{(n + 5)^{0.4}}$$

where `n` is the number of tour-level matches completed **before** the current one. This
gives K ≈ 130 for a debutant and K ≈ 25 for a veteran with 500 matches, then scales by a
match-importance weight (Grand Slam final 1.20 down to team events 0.80).

Four independent rating series are kept per player — overall, hard, clay, grass. For
display and simulation they are blended:

$$\text{blended} = w \cdot \text{surface elo} + (1 - w) \cdot \text{overall elo}, \qquad w = \min\left(0.75, \frac{\text{surface matches}}{40}\right)$$

so a player with five clay matches leans on their overall rating while a clay specialist
with a hundred leans on their clay rating. Ratings are recomputed from scratch on every
full ingest and never incrementally patched, so a bug fix is always one rerun away from
correct.

The full derivation, including the simulation chain from point to match, will live at
`/methodology` on the live site.

## Data attribution and license

This project is built on data originating from the work of **Jeff Sackmann /
[Tennis Abstract](http://www.tennisabstract.com/)**, licensed
**[CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/)**.

> **Note (September 2026):** the canonical `JeffSackmann/tennis_atp` and
> `JeffSackmann/tennis_wta` repositories are no longer public. Only
> [`tennis_MatchChartingProject`](https://github.com/JeffSackmann/tennis_MatchChartingProject)
> remains. This project therefore ingests from license-compliant redistributions of that
> data. See [`DATA_LICENSE.md`](DATA_LICENSE.md) for the full provenance chain and the
> exact sources used.

The license is taken seriously here, because its author takes it seriously:

- **Attribution** appears in the site footer on every page and in `DATA_LICENSE.md`.
- **NonCommercial** — there are no ads, no payment flows, and no paid tier. Ever.
- **ShareAlike** — all ingested and derived data, including the seed fixtures in this
  repo, is redistributed under CC BY-NC-SA 4.0.

**Code** in this repository is licensed under the [MIT License](LICENSE). **Data**, and
any dataset derived from it, is licensed under
[CC BY-NC-SA 4.0](DATA_LICENSE.md). See
[ADR-0001](docs/decisions/0001-dual-license-code-and-data.md) for the reasoning behind
that split.

## Deliberately not built

- **Live in-play scores.** Requires a paid feed ($40/month at the low end, enterprise
  quote at the high end). The architecture leaves a websocket seam for it, but shipping it
  would mean either paying indefinitely or scraping — neither is defensible for a public
  non-commercial site.
- **Betting odds, tipping, or predictions framed as picks.** The simulator reports
  probabilities and shows its working. It is not a gambling product and will not be
  shaped into one.
- **User accounts, comments, social features.** They add moderation burden and privacy
  obligations without making the statistics better.
- **Mobile apps.** The site is responsive to 360px; that is the right amount of mobile
  investment for this product.
- **Doubles** _(v1)_. ~26,000 doubles matches exist and are not discarded, but they need a
  four-player match model and a separate rating treatment. The schema does not preclude
  them; the work is deferred.

## Roadmap

| Phase | Scope | Status |
|---|---|---|
| 1 | Ingestion, schema, read-only API | In progress |
| 2 | Elo engine, player pages, H2H, rankings, search | Not started |
| 3 | Match simulator (closed form), draw simulator (Monte Carlo) | Not started |
| 4 | Match Charting Project, clutch metrics, methodology page, k3s, image builds | Not started |

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — system design _(pending)_
- [`docs/methodology.md`](docs/methodology.md) — ratings and simulation in full _(pending)_
- [`docs/decisions/`](docs/decisions/) — architecture decision records
