# Architecture

> Stub. Filled in as Phase 1 lands — see the [Phase 1 tracking issue](https://github.com/sami0076/tennis-wiki/issues/16).

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
| `internal/db` | pgx pool setup and sqlc-generated queries |
| `internal/ingest` | Source registry, CSV parsing, upserts |
| `internal/rating` | Elo engine |
| `internal/simulate` | Closed-form match, Monte Carlo draw |
| `internal/score` | Score-string parser |
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

Reasoning for individual columns lives in comments in `migrations/`. Three choices are
worth stating here because they are not obvious from any single file.

**`rating_surface` is a separate enum from `surface`.** The `ratings` primary key includes
the surface, and the overall series has no surface. A nullable column cannot sit in a
primary key, and a generated `COALESCE(surface::text, 'overall')` column is rejected too —
the enum-to-text cast is only STABLE, not IMMUTABLE. A dedicated enum with an `overall`
member is simpler than either and says what it means.

**`matches.winner_id` is denormalised, and constrained.** A deferred composite foreign key
to `match_players (match_id, player_id)` guarantees the winner actually played. The
constraint is deferred because a match row is written before its participants inside one
transaction. It does not enforce that the winner's `won` is true — that stays a `cmd/dataqual`
check.

**`matches` is not partitioned yet.** At ~1.63M rows it does not need to be, and
partitioning by season would force the partition key into the primary key and every
foreign key referencing it. Revisit under #20 with measurements rather than now on
speculation.

## To be documented

Filled in as the work happens rather than guessed at now:

- [ ] Ingest pipeline: worker pool shape, batching, idempotency keys
- [ ] Rating recompute: runtime over the full dataset (#20)
- [ ] Query plans for the hot paths, with `EXPLAIN` output (#20)
- [ ] Deployment topology (Phase 4)

## Decisions

Architecture decision records live in [`docs/decisions/`](decisions/).
