# Scale and performance

Measurements, not estimates. The point of issue #20 was that the jump to full-depth
coverage — 1.63 million matches and 115,000 players, per
[ADR-0003](decisions/0003-full-depth-player-coverage.md) — should be measured early, while
the schema is still cheap to change.

**Method.** One machine: Windows, Docker Desktop, Postgres 16.10 in the compose stack.
Early figures came from a fixed slice — seasons 2015–2019, about 160,000 matches — and are
marked as such. **The full dataset has since been ingested**, so the headline numbers are
now measured rather than extrapolated.

## The full dataset, measured

| | |
|---|---|
| matches | 1,624,479 |
| match_players | 3,249,201 |
| players | 125,868 |
| tournaments | 63,681 |
| rankings | 5,123,471 |
| seasons | 1922–2026 |
| **database size** | **2,054 MB** |

Size by table: `match_players` 765 MB, `rankings` 749 MB, `matches` 465 MB, `players` 52 MB.

**The earlier extrapolation of "about 1 GB" was low, and rankings are why.** It was made
before #18 populated them, and 5.1 million ranking rows are 749 MB — more than the matches
themselves. Anything sizing this database has to count them.

**1,355,420 of 1,624,479 matches carry no serve statistics** — 83%. That is not a defect,
it is what full-depth coverage means: Futures and ITF never recorded them, and nothing did
before 1991. It is also why the API distinguishes the kinds of absence rather than
returning zeroes.

## Ingest

| | Wall time | Rate |
|---|---|---|
| Before | 1,112s | 145 matches/s |
| After | 691s | 231 matches/s |

The two runs fetched 161,864 and 159,740 matches — the mirrors differ slightly between
fetches — so the rates are per match rather than the raw times compared directly.

**The bottleneck was one round trip per match.** `pg_stat_activity` showed the connection
`idle in transaction` on `INSERT INTO matches`, waiting for the next statement. Every other
write in the pipeline already batched; matches was the exception, and at 1.6 million rows
that is 1.6 million round trips. Pipelining them into one `pgx.Batch` per chunk removed
about 40% of the wall time.

**Measured on the full dataset**: the match stage ran in roughly 40 minutes of accumulated
wall time across chunks, the reference stage (player tables and the full ranking history) in
about 23 minutes, and identity reconciliation over 126,114 players in **20 seconds**. Still
dominated by fetching a few hundred files. `make ingest` loads the seed fixture in ten
seconds precisely so this is not on anyone's critical path.

### Run it in chunks, not one shot

A full ingest from empty is long enough that anything interrupting it — a machine running
low on memory, a transient DNS failure, one row Postgres refuses — costs the whole run,
because the ingest has no resume: it re-reads every file from the start and re-upserts rows
it already has.

Chunking by tour and season bounds that loss:

```
ingest --stage matches --tours wta --seasons 1922-1989
ingest --stage matches --tours wta --seasons 1990-2005
ingest --stage matches --tours atp --seasons 2020-2026
ingest --stage reference
ingest --stage reconcile
```

Each chunk is idempotent, so a failed one is simply repeated. Resumability that skips
already-complete (source, season) pairs would be a real improvement.

### Concurrency defaults are too aggressive for a busy machine

The default is one reader per CPU (12 here) with 2,000-row batches. On a machine with
Docker, a browser and an IDE already resident, that was enough to get the process killed for
memory pressure twice. `--workers 2 --batch 500` completed comfortably and was not
noticeably slower, since the bottleneck is the network.

### Postgres needs more shared memory than Docker gives it

`cmd/dataqual` failed on the full dataset with:

```
could not resize shared memory segment: No space left on device (SQLSTATE 53100)
```

Docker allocates a container 64 MB of `/dev/shm`, and Postgres uses it for parallel query
workers. At 1.6 million matches the data-quality queries want more. The compose file now
sets `shm_size: 1gb`.

## Database size

At 160,000 matches:

| | Size |
|---|---|
| Total | 102 MB |
| `match_players` | 53 MB |
| `matches` | 35 MB |
| `players` | 4.4 MB |

**Extrapolated to 1.63M matches: about 1 GB**, before ratings. Comfortable.

## The ratings table

Spec §7.6 wants weekly snapshots of four rating series per player, and warns that
snapshotting every player every week is the wrong shape. Measured on the 2015–2019 slice:

| | Rows |
|---|---|
| Snapshot only weeks a player actually played | 651,288 |
| Snapshot every player every week | 9,323,720 |

**14× on five seasons of data, and the ratio grows with the span.** It has to: the naive
figure is `players × weeks`, and most of the 115,000 players are Futures players active for
two or three years out of a fifty-seven-year history. Extrapolating the full dataset —
115,000 players across roughly 2,960 weeks — gives **1.4 billion rows the naive way against
roughly 7 million** if only active weeks are stored.

So the rule is not an optimisation, it is the difference between a table that fits and one
that does not. The rating engine is Phase 2; this is recorded here so it is designed in
rather than discovered.

## Player search

Search ranks by trigram similarity weighted by the best tier a player has reached, so that
a query like "alexander" does not bury Zverev under Futures players of the same name. That
weighting originally came from a lateral aggregate over each candidate's matches.

Measured at 160,000 matches, for the query `martin` — 86 candidates spanning 3,343 matches:

| | Cold | Warm |
|---|---|---|
| Lateral aggregate per candidate | 288ms | 93ms |
| Read from a derived column | 20ms | 44ms |

For a query with few candidates the two are within noise; the cost is proportional to
candidates × their matches, so it is the common surnames that hurt, and they are exactly
what people search for.

`players.career_matches` and `players.best_tier` are now derived columns, recomputed by a
single pass at the end of ingest (1.7s on this slice). Derived columns rather than a
materialised view because a view cannot see uncommitted rows, which would make the ranking
untestable by any test that rolls back.

They are stale between ingest runs, by design. A player's tier and match count move slowly,
and search ranking is the only thing that reads them.

## Not measured

**Full rating recompute wall time**, which #20 also asks for. The rating engine does not
exist yet — it is Phase 2 — so there is nothing to time. The sizing analysis above is the
part that can be settled now, and it is the part that constrains the schema.
