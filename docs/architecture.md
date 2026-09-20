# Architecture

> Current as of the end of Phase 4, 16 September 2026, and deployed at
> [deucepoint.net](https://deucepoint.net). The simulation chain is in
> [`docs/methodology.md`](methodology.md); the measurements every figure below rests on
> are in [`docs/performance.md`](performance.md).

## Shape of the system

Four Go binaries and an API share one PostgreSQL database. Nothing talks to anything else
over the network internally; the database is the integration point.

```
cmd/ingest   →  reads CSV sources, writes matches/players/rankings
cmd/rate     →  reads matches, writes ratings
cmd/dataqual →  reads everything, writes a report
cmd/validate →  reads ratings + matches, writes a report
cmd/api      →  reads everything, serves JSON
```

`ingest` and `rate` are batch jobs. `api` is the only long-running service. This split is
deliberate: the expensive work happens offline and on a schedule, so the request path stays
a read against indexed tables.

## Repository layout

| Path | Contents |
|---|---|
| `cmd/` | One directory per binary. Thin — argument parsing and wiring only |
| `internal/cache` | Redis read cache, cleared by the ingest |
| `internal/db` | pgx pool setup and sqlc-generated queries |
| `internal/ingest` | Source registry, CSV parsing, upserts |
| `internal/rating` | Elo engine |
| `internal/simulate` | Closed-form match, Monte Carlo draw |
| `internal/score` | Score-string parser |
| `internal/identity` | Player reconciliation across the source id spaces |
| `internal/charting` | The Match Charting Project, attached to matches the database already holds |
| `internal/events` | What a tournament is across seasons: the rule that keys an event, and the slugs |
| `internal/validate` | Rating accuracy, calibration and continuity checks |
| `internal/dataqual` | Data-quality checks over the loaded database |
| `internal/testdb` | Throwaway Postgres and Redis containers for the tests |
| `internal/httpapi` | Handlers and middleware |
| `migrations/` | goose SQL |
| `testdata/` | Fixtures. Tests never touch the network |

There is no `pkg/`. Everything is `internal/` until something outside the repository needs
to import it, which so far nothing does.

## Conventions

- `log/slog` for logging. No logrus, no zap.
- Errors wrapped with `fmt.Errorf("...: %w", err)`. No error libraries.
- `context.Context` threaded through anything touching the database or network.
- Each binary is `main` → `run(ctx) error`, so failure paths return rather than call
  `os.Exit` from deep in the stack, and SIGTERM drains cleanly.

## Schema decisions

Reasoning for individual columns lives in comments in `migrations/`. A few choices are
worth stating here because they are not obvious from any single file.

**`rating_surface` is a separate enum from `surface`.** The `ratings` primary key includes
the surface, and the overall series has no surface. A nullable column cannot sit in a
primary key, and a generated `COALESCE(surface::text, 'overall')` column is rejected too —
the enum-to-text cast is only STABLE, not IMMUTABLE. A dedicated enum with an `overall`
member is simpler than either and says what it means.

**`matches.winner_id` and `matches.loser_id` are denormalised, and constrained.** A deferred
composite foreign key each, to `match_players (match_id, player_id)`, guarantees both
players actually played. The constraints are deferred because a match row is written before
its participants inside one transaction. They do not enforce that the winner's `won` is true
and the loser's is false — that stays a `cmd/dataqual` check.

**A match is identified by its draw, its number, and the pair who played it.** `match_num` is
unique within a draw block, not within a tournament, and one `tourney_id` can hold more than
one block — with nothing in the source to tell them apart. Keyed on the number alone, two
different matches share a row and accumulate four participants. The pair is stored unordered
in the index, so a source correcting who won updates the match rather than duplicating it.
Migrations 00007 and 00011 carry the measurements.

**An event is derived, and every tournament row says how it got onto its own.** The
sources carry a row per event per season and no identity across seasons that holds for
both tours: the ATP's number does, the WTA's is a sequence within the year until 1987 and
the ITF circuit's to 1995. So `events` is written by a stage of the ingest from a rule —
the tour's number where it is real, the name within tour and tier where it is not, a
short overrides file for the numbers a person has checked — and `tournaments.event_link`
records which of those placed each row, for the page to print. ADR-0012 carries the
measurements; `configs/event_overrides.json` the decisions.

**What a leaderboard ranks is summed once, not per request.** A board over the whole
database is an aggregate of 3.3 million appearances, and summing them on demand took eleven
seconds. `player_totals` holds the sums per player, season, tier and surface — matches,
wins, titles, the serve line, the opponents' serve line, and what the score said — rebuilt
by the ingest's refresh step in twenty seconds alongside the clutch and serve baselines.
The same step derives sets and games from every score string, the way tiebreaks and the
deciding set already were. The leaderboards and the player page's year-by-year read that
table and nothing heavier; `docs/performance.md` carries the figures.

**`matches` is not partitioned yet.** At ~1.63M rows it does not need to be, and
partitioning by season would force the partition key into the primary key and every
foreign key referencing it. Revisit under #20 with measurements rather than now on
speculation.

## Deployment topology

One node and a CDN. Everything the API needs runs in a single-node k3s cluster on one
VPS; the frontend is static files on Cloudflare Pages and is deliberately not in the
cluster.

```
browser ──HTTPS──▶ deucepoint.net        Cloudflare Pages: the SPA, built from web/ on main
   │
   └──HTTPS──▶ api.deucepoint.net        one VPS, k3s
                 └─ Traefik (80/443, Let's Encrypt via cert-manager)
                      └─ api ×2 ──▶ redis (cache, allowed to be empty)
                                └──▶ postgres (StatefulSet, local-path on the node's disk)
                 jobs, by hand: migrate (goose), load (ingest, then rate)
```

The browser talks to two origins, so CORS is load-bearing and the API names the site's
origin explicitly. Images come from GHCR pinned by digest; CI builds them, a person
deploys them. The database is not backed up: every input is public and the load Job
rebuilds it from empty in about an hour, which is the recovery plan.

[`docs/deployment.md`](deployment.md) has the procedures; [ADR-0008](decisions/0008-published-images-and-target-architecture.md)
and [ADR-0009](decisions/0009-postgres-in-the-cluster.md) have the reasoning; the
manifests are in [`deploy/k8s/`](../deploy/k8s/README.md).

## The ingest pipeline

A plan, a pool of readers, and one writer. The plan is every (source, season) file the
registry says exists for the tours and seasons asked for — 340 on a full run — minus the
ones the ledger says have not changed. Readers take files from a channel, parse them
streaming, and hand chunks of parsed rows to a single writer over a channel; the writer
holds the only connection that writes matches, so the database sees one ordered stream
however many files are being read. The defaults are one reader per CPU and 2,000 rows per
chunk; the live cluster runs two readers and 500-row chunks, because the bottleneck is the
mirrors and not the machine, and the defaults were once enough to get the process killed
for memory next to a busy Postgres while two readers were not measurably slower.

Each chunk is one transaction. Matches, participants and stat lines go in as one
`pgx.Batch` — one round trip per chunk rather than per row, which was the difference
between 145 and 231 matches a second. The write is an upsert on the match's natural key,
`(tournament, match number, qualifying, the two players)`: the same row from two mirrors
lands once, and a corrected row replaces its earlier self. A file is recorded in
`ingest_files` with its ETag only after its last chunk commits, so a killed run resumes at
the file it was in, and an unchanged file on the next run costs one conditional request.
The reference stage (player tables, ranking history), the events stage and the charting
stage run after the matches, sequentially, because each needs everything the stage before
it created.

## The rating recompute

`cmd/rate` replays every match from scratch in draw order — the round within an event,
not the calendar, since most events carry one date for all their matches — and never
patches a rating in place. On the full database that is 1.61 million rated matches, 75,000
players and 3.1 million snapshots in **1m 41s**, and it runs after every load and after
any identity merge, because a merge moves matches between players and the stored ratings
would otherwise describe a database that no longer exists. Snapshots are written only for
the weeks a player played: every player every week would be 1.4 billion rows against
roughly 7 million.

## The hot paths

Measured rather than planned, and recorded by shape rather than as `EXPLAIN` dumps, which
go stale with the statistics and were dropped for that reason.

- **A player's match history** reads `match_players` by player, joins each of their matches
  by primary key, sorts on `(played_on, id)` and returns a page; the cursor is a keyset on
  that pair, so a deep page costs what the first one does — 19ms warm for the longest
  career in the database, 590ms cold. The ordering key lives on `matches` and the player on
  `match_players`, which is why every page reads the whole career; carrying `played_on` on
  `match_players` would make it a 25-row scan and is not yet worth the schema change.
- **A player's ratings** read the `ratings` primary key, which begins with `player_id`, so
  a trajectory of 406 weekly points is 3ms and a current-and-peak per series 9ms warm.
- **Search** ranks trigram similarity on `full_name` (a GIN index) weighted by two derived
  columns on `players` — the career match count and the best tier reached — refreshed
  after every ingest, in place of the lateral aggregate over each candidate's matches that
  it began as.
- **Head-to-head** is one query over both players' `match_players` rows joined on the match;
  the record, the surface split and the tier split are counted in Go from the same rows so
  three aggregates cannot disagree.
- **The API** answers from a Redis read cache with a 24-hour TTL that the ingest clears when
  it finishes; nothing else invalidates it, because nothing else changes the data.

## Decisions

Architecture decision records live in [`docs/decisions/`](decisions/).
