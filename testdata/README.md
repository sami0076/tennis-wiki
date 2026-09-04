# Seed fixture

A small slice of real match data so `make ingest` finishes in seconds and the site has
something honest to show on first boot. `make ingest-full` loads the real thing.

## Licence and attribution

**These files are adapted material under [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/).**

They derive from [Jeff Sackmann's Tennis Abstract](https://github.com/JeffSackmann)
datasets, by way of the mirrors recorded in
[ADR-0002](../docs/decisions/0002-data-sources-after-upstream-removal.md) and configured in
[`configs/sources.json`](../configs/sources.json).

ShareAlike permits redistributing them; the NonCommercial term means **this fixture may not
be used in any commercial context, ever**. It is committed as a demo aid, deliberately kept
small, and is not a redistribution of the dataset — the full data is ~1.6 million matches,
and this is roughly 0.25% of it. See [`DATA_LICENSE.md`](../DATA_LICENSE.md) for the full
provenance chain.

## What is in it, and why

The point is **depth of kind, not depth of volume**: a reviewer running `make seed` should
see all three data regimes immediately, and see the site handle each one honestly, rather
than discovering them after a twenty-minute ingest.

| file | matches | tournaments | serve statistics |
|---|---|---|---|
| `atp_matches_2019.csv` | 699 | 16 | 696 of 699 (100%) |
| `wta_matches_2019.csv` | 700 | 15 | 697 of 700 (100%) |
| `atp_matches_qual_chall_2019.csv` | 692 | 34 | 686 of 692 (99%) |
| `wta_matches_qual_itf_2019.csv` | 699 | 15 | 389 of 699 (56%) |
| `atp_matches_futures_2015.csv` | 682 | 22 | 0 of 682 (0%) |
| `atp_matches_1975.csv` | 699 | 24 | 0 of 699 (0%) |

### Players and rankings

| file | rows |
|---|---|
| `atp_players.csv` | 1129 |
| `wta_players.csv` | 511 |
| `atp_rankings_10s.csv`, `wta_rankings_10s.csv`, `atp_rankings_70s.csv` | 12032 |

Filtered to exactly the players the seed matches reference, so every biography and every
ranking belongs to someone with a page. Ranking history is sampled at one date a month —
weekly rankings for even this many players would dwarf every match file put together.

That covers every case the API's `serve.availability` field distinguishes:

- **`recorded`** — the 2019 tour and Challenger files
- **`partial`** — the WTA ITF file, where just over half the matches carry statistics
- **`never_recorded_for_tier`** — Futures, where no match in any year has ever recorded one
- **`never_recorded_in_era`** — 1975, before the tour recorded anything

The tour files include all four Grand Slams, so the players are recognisable rather than
being a random slice of qualifiers.

## How it was produced

Whole tournaments only. Truncating a file mid-draw would give players half a career and
make the seed misleading in exactly the way this site is meant not to be.

For each source file listed in `configs/sources.json`, the earliest tournaments were kept
until roughly 700 matches, Grand Slams first for the tour files. Seasons were chosen to
cover the four regimes above: 2019 for current data, 2015 for Futures, 1975 for the
pre-statistical era.

## Regenerating

Only necessary if the source files change or a new regime needs representing. Fetch the
season from the mirror in `configs/sources.json` and keep whole tournaments up to the row
cap. Do not simply `head` the file — that cuts draws in half.
