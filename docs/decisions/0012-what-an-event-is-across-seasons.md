# ADR-0012: An event is keyed by the tour's number where there is one, and by its name where there is not

- **Status:** Accepted
- **Date:** 2026-09-16
- **Context:** issue #121, the first of Phase 5 (#131); three page issues (#122, #123,
  #128) and the structured data (#130) address an edition and depend on this

## Context

`tournaments` has held a row per event per season since migration 00003, and
`simulate/draw` reconstructs a bracket from one, but nothing in the schema says which rows
are the same event in different years. `source_id` is the raw `tourney_id` of whichever
file the row came from, and the only stable part of it — when there is one — is what
follows the season. The question is what the identity of an event across seasons is, and
the answer has to be counted before it is chosen, because the cases that do not fit are
not the exceptions: they are most of the women's tour.

Everything below was measured on the full database as loaded on 16 September 2026 —
64,626 tournament rows (28,865 ATP, 35,761 WTA) over 1,654,193 matches, the same sources
and the same load the live site runs. The counts that the rule depends on staying true
become `cmd/dataqual` checks in the consequences; the rest were one-off SQL.

### What an id looks like

Five families, and a tour is not in the same family for its whole history.

| Family | Shape | ATP rows | WTA rows | Carries an identity across seasons? |
|---|---|---|---|---|
| The tour's number | `2019-580`, `2024-0421` | 10,302 (418,050 matches) | 17,314 | ATP: yes, every era. WTA: **only from 2016** |
| Sackmann's M-codes, 2016–20 | `2016-M006` | 58 | 56 | Within the five years, yes; to the number before and after only through the name |
| ITF-style | `1998-M-FU-ARG-01A-1998`, `2009-W-INT-AUS-01A-2009` | 14,396 (446,201 matches) | 12,723 | By construction, no: country plus a sequence number within the year |
| A team tie | `2019-M-DC-2019-FLS-A-M-FRA-JPN-01`, `1968-D001` | 4,069 (15,123 matches) | 5,612 (12,400 matches) | One row per tie; the competition is in the name |
| Leftovers | `1968-T101` (39 rows, 1967–69), `1978-TLC-1978`, `2016-O16` (both tours) | 40 | 10 | No |

**The ATP number is an identity in every era.** Of consecutive-season pairs of the same
number on the tour tier, the name is unchanged in 84% of them in 1965–69, 91% by the late
1970s, and 97–99% from 2000 on. It survives the change of source: 223 numbers appear in
both Sackmann's 2024 file and Tennismylife's 2025 file, and the 168 whose names differ
are almost all Sackmann's ` CH` suffix (`Troyes CH` → `Troyes`). Eleven tour-level
numbers stop at 2024 (Newport, Hong Kong, Estoril, Lyon, Cordoba, Atlanta, Belgrade, the
Paris Olympics, and the old Brisbane and United Cup ids) and one starts in 2025 (Athens);
that is the calendar changing, not the numbering.

**When the number changes owner, it is the ATP's own idea of the event.** 356 times
across 226 ATP numbers, consecutive editions share no word of their name: 306 is Bari
(1989), Genova, St. Pölten, Pörtschach and then Kitzbühel (2009–); 424 is Berkeley,
Albany, San Francisco, San Jose, New York and Dallas (2022–); 404 is Palm Springs, Rancho
Mirage, La Quinta and Indian Wells; 403 is Delray Beach, Boca West, Key Biscayne and Miami.
The number is the sanction, and the sanction moves. The ATP's own site treats Dallas as
the continuation of San Jose, and this ADR does too.

**The WTA number is a per-season sequence number until 1987.** In every season from 1922
to 1979 the numeric ids on the women's tour are a contiguous block — 1001, 1002, … 1094 —
99–100% of the span between the smallest and largest id is occupied. Key `1056` has 59
different names over 67 seasons. Of consecutive-season pairs of the same number, the name
is unchanged in 1–2% of them in every five-year window through 1984, and the few dozen
numbered rows of 1984–87 are exhibitions, the Wightman Cup and the Wimbledon Plate, still
numbered in sequence. The women's Australian Championships sit under twenty-two
different numbers between 1923 and 1967; Wimbledon under thirty-six. That is 15,471 rows
(15,115 tour, 356 Challenger) and 175,621 matches with no identity in the id at all.

**From 1988 to 2015 the women's tour has almost no number.** Four numeric tour-tier rows
in twenty-eight seasons. The tour is filed under ITF-style ids (1,855 rows, 148,222
matches), and the middle of those — category, country, sequence — holds only for events
that are the only one of their category in their country: Wimbledon is `W-SL-GBR-01A` and
the US Open `W-SL-USA-01A` for all 48 editions from 1968 to 2015; everything else shifts
as the calendar does, and name continuity across the family is 68%. The 1,309 numeric
Challenger rows of 1988–95 (`1991-0297 ITF Schwarzach`) are the ITF circuit's own
numbering, with 73% name continuity — and a space that collides with the WTA's: 540 is
ITF Indianapolis in 1991 and Wimbledon in 2016, 560 is Haskovo and then the US Open. Four
numbers collide, all four of them Slams, so those rows cannot be numbers either.

**From 2016 the WTA number is the WTA's own, except where Sackmann used the ATP's.**
Sackmann's 2016–21 files carry the WTA's numbering (1003 Doha, 1017 Cincinnati, 806
Canada) and Tennismylife continues it: of the 34 numbers in both the 2021 and the 2022
file, 29 keep their name and five drift (Gdynia → Warsaw, Montreal → Toronto, Guadalajara
Finals → Fort Worth Finals). But Sackmann filed the four women's Slams under the men's
numbers — 580, 520, 540, 560 — and Tennismylife uses the WTA's: 901, 903, 904, 905. Of the
83 numbers Tennismylife has used since 2022, 46 match a Sackmann row by number and name,
6 by number with a drifted name, 11 by name under a different number (the four Slams,
Budapest 2036 → 578, Adelaide 2030 → 2014, Guadalajara 2002 → 2075, Abu Dhabi 2028 → 2088,
Ostrava 2025 → 1154, and two that are different events in the same city), and 22 are new.

**Sackmann's M-codes cut a five-year hole in the biggest events.** For 2016–2020 both
tours' files use `M` plus a serial for events whose number Sackmann did not carry:
Indian Wells is `M006`, Miami `M007`, Rome `M009`, Madrid `M021`, Cincinnati `M024` —
the Masters 1000s, exactly the pages a visitor opens first. The codes are stable within
the five years (`M006` is Indian Wells every year) and are 58 ATP and 56 WTA rows.

**Futures and ITF have a slot, not a place.** An ATP Futures id is `M-FU-ARG-01A`, and its
name is `Argentina F1`: the slot *is* the name, and it holds across seasons 92% of the
time even though the F1 of one year is often in a different city from the last. 12,771 of
the 13,859 Futures rows are under a name seen in more than one season. The women's ITF
circuit names the city and the prize band instead — `Antalya 10K`, `Antalya $10K`,
`W15 Antalya` — so the slot holds only 40% of the time and the name is the better key,
with the naming conventions of different eras splitting one city into several runs.

**The season in the id is the tour's season; the year of the first match is not.** 371
rows (103 ATP, 268 WTA) have an id whose year differs from `season`, which ingest sets
from the start date. `1969-243 Perth` began on 30 December 1968; `1985-605 Masters` was
played in January 1986; Adelaide, Doha, Chennai and Brisbane each have a season with two
"editions" for this reason, and the 1986 calendar year has two Masters — the 1985 one in
January and the 1986 one in December — which are two seasons, not one. Seven rows have no
year in the id at all (`W-FC-2026-QLS-M-AUS-GBR-01`).

**A name is not an identity either.** Since 1990, 222 normalised Challenger names sit
under more than one ATP number (2,229 rows): Sao Paulo under eight, Prague under six. Of
605 pairs of ATP Challenger numbers that share a name, 140 are concurrent — Oeiras 1
through 5 in one season, six Hersonissos events in 2025 — and 465 are sequential, a city
whose sanction number changed. Grouping by name would merge the concurrent ones and could
not tell the sequential ones from a rename.

**The tours do not share a number.** The only numbers common to both tours with the same
name are 520, 540, 560 and 580 — the Slams, by Sackmann's borrowing, for six seasons — and
Tennismylife has already undone it. Indian Wells is 404 on one tour and 609 on the other.
There is nothing in the data that pairs the men's and women's edition of a combined event.

## Decision

**An event is a row in a new `events` table, every `tournaments` row points at exactly one,
and the table is derived from the data by a rule, with a short overrides file for what the
rule cannot know.** It is rebuilt by `cmd/ingest -stage events`, which is idempotent and
runs after the match stage in the load job, the way the reconcile stage does for players.

### The key

For each `tournaments` row, in this order, the first that applies:

1. **An override.** `configs/event_overrides.json` names rows — one number, one of
   Sackmann's M-codes, one exact id, or a list of ids — and the number the event is filed
   under. It holds the renumberings the data shows and a person has checked: the women's
   Slams (580 → 901, 520 → 903, 540 → 904, 560 → 905), the Olympics (ATP 84, 88, 92 and
   `O16` → 96; WTA `O16`, `1924-1181` and the city-named `W-OL` rows → 650), the six WTA
   renumberings above, the seven ATP M-codes whose name sits under several numbers
   (Sydney, Acapulco, Eastbourne, Brisbane, Washington, Rio, Adelaide: 29 rows), and the
   WTA's year-end championships of 1975–2013 — forty ids under five names, filed under
   808, because the ATP's equivalent is one number since 1970 and the women's would
   otherwise be five pages. Each entry carries a reason. Thirty entries; an override is
   for a number that changed, never for a name.
2. **The tour's number**, where the id is `YYYY-N` and the number is a real one: every
   ATP row, and WTA rows from 2016. The key is `(tour, N)` with leading zeros dropped, so
   `2018-0451` and `2019-451` are one event.
3. **The name**, normalised, within the tour and the tier: everything else — the women's
   tour before 2016, Futures, ITF, the M-codes, the 1968 T-codes. A name-keyed run
   **bridges** to a numbered event when exactly one numbered event of the same tour and
   tier has an edition under that normalised name; if none has, the run is an event of
   its own; if more than one has, it stays its own rather than guess. As implemented,
   with the overrides applied first: on the women's tour tier, 17,314 name-keyed rows, of
   which 2,378 bridge and 14,936 are events of their own; fifteen names across both
   tours stay apart because the name is under two numbers (Melbourne, Budapest, Chicago,
   Tokyo, Istanbul, Charleston — each a city with two events since 2016). The women's
   Wimbledon of 1923–2015 (87 editions), Roland Garros (82), the US Open (48) and the
   Australian Open (47) all bridge, and so do Sydney (91), Lausanne (86) and Eastbourne
   (84), which is the rule's reach and its risk in one line. All but seven of the
   M-codes (`Indian Wells Masters` → 404) resolve here without an override. Without the
   bridge the women's Slams are two pages each, and the site's "both tours on the same
   footing" is false on the page a visitor opens first.
4. **Team ties** key on the competition, which is the name before the colon — Davis Cup,
   Fed Cup, Billie Jean King Cup, United Cup, ATP Cup — within the tour. An edition of a
   team competition is a season and holds its ties; it is not a draw, and #122 decides
   what its page is.

Normalising a name lowercases it, strips Sackmann's ` CH` suffix and a trailing edition
number (`Oeiras 2`), and folds punctuation, so `Punta Del Este CH` and `Punta del Este`
are one name and `Antalya $10K` and `Antalya 10K` are one name. It does not strip a
category prefix (`W15 Antalya`), because the category is part of what the event was.

### What the row keeps

`tournaments.event_id` points at the event, and `tournaments.event_link` says how the
row got there: `number`, `override`, `name`, `bridged`, or `team`. The column is nullable
in the schema because the match stage writes the row and the events stage keys it; a row
still null after a load is a `dataqual` integrity failure. **The link is provenance, and
the page prints it** — an event page whose early editions were joined by name says so, in the same
place a serve-statistics table says which matches it is missing. This is the standing
rule, absent is not zero, applied to identity: a row on an event page by exact number and
a row on it by name are two different claims, and the reader gets to see which.

### The season

An edition's season is the year in its id, not the year of its first match; ingest sets
`tournaments.season` from the id where it has one and falls back to the start date for the
seven rows that do not. The 371 rows move to the season the tour says they were in, the
double editions of Adelaide, Doha, Perth and the 1985–86 Masters resolve, and every reader
of `tournaments.season` — the match log's season filter, the simulator's event lookup, the
decade split on the player page — gets the tour's season for free.

### The slug

`events.slug` is minted from the event's display name and **always ends in the tour**:
`wimbledon-wta`, `indian-wells-atp`, `argentina-f1-atp`. Nearly every tour-level event
exists on both tours under the same name, so a rule that added the tour only on collision
would add it almost everywhere and unpredictably; adding it always is predictable, says
on the URL which draw sheet the reader is on, and leaves the bare `wimbledon` unclaimed
for a combined page if one is ever built. A name that already carries the tour is not
suffixed again: `wta-finals`, `next-gen-atp-finals`. Collisions inside a tour — the eight
Sao Paulo Challengers, the 465 sequential ones — get a serial, `sao-paulo-atp-2`, the run
still on the calendar (then the longer one) taking the bare slug, so the current Acapulco
is `acapulco-atp` and the 1974 one-off the serial; 676 events carry one, twelve of them
current, and those are cities with two events in one season.

The display name is the name of the event's most recent edition, unless the overrides
file pins one: Canada is `Canada Masters` in every ATP file, but the WTA's 806 alternates
`Montreal` and `Toronto` by the year, and the slug has to be neither. The slug is minted
once and kept: when 424 moves again, `dallas-atp` stays, the page shows the current name
and the run of former ones, and the URL is a handle rather than a label. A full reload
mints from the latest name at that time, so a sanction that moved city between two full
loads gets a new slug and the old one stops resolving; a handful of events a year move,
and that is cheaper than a redirect table nobody would maintain.

### The endpoints

As #121 has them, with the slug carrying the tour:

```
GET /api/v1/tournaments?tour=&level=&q=
GET /api/v1/tournaments/{slug}
GET /api/v1/tournaments/{slug}/{season}
```

`simulate/draw` addresses an edition by the same `slug` and `season`, replacing the
`tour`, `season`, `event` name triple, so the simulator and the sheet agree on what an
event is; the name form is kept for one release as an alias and then removed.

## Why not the alternatives

**A derived slug column and no table.** Enough for the ATP, where the number is always
there. It has nowhere to put the display name, the pinned names, the bridge, or the
provenance, and the WTA needs all four.

**Group by name everywhere and ignore the number.** Merges the 140 concurrent Challenger
pairs, splits every sanction that moved city into a page per city, and throws away the
one identity the ATP actually maintains.

**Group by number everywhere and leave the rest as single-edition events.** Honest, and
what the ATP would get anyway. It leaves the women's tour before 2016 — 94 seasons, all
of the WTA's history the site holds except the last ten years — as some 17,000 events of
one edition each, and puts the women's Wimbledon on two pages. The site's reason to exist is
that it does not treat the women's tour as an afterthought.

**Curate the mapping by hand.** Some 2,300 numbered events across the tours and 5,000-odd
names on the women's tour alone. The overrides file is for what the rule cannot know — a number that changed — and
is kept small enough to read; the rest is a rule, so a reload and a new source give the
same answer without a person.

**Bridge by name across tiers, or to the nearest numbered event in time.** More matches,
and a Futures `Antalya` would attach to the `Antalya CH`. A bridge is a claim that two
runs are one event; within tour and tier, to exactly one candidate, is the claim the data
supports, and the page says when it was made.

**The tour as a path segment, or the bare slug for whichever tour is loaded first.**
`/tournaments/atp/wimbledon` is a fine URL and would be the choice on a fresh site.
Player URLs are `/players/{slug}` with the tour folded into the slug on collision, and
giving the bare slug to the first tour loaded would make the ATP the default in every
URL, which is the asymmetry the project exists to refuse. The suffix on every event slug
is the smallest rule that is fair to both tours and consistent with the player URLs.

**One event across both tours.** The right product notion for a combined event, and the
data has no link for it: nothing pairs 404 with 609. A pairing would be a curated table
of a few hundred rows, is its own issue, and the bare slug is left free for it.

## Consequences

- Migration 00016: `events` (`id`, `tour`, `slug` unique, `name`, `key`, `first_season`,
  `last_season`), `tournaments.event_id`, `tournaments.event_link`; it also moves the 371
  rows to the season in their id and merges the two United Cup editions the old rule had
  split across New Year. The ingest match stage sets `season` from the id from then on.
  On today's data the rule yields 1,586 numbered, 2,003 named and 1 team event on the
  ATP, and 143 numbered, 6,721 named and 8 team events on the WTA; by row, the ATP is
  10,299 by number, 26 by override, 42 bridged, 14,429 by name and 4,069 team ties, and
  the WTA is 540 by number, 79 by override, 2,394 bridged, 27,083 by name and 5,663
  ties. Those are the proportions the page's provenance line will show, and they are
  what "the women's tour before 2016 is joined by name" means in numbers.
- `configs/event_overrides.json`, loaded by the new stage the way `player_overrides.json`
  is; `cmd/dataqual` reports every override that matched no row, so a renumbering that
  a source undoes does not sit in the file forever.
- `cmd/ingest -stage events` derives the table in nine seconds; a full load runs it after
  the matches, and `deploy/k8s/jobs/events.yaml` runs it alone after a change to the
  overrides. `dataqual` checks that no row is left unkeyed and no event is left empty,
  and reports the editions joined by name, the bridged ones, and the events named after
  a serial, so the rule's reach is a number rather than a guess.
- The three endpoints, with ETag and cache behaviour matching the rest of `v1`;
  `FindTournament` is replaced by a lookup on `(slug, season)`. `docs/performance.md`
  carries the measured cost of the edition endpoint on a 128 draw, cold and warm.
- Every tournament name on every page becomes a link once #122 lands; team ties link to
  their competition's season, whose page is #122's to decide.
- Out of scope, and left for their own issues: pairing the men's and women's editions of
  a combined event; linking an event to the tour's official page by its number.
