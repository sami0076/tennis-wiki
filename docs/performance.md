# Scale and performance

Measurements, not estimates. The point of issue #20 was that the jump to full-depth
coverage — 1.63 million matches and 115,000 players, per
[ADR-0003](decisions/0003-full-depth-player-coverage.md) — should be measured early, while
the schema is still cheap to change.

**Method.** All figures below are from one machine (Windows, Docker Desktop, Postgres 16.10
in the compose stack) against a fixed slice: every configured source for seasons 2015–2019,
about 160,000 matches. That is roughly a tenth of the full dataset. Where a number is
extrapolated rather than measured it says so.

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

**Extrapolated to the full dataset**: roughly 2 hours, against 3 hours before. Still the
longest operation in the project by far, and still dominated by fetching a few hundred
files over the network. `make ingest` loads the seed fixture in ten seconds precisely so
this is not on anyone's critical path.

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
