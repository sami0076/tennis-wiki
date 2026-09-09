# Deucepoint

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
of showing a zero.

**There is a currency gap, and it is disclosed rather than papered over.** Full-schema data
runs through **2026-01-17 (ATP)** and **2024-12-31 (WTA)**; the upstream repositories were
withdrawn mid-project and the mirrors stop there. Filling it with results-only data would
put two data regimes in one database and produce plausible-looking wrong numbers, so
[ADR-0006](docs/decisions/0006-accept-and-disclose-the-coverage-gap.md) accepts the gap
instead. `GET /api/v1/coverage` reports the real dates, queried from the database, so the
claim cannot drift from the data.

Full detail in [`docs/methodology.md`](docs/methodology.md) and
[`DATA_LICENSE.md`](DATA_LICENSE.md).

## Run it locally

Needs Docker and Go 1.23+.

```bash
make seed                 # Postgres + Redis, schema, and ~4,100 real matches
make api                  # http://localhost:8080
```

`make seed` takes about ten seconds. The fixture is small but deliberately covers every
data regime the site has to handle — full serve statistics, partial, never recorded at
this tier, and never recorded in this era — so the interesting behaviour is visible
immediately rather than after a full ingest. See [`testdata/README.md`](testdata/README.md).

Or step by step:

```bash
docker compose up -d      # Postgres 16 + Redis 7
make migrate-up           # apply the schema
make ingest               # load the seed fixture
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
| `make seed` | stack, schema, and seed fixture in one |
| `make api` | run the HTTP API on port 8080 |
| `make test` | run the test suite (starts its own Postgres) |
| `make lint` | run golangci-lint |

### The API

`make ingest` then `make api`, and the read-only API is up:

```
GET /api/v1/health                        readiness, including a database round trip
GET /api/v1/coverage                      what is actually in the database, and through when
GET /api/v1/players?q=&tour=&limit=       fuzzy search, diacritic-insensitive
GET /api/v1/players/:slug                 profile and career summary
GET /api/v1/players/:slug/matches         match history, filterable and cursor-paged
GET /api/v1/players/:slug/ratings         Elo trajectory, per surface
GET /api/v1/players/:slug/rankings        published ATP/WTA ranking over time
GET /api/v1/players/:slug/clutch          break points, tiebreaks, deciding sets
GET /api/v1/h2h/:slug/:opponent           head-to-head, either way round
GET /api/v1/rankings?type=elo|official    leaderboards, as of the last week that exists
GET /api/v1/rankings/trajectory           the leaders' rating lines, for a chart
```

```bash
curl 'localhost:8080/api/v1/players?q=Djokovi%C4%87'
curl localhost:8080/api/v1/players/novak-djokovic
curl 'localhost:8080/api/v1/players/novak-djokovic/matches?surface=clay&season=2016'
```

The history takes `surface`, `tier`, `season` and `opponent`, and they compose. Each row
carries that player's serve line for the match, or the reason there is none.

A ranking is always **as of the last week that exists**, never today, and the response says
which week it used. Asking for a date outside coverage returns an empty page with that date
stated rather than a 404. An Elo leaderboard also drops players whose last rating is over a
year old — without that it is a list of the retired.

**Under pressure is measured against a stated population.** `/clutch` reports break points
saved, tiebreaks won and deciding sets won, each against what the tour did at the same
levels in the same decades, weighted by where that player's own opportunities actually
fell -- so a Futures career is not measured against a tour average, and a career spanning
1995 and 2025 is not measured against either one alone. The population is in the response,
because a "+4" against an unnamed average is a number pretending to be a fact. Tiebreaks
and deciding sets are derived from the parsed score and the baseline is a table the ingest
rebuilds, since neither belongs in a request. Two of the three averages come out at exactly
50%, and [`docs/performance.md`](docs/performance.md) explains why that is arithmetic
rather than a finding.

The profile carries current and peak Elo for every series the player has one in — a surface
they never played is absent rather than sitting at the base rating, and a player nothing
rated has a null block rather than five 1500s.

Statistics that were never recorded are reported as absent with a reason, never as zero —
see [Coverage](#coverage). Errors are RFC 7807 `problem+json`, and list responses are
cursor-paginated.

`make ingest-full` performs the complete multi-decade ingest from the configured sources.

Tests need Docker but no setup: the integration suites start a throwaway Postgres per
package, apply the migrations, and stop it afterwards. Without Docker they skip with an
explanation rather than failing. Set `TEST_DATABASE_URL` to use a server you already have;
it must be named `*test*`, because some of those tests truncate tables.
### The frontend

```bash
make web        # Vite dev server on :5173, proxying /api to the local API
make site       # the whole stack in Docker: site on :5174, API on :8080
make web-build  # tsc, eslint, vitest and a production build
```

The site is **Deucepoint**; the repository, the Go module and the compose project keep the
name `tennis-wiki`, which is an identifier rather than a brand and is not worth the churn of
renaming.

React, TypeScript and Vite under `web/`, styled with CSS Modules and custom properties.
No Tailwind, no component library, no charting library — the design rests on a small token
set and hairline structure, and the reasoning is in
[`docs/design/design-system.md`](docs/design/design-system.md). `/_components` renders every
component in every state, which is the fastest way to check the system against a design, and
`/players/:slug` is the first real page.

`/h2h/:a/:b` is the comparison and `/h2h` the picker, which is the same page: the URL is the
state, so a comparison is a link somebody can send, and asking the other way round is the same
rivalry read from the other end. The surface toggle filters the record, the rivalry strip and
the meeting list from the meetings themselves; it cannot filter the serve figures, because the
endpoint aggregates a rivalry once, and the caption says so rather than letting them look
filtered. Two players who never met is a full page, not an error.

Search is in the header on every page: a combobox rather than a div that looks like one, so
arrow keys and a screen reader reach the same results. It debounces and cancels superseded
requests, and every row carries tour, country, career match count and best tier, because at
115,000 players a name is not an identifier. `/players?q=` is the same search as a full list,
and the query and tour filter live in the URL so a search can be sent to somebody.

**The TypeScript types are generated from the Go response structs**, not written by hand.
`make web-types` runs [tygo](https://github.com/gzuidhof/tygo) over `internal/httpapi` into
`web/src/api/types.gen.ts`, and CI regenerates and diffs it on every change, so altering a
handler's shape without updating the client fails the build rather than the browser. Same
discipline as sqlc, one layer up.

**The absence system is three components, not one.** `AbsentCell` is a missing value in a
populated table, `PartialAggregate` is a summary over a gappy column that has to declare
its denominator, and `EmptyState` is a whole section that explains itself and points
somewhere with data. 83% of matches carry no serve statistics, so these render more often
than the numbers do; collapsing them into a falsy check would throw away the distinction
every layer below works to preserve.

`docker compose up` builds and runs the whole stack, with the API same-origin behind nginx
so the built site and the dev server behave identically. `make ingest-full` loads the
complete dataset.

## Rating methodology

Ratings are computed from scratch over every match in chronological order — no ratings
are imported from any API. Standard Elo, with a decaying K-factor so that a player's early
matches move their rating far more than their five-hundredth:

$$K(n) = \frac{250}{(n + 5)^{0.4}}$$

where `n` is the number of matches that player has completed **before** the current one, in
that series. This gives K = 131.33 for a debutant and K = 20.73 at 500 matches — the spec's
gloss of "about 25" is wrong, and K reaches 25 at roughly 311 matches
([ADR-0004](docs/decisions/0004-tier-taxonomy-and-elo-pool.md)). K then scales by a
match-importance weight, from a Grand Slam final at 1.20 down to Futures at 0.60.

Matches are replayed in draw order, not date order: nearly every tournament in the source
carries a single date for all of its matches, so ordering by date alone would rate a final
before the semi-final that produced its finalist. Walkovers are not rated — nobody played
them — and team events are excluded by default.

Five independent rating series are kept per player — overall, hard, clay, grass, carpet.
A series is snapshotted only in the weeks it moved, which is what keeps the table at three
million rows instead of two billion. For display and simulation they are blended:

$$\text{blended} = w \cdot \text{surface elo} + (1 - w) \cdot \text{overall elo}, \qquad w = \min\left(0.75, \frac{\text{surface matches}}{40}\right)$$

so a player with five clay matches leans on their overall rating while a clay specialist
with a hundred leans on their clay rating. Ratings are recomputed from scratch on every
full ingest and never incrementally patched, so a bug fix is always one rerun away from
correct.

`make validate` replays the whole history and reports predictive accuracy and calibration
per tier, mean reversion across the pool, and promotion continuity. Tour-level accuracy is
**69.9%**, inside the 68-72% the spec asks for. The tier weights are configuration rather
than constants, so `validate --weights` tries alternatives without a rebuild; what the
evidence says about them is in [the methodology](docs/methodology.md).

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
- [`docs/methodology.md`](docs/methodology.md) — coverage now; ratings and simulation with Phases 2 and 3
- [`docs/performance.md`](docs/performance.md) — measured ingest, size and query cost at scale
- [`docs/decisions/`](docs/decisions/) — architecture decision records
