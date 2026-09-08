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
| players rated | 75,128 |
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

**The pool mean sits at 1684, not 1500.** K decays per player, so a debutant beating a
veteran gains more than the veteran loses and the pool's total rating is not conserved.
Whether that inflation is acceptable, and what the tour-level mean should be alongside it,
is exactly what the mean-reversion check in #42 is for; it is recorded here as an
observation, not settled as a result. The names it produces are the right ones — Graf,
Navratilova, Seles, Evert, Djokovic at the top of the all-time peaks — which says the
replay is sane, not that the weights are calibrated.

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

## Not measured

**Full rating recompute wall time**, which #20 also asks for. The rating engine does not
exist yet — it is Phase 2 — so there is nothing to time. The sizing analysis above is the
part that can be settled now, and it is the part that constrains the schema.
