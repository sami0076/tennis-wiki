# Ingest fixtures

Small real samples used by the ingest tests. Tests never touch the network, so
these are the only source of truth for parser behaviour.

Five or seven rows each, chosen to cover the cases that matter:

| File | Covers |
|---|---|
| `atp_matches_2024.csv` | Sackmann layout, statistics present |
| `atp_matches_1969.csv` | Pre-1991: no statistics, no minutes |
| `wta_matches_2019.csv` | Same layout, other tour |
| `atp_matches_futures_2015.csv` | Futures: no statistics in any year |
| `atp_matches_qual_chall_2022.csv` | Qualifying rounds bundled with the main draw |
| `tml_2026.csv` | Different column order, extra `indoor`, alphanumeric player ids |

## Attribution

Derived from data compiled by **Jeff Sackmann / [Tennis Abstract](http://www.tennisabstract.com/)**,
licensed [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/).
`tml_2026.csv` is from [Tennismylife/TML-Database](https://github.com/Tennismylife/TML-Database),
built on the same structure.

These samples are redistributed under the same licence. See
[`DATA_LICENSE.md`](../../../DATA_LICENSE.md) — non-commercial use only.
