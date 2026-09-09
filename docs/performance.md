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

### A second run over unchanged sources costs almost nothing

An ingest used to re-read every configured file from the start and re-upsert rows it already
had. `ingest_files` now records, per file, the validator the mirror gave for the content that
was read, and the next run asks for that file conditionally.

```
GET wta_matches_1937.csv                          200, 498,854 bytes
GET wta_matches_1937.csv  If-None-Match: "bd83…"  304,       0 bytes
```

Measured on the full database, two slices:

| slice | files | rows | cold | unchanged |
|---|---|---|---|---|
| ATP 2015–2024 | 27 | 224,752 | 376s | **16s** |
| WTA 1990–2000 | 22 | 111,823 | 116s | **4s** |

Both unchanged runs read zero rows. `--force` over WTA 1990–1992 reported `already_ingested=0`
and read all six files, as it should.

**Interruption, tested by killing the process mid-run.** Four files had committed; the ledger
held exactly those four, and re-running the same command skipped them and read the remaining
eighteen:

```
ingest finished  files_read=18  files_skipped=4  files_missing=0
```

A file is recorded only after every one of its rows has been committed, so a recorded file
is one the database genuinely holds — which is also what makes an interrupted run resume at
the file it died on rather than at the beginning. `--force` re-reads regardless.

Two things deliberately do **not** get recorded. A ranking file with rows referencing a
player who does not exist yet stays unrecorded, because skipping it on the run that finally
has that player would lose those rankings for good. And `ingest --stage prune` un-records
the files whose matches it deletes, since leaving them would make the next run skip exactly
the files it has to read again.

### Run it in chunks, not one shot

A full ingest from empty is long enough that something will interrupt it — a machine running
low on memory, a transient DNS failure, one row Postgres refuses. That used to cost the whole
run. It now costs the file that was in flight, because every file completed before the
interruption is recorded and skipped on the next attempt.

Chunking by tour and season is still worth doing on a first run, to bound how much a single
attempt has to get through:

```
ingest --stage matches --tours wta --seasons 1922-1989
ingest --stage matches --tours wta --seasons 1990-2005
ingest --stage matches --tours atp --seasons 2020-2026
ingest --stage reference
ingest --stage reconcile
```

Each chunk is idempotent, so a failed one is simply repeated — and now a repeated one is
nearly free.

### Repair a few rows without re-reading 1.6 million

Every write is an upsert, so tightening a rule does not remove the rows already written
under the old one — only a re-ingest does, and that is an hour to clear seven rows.
`ingest --stage prune` reaches the same state in seconds by applying the current rules to
the stored rows instead of the source rows. `--dry-run` counts first.

It does two repairs. A stat line that cannot describe a match loses its statistics and keeps
its match. A match holding other than two players cannot be repaired in place at all — it is
two source rows fused by an outgrown natural key — so it is deleted, and prune names the
(source, season) pairs to re-ingest:

```
prune: cleared matches holding other than two players matches=131
prune: re-ingest to write these matches back separately source=sackmann-wta-tour season=1939 matches=30
```

Repairing all 131 that way took **under two minutes**: 24 files rather than 1,073.

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

### The engine, measured on the full database

`make rate` replays every match from scratch. Measured on 1,624,610 stored matches:

| | |
|---|---|
| matches rated | 1,585,442 |
| players rated | 75,021 |
| **wall time** | **1m 36s** |
| snapshots written | 3,065,844 |
| active weeks | 5,114 |
| `ratings` size | 449 MB |

**The sparse rule holds, and one more turn of it was worth taking.** Three candidate
snapshot rules, counted on this database:

| Rule | Rows |
|---|---|
| Every player, every week, five series | ~1.92 billion |
| Every week a player played, five series | 7,732,645 |
| **Only the series that moved that week** | **3,065,844** |

The middle row is the projection this document made before the engine existed, and it is
right about the shape. The third is what is implemented: a clay week does not restate a
player's grass rating, because nothing about it changed. Reading a rating "as of" any date
is a lookup of the last row at or before it either way, so the extra sparsity costs the
read path nothing. 1,546,529 active player-weeks produce two rows each on average — an
overall series and the one surface that week was played on.

**Two runs from scratch are byte-identical.** The match order is total (date, tournament,
round, match number, id) and each week's rows are emitted sorted, so a rerun is safe to
repeat rather than something to be careful with:

```
md5 of the whole table, run 1:  23bafa74a5b66787d940f6f6c3e4c877
md5 of the whole table, run 2:  23bafa74a5b66787d940f6f6c3e4c877
```

**39,168 matches are not rated, and the run says so rather than implying it:**

```
rate finished  matches=1585442  players=75128  snapshots=3065844
               excluded_team_events=27001  excluded_walkovers=12167
```

Team events are excluded by default — a Davis Cup tie is played for a country and the
selection is not the player's — and `--team-events` includes them. A walkover is excluded
because nobody played it; a retirement is rated, since tennis was played and someone won.
The 2,224 players who appear in the database but not in the ratings are those whose only
matches were one of those two kinds.

**The pool mean is 1481 against a base of 1500**, measured by `cmd/validate` over each
player's final rating. An earlier note here said 1684 and was wrong: that figure was the
mean of the `ratings` rows, which weights every player by how many weeks they played and so
counts the strong ones hundreds of times. The pool is not inflated. Tour players sit at
1520, above the pool, which is what a single weighted pool is supposed to look like.

The names it produces are the right ones — Graf, Navratilova, Seles, Evert, Djokovic at the
top of the all-time peaks — and the validation below says the accuracy is where the spec
expects it.

## Validating the ratings

`make validate` replays the whole history without writing anything and reports on what the
engine believed as it went. **12.7 seconds** for 1,585,442 matches, against the 96 seconds
`make rate` takes — the difference is the three million snapshots it does not write.

Measured on the full database with the ADR-0004 weights:

| Tier | Predictions scored | Accuracy | Mean calibration gap |
|---|---|---|---|
| tour | 449,144 | **69.9%** | 1.6 |
| challenger | 216,073 | 64.5% | 1.3 |
| futures | 326,368 | 68.5% | 1.3 |
| itf | 279,006 | 68.3% | 1.6 |

Tour-level accuracy lands inside the 68–72% the spec asks for. A prediction is scored only
once both players have ten matches of history: a match between two debutants is a coin toss
the engine had no way to call, and counting it would measure the pool's shape rather than
the model.

Challenger is the hardest tier to call, which is what a tier full of players moving between
levels should look like.

### Promoted players beat their ratings, and the weights are not the whole reason

The promotion-continuity check is the sensitive one. Across **7,050 promotions and 81,093
first tour-level matches, players won 33,120 against 31,323 expected — z = +14.5**, well
past the ±3 threshold. Promoted players arrive underrated.

The obvious reading is that the lower tiers are weighted too low. Running the alternatives
says that is only part of it:

| Challenger / Futures | Promotion z | Tour accuracy | Challenger calibration | Futures calibration |
|---|---|---|---|---|
| **0.80 / 0.60** (ADR-0004) | +14.50 | 69.9% | 1.33 | 1.33 |
| 0.90 / 0.75 | +12.69 | 70.0% | 1.74 | 0.40 |
| 1.00 / 0.90 | +11.24 | 70.0% | 2.13 | 0.50 |

Raising the lower tiers by a quarter removes about a fifth of the surplus and makes
Challenger calibration measurably worse. Whatever most of that surplus is, the tier weight
is not the lever for it.

**The likeliest explanation is selection.** A player who earns promotion is one who has been
improving, and their rating is an average over the period they were still worse than they
now are. No static weight corrects for a player being better than their own history; it is
a form effect rather than a weighting one. That is a hypothesis this report cannot settle,
and it is why the weights stay where ADR-0004 put them: the change on offer trades a real
calibration cost for a partial fix to something the weights do not control.

Alternatives run without a rebuild, which is the reason the weights were made configuration:

```
validate --weights configs/weights.json
```

### A merge invalidates every rating

Identity reconciliation merging two rows moves matches between players, so the stored
ratings describe a database that no longer exists. `make rate` has to follow a reconcile
that merged anything. Fixing the 179 split ATP identities changed Sinner's overall rating
from 2678 over 44 matches to 2752 over 498, which is the difference between a fragment and
a career.

The validation figures barely move: 69.9% tour accuracy either way, and the pool mean shifts
by a point. A hundred and forty-nine careers out of seventy-five thousand players is not
visible in an aggregate, which is exactly why the aggregate could not have caught it.

## Player match history

`GET /players/:slug/matches` is the list a player page is mostly made of, so it was measured
against the longest career in the database: **Martina Navratilova, 1,735 matches**.

End to end over HTTP, warm:

| | |
|---|---|
| First page, 25 rows | 19ms |
| Filtered to one surface and season | 9ms |
| The profile endpoint on the same player, for comparison | 66ms |

The query itself, timed in the server, is 39ms for the first page, 26ms for a page reached
by cursor deep in the history, and 20ms filtered.

**Every page costs about the same, and that is the shape to know.** The ordering key
`played_on` lives on `matches` while the player is on `match_players`, so the database
fetches all 1,735 of a player's matches, sorts them, and returns 25. The cursor stops a
client paying more the deeper it pages — an OFFSET would — but it does not make the first
page cheaper.

**Cold, it is 590ms, and once it exceeded the API's 5-second statement timeout outright.**
That happened on the first request after the test suite had evicted the page cache, and it
is 1,735 primary-key lookups into `matches` at 0.24ms each when every one of them is a
disk read. Worth knowing before the first request after a deploy is the one a visitor makes.

If this needs to be faster, the fix is to carry `played_on` on `match_players` with an index
on `(player_id, played_on DESC, match_id DESC)`, turning the whole thing into a keyset scan
that reads 25 rows instead of 1,735. That is a schema change with an ingest cost, and 19ms
warm on the worst case in the database does not justify it yet. Recorded here so the next
person does not have to rediscover why it is the shape it is.

## Player ratings

Two queries, measured against the same career: Martina Navratilova, 807 rating rows across
five series.

| | Cold | Warm |
|---|---|---|
| Current and peak per series (on the profile) | 415ms | 9ms |
| A whole trajectory, 406 weekly points | — | 3ms |

Both read through the `ratings` primary key, which begins with `player_id`, so they touch
only that player's rows. The profile endpoint went from 66ms to 67ms with the ratings block
added — the cold figure is the first read of those pages off disk, the same shape as the
match history above.

The trajectory is deliberately not paginated. A chart wants the line, and a page of a line
is not one; 406 points is a few kilobytes, and the longest careers in the database are not
much longer.

**Peak is not the last row**, which is why both are served. Navratilova's overall Elo peaked
at 2920 in November 1984 — she went 86–1 that year — and her last rating, in 2005, is 2498.
A page that showed only the current figure would describe a different player.

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

## Clutch statistics and their baseline

Three figures — break points saved, tiebreaks won, deciding sets won — measured against
what the tour did over the same levels and the same decades.

Neither half can be a per-request query. Tiebreaks and deciding sets live in the score
string and nowhere else, so reading them means running the Go parser over 1.6 million
strings; the baseline is an aggregate over every appearance in the database. Both are
derived once, in the same family as the prominence columns above.

### Deriving the match columns

| | Matches | Wall time |
|---|---|---|
| First run, whole database | 1,567,729 derived | 67s |
| Every run after it | 0 derived | 4.1s |

The 4.1s is the baseline rebuild, which runs every time: one pass over 3.2 million
appearances joined to matches and tournaments, into 37 rows. The derivation itself finds
nothing to do, because the ingest writes the columns as it writes each match and the
backfill only exists for data loaded before the columns did.

Paged by id rather than by "still NULL". A score the parser cannot read stays NULL, so
selecting on the condition it fails to clear would be an infinite loop over exactly those
rows. `--stage refresh --force` re-derives everything, which is what a change to the
derivation itself needs — and one was needed: the first version required the tiebreak
points to be written down, and the files often did not write them. Bjorn Borg came out
with 35 tiebreaks in 764 matches. Reading the games instead — a set cannot be won by one
game any other way — gives 177.

### Reading it back

For Djokovic, 1,434 matches across 4 tiers and 3 decades:

| | Cold | Warm |
|---|---|---|
| Player query, planning included | 45ms | 20ms |
| `/players/:slug/clutch` end to end | 73ms | 42ms |

The endpoint runs three queries: the clutch aggregate, and the career summary and tier
splits that explain an absent break-points figure in the same words the profile uses. The
join to `clutch_baselines` is a sequential scan over 37 rows and costs nothing measurable.

### What the baseline actually says

Break points saved varies by era, which is why the cell has a decade in its key: ATP tour
level ran 59.3% in the 1990s and 60.8% in the 2020s.

Tiebreaks won and deciding sets won come out at **exactly 50.0%** in every cell, and that
is not a bug. Every tiebreak has a winner and a loser and both are in the same match, so a
population counting both sides wins exactly half of its own. The figures are stored rather
than assumed, so the page states a measured number and anything that breaks the symmetry
shows up instead of hiding — a test asserts it on every run.

## The read cache

Redis 7 has been in the compose stack since #2 and nothing used it. This data barely
changes -- it moves when an ingest runs, which is deliberate and infrequent -- so it is
unusually cacheable, and `ETag` revalidation has been in the API since #11 for the same
reason. The cache extends that idea to the server side.

Measured on the full database, with Postgres already warm, so the comparison is the cache
against a fair fight rather than against a cold buffer pool. Best of five each way.

| Endpoint | Uncached | Cached |
|---|---|---|
| `/coverage` | 220ms | 3.5ms |
| `/players/novak-djokovic` | 25ms | 2.9ms |
| `/players/novak-djokovic/clutch` | 29ms | 3.5ms |
| `/players/novak-djokovic/matches?limit=25` | 63ms | 3.2ms |
| `/h2h/bjorn-borg/john-mcenroe` | 9.5ms | 2.7ms |
| `/rankings?type=elo&tour=atp&limit=50` | 77ms | 3.1ms |
| `/rankings/trajectory?tour=atp&players=8` | 71ms | 2.9ms |

Every cached response lands in about 3ms regardless of what it cost to produce, which is
the shape you would expect: the work is a Redis round trip and a copy, and the endpoint it
came from stops mattering. The two that gain most are the two that read the most rows.

**Against a cold Postgres the gap is much wider** -- the same run with the buffer pool cold
measured 842ms for the Elo leaderboard and 450ms for a match page. That is the number a
first visitor after a restart would see, and the one the cache removes for everyone after
them.

**`/h2h` gains the least**, at 9.5ms uncached. One indexed query over a few dozen rows was
already fast, and caching it is worth about 7ms. It is cached anyway because it costs
nothing to include and the endpoint is one of the two the site is for.

### Invalidation is an event, not a timer

An ingest clears the cache when it finishes, which is the only moment the answers change.
Keys carry a 24-hour TTL, and that is a backstop rather than a freshness policy: it bounds
how long a failed flush can matter and stops keys nobody asks for accumulating. A flush is
a `SCAN` over the `deucepoint:v1:` prefix and an `UNLINK`, not `FLUSHDB`, because the Redis
may not be ours alone.

### A cache that is down costs time and nothing else

Every operation has a 50ms timeout and no retries, and every failure is a miss. Redis
being unreachable means the handler does the work it would have done anyway, and the
readiness probe still reports the process healthy -- a cache is not a dependency, and a
probe that failed on one would take a working process out of rotation in exchange for a
slower one. An integration test points the API at a dead port and asserts exactly that.

### The hit rate is reported, not assumed

`GET /api/v1/health` carries the counters:

```json
"cache": { "enabled": true, "hits": 91, "misses": 51, "errors": 0,
           "hit_rate": 0.64, "reachable": true }
```

and every response says which it was in `X-Cache: hit|miss`, so a single request can be
checked without reading an aggregate.

## Not measured

**Full rating recompute wall time**, which #20 also asks for. The rating engine does not
exist yet — it is Phase 2 — so there is nothing to time. The sizing analysis above is the
part that can be settled now, and it is the part that constrains the schema.
