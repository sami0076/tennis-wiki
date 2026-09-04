# ADR-0005: No ORM — sqlc over GORM and ent

- **Status:** Accepted
- **Date:** 2026-09-04
- **Implements:** build specification §4

## Context

The read path for this project is not CRUD. Almost every question a page asks is an
aggregate over `match_players`: career serve percentages, surface splits, form over the
last twenty matches, head-to-head records, ranking movement. The natural expression of
most of them is a window function or a `FILTER`ed aggregate.

Three options were on the table: an ORM (GORM), a schema-first toolkit with a query
builder (ent), or generated typed functions over hand-written SQL (sqlc).

Two constraints shape the choice.

**The statistics are the product.** A wrong number here is worse than a missing one, and
the project has already produced two classes of plausible-looking wrong answers — a
natural key that collapsed distinct matches, and a tier assigned per source file rather
than per row. Both were found by reading the SQL against the data. Query text that is
generated at runtime and never read is a bad fit for a codebase where the queries are the
thing most likely to be subtly wrong.

**Missing statistics carry meaning.** `serve_points` is `NULL` for every pre-1991 match
and every Futures match ever played, and that is a fact to report, not a gap to paper
over. `NULL`, zero, and "never recorded for this tier" are three different answers and
the API has to distinguish them. This has to survive from the SQL through the generated
types to the JSON, which puts the emphasis on faithful type mapping rather than on
ergonomics.

## Decision

**No ORM. `sqlc` generates typed Go functions from SQL kept in `internal/db/queries/`.**

- The SQL is written by hand and reviewed as SQL.
- `sqlc` reads `migrations/` directly, so the schema it types against is the schema that
  will exist — a query referring to a dropped column fails at generate time.
- Generated code is committed, so `go build` and CI work without the tool installed.
  `make sqlc` regenerates it, pinned to one version.
- `emit_pointers_for_null_types` is on. A nullable column becomes `*T`, so dropping a
  `NULL` into a zero-valued field takes a deliberate dereference rather than happening by
  default.
- Anything `sqlc` cannot type — dynamic filters, or a query built from user input — is
  written against `pgx` directly rather than worked around in the config.

## Consequences

**What this costs.** No lazy loading, no association traversal, no automatic migration
from struct tags. Every query is written out. A new endpoint that needs a slightly
different projection means new SQL, not a new argument to a finder. Rewriting a table
means touching every query that reads it, with the compiler pointing at each one.

**What it buys.** The exact SQL that runs is in the repository and reviewable. Window
functions, `FILTER`, lateral joins, and trigram operators are available without fighting a
builder that has no vocabulary for them — the player search in `SearchPlayers` uses a
lateral join and `pg_trgm` and would be awkward in any of the alternatives. Query planning
problems are debuggable, because the statement in the file is the statement in
`pg_stat_statements`.

**On GORM specifically.** Its `NULL` handling — zero values and `sql.Null*` mixed with
struct tags controlling both — is the wrong default for a dataset whose missing values are
load-bearing.

**On ent.** Closer to acceptable: schema-as-code, real type safety, good migrations. It
was rejected because the aggregate queries above would mostly end up in its raw-SQL escape
hatch, leaving the schema definition as a second source of truth alongside `migrations/`
without much in return.

**Revisit if** the write path grows beyond ingest. This choice is made for a read-heavy
API in front of a batch-loaded database; a transactional application with a wide write
surface would weigh it differently.
