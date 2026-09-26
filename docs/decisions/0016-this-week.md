# ADR-0016: This week, from the source's ongoing files

- **Status:** Accepted
- **Date:** 2026-09-26
- **Context:** the home page's "This week" card and ticker, and `GET /this-week`.

## Context

The weekly load brings a tournament in once its season file has it, so the home page could
show last week's finals and nothing of the week being played. The maintainer of layer D
(ADR-0002, amended) also publishes ongoing files: `ongoing_tourneys.csv` for the ATP,
`wta_ongoing_tourneys.csv` for the WTA and one for Challengers, in the season files' layout,
updated several times a day.

They are not live scores. They hold finished matches only, with no schedule and nothing in
progress. The ATP file dates each match by the day it was played and the WTA file by the
week's start. The WTA file writes rounds as words ("Quarterfinals"). Rows can still be
corrected or dropped until the event ends.

## Decision

**Kept apart from `matches`.** Ongoing rows go to `ongoing_matches`, and each fetch replaces
a file's rows in one transaction. Ingest only ever upserts: a row the source later dropped
or re-keyed would outlive it in `matches`, and ongoing matches dated to the week's start
would move `current_through` mid-week. Nothing that rates, ranks, aggregates or draws a
sheet reads `ongoing_matches`. The finished event reaches all of those the usual way, from
its season file, in the Monday load.

**Hourly, on its own.** `cmd/ongoing` runs from the `ongoing-hourly` CronJob. It sends the
stored ETag, so an unchanged file costs one 304. It runs none of ingest's refresh steps and
flushes no cache: `/this-week` sits outside the response cache because it reads a few dozen
rows.

**Tour level only.** Main-draw tour-level matches from the ATP and WTA files. The Challenger
file is not fetched, and team ties (levels D and T) and qualifying rounds are dropped. The
Laver Cup stays, because the season files already treat it as a tour event.

**Said as what it is.** The card says "results so far, not live scores" and how long ago the
files last changed, and that ratings take the results in once each event is over. Each event
shows the furthest round with a result in, or its champion once the final is in. An event
still listed a week after its last result is dropped as stale.

**Players linked where known.** A player links by source id: directly for one first seen in
the TennisMyLife files, through `player_aliases` for one the older files named differently.
An unknown id shows as a name without a link, never as a guessed match.

## Consequences

- The home page is current to within an hour of the source, instead of a week behind.
- The Challenger file and a partly played draw sheet are left out. The edition page would
  need to render unplayed rounds (ADR-0010's sheet grows its tree from the latest round),
  and that is a separate piece of work.
- Match dates mean different things per tour, so the page shows rounds, not dates.
