# Data license and attribution

The **code** in this repository is licensed under the MIT License — see [`LICENSE`](LICENSE).

This file governs the **data**: everything ingested by this project, every dataset derived
from it (including the computed Elo ratings and the seed fixtures committed under
`testdata/`), and any export or API response served from it.

---

## License

All data in and derived from this project is licensed under

**[Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International (CC BY-NC-SA 4.0)](https://creativecommons.org/licenses/by-nc-sa/4.0/)**

[![CC BY-NC-SA 4.0](https://licensebuttons.net/l/by-nc-sa/4.0/88x31.png)](https://creativecommons.org/licenses/by-nc-sa/4.0/)

You are free to **share** (copy and redistribute in any medium or format) and **adapt**
(remix, transform, and build upon the material), under the following terms:

| Term | What it means here |
|---|---|
| **BY** — Attribution | You must credit Jeff Sackmann / Tennis Abstract, link to the license, and indicate if changes were made. |
| **NC** — NonCommercial | You may not use the material for commercial purposes. |
| **SA** — ShareAlike | If you remix or build upon the material, you must distribute your contributions under this same license. |

## Attribution

> Tennis match data originally compiled by **Jeff Sackmann** /
> **[Tennis Abstract](http://www.tennisabstract.com/)**, licensed under
> [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/).
> Shot-by-shot data from the
> [Tennis Abstract Match Charting Project](https://github.com/JeffSackmann/tennis_MatchChartingProject),
> a crowdsourced effort by dozens of contributors, same license.
> This project has modified the data: it is reshaped from a winner/loser row format into a
> normalised relational schema, and Elo ratings are computed from it.

This attribution appears in the footer of every page on the live site, as required.

## Provenance — read this before changing an ingest source

The build specification names `JeffSackmann/tennis_atp` and `JeffSackmann/tennis_wta` as
the primary sources. **As of 2 September 2026 both repositories return HTTP 404.** The
GitHub API reports `public_repos: 1` for that account; the only surviving repository is
`tennis_MatchChartingProject`. No public statement explaining the removal has been found.

That surviving repository's README carries this notice from its author:

> "I'm serious about the license, and I'm really disappointed with the handful of people
> who have chosen to violate it. If violations continue, I may stop updating the repo
> entirely."

The removal of the two match repositories should be read in that light. This project
therefore commits to the following, and these are not negotiable:

1. **No commercial use, ever.** No ads, no sponsorship, no paid tier, no affiliate links,
   no "pro" features. This is what makes our use of the data legitimate.
2. **Attribution to the originator**, not to the mirror. Mirrors are a distribution
   channel; Jeff Sackmann and the Match Charting Project contributors are the authors.
3. **ShareAlike propagated.** Our derived data carries this same license.
4. **Bulk redistribution is not a feature.** We serve statistics and analysis, not a
   competing copy of the raw dataset.
5. **If the author asks us to stop, we stop.** Contact details are on
   [tennisabstract.com](http://www.tennisabstract.com/). Compliance is not conditional on
   being legally compelled.

### Sources actually used

CC BY-NC-SA 4.0 permits redistribution, so ingesting from a compliant mirror is within the
license. Every figure below was measured against the real files. See
[ADR-0002](docs/decisions/0002-data-sources-after-upstream-removal.md) for the source
layering and [ADR-0003](docs/decisions/0003-full-depth-player-coverage.md) for the decision
to cover every tier.

| Layer | Source | Tours | Tiers | Coverage |
|---|---|---|---|---|
| A | Complete Sackmann snapshot | ATP + WTA | tour, qualifying+Challenger, Futures, qualifying+ITF, doubles, amateur | → 2022-01-10 |
| B | Restructured ATP mirror | ATP | tour, qualifying+Challenger | → 2024 |
| C | Vendored WTA snapshots | WTA | tour | → 2024 |
| D | [`Tennismylife/TML-Database`](https://github.com/Tennismylife/TML-Database) | ATP | tour | 2025 → 2026-01-17 |
| E | [`JeffSackmann/tennis_MatchChartingProject`](https://github.com/JeffSackmann/tennis_MatchChartingProject) | ATP + WTA | charted matches | → 2026-05-24 |

Layer A is the **only** located source for Futures, WTA ITF, and doubles, and its WTA
records reach back to **1923**. Layers B–D are fresher but carry only the upper tiers.

### About the Match Charting Project (layer E)

Worth stating plainly, because it is easy to misread its role: **`tennis_MatchChartingProject`
is public, intact, and actively maintained.** It is the one Sackmann repository that
survived, and it is also **the most current source available to this project** — charted
through 24 May 2026, ahead of every other source.

It cannot substitute for the match database. It holds 7,566 ATP and 4,080 WTA matches
against roughly 195,000 ATP tour-level matches alone — 3.9% coverage, skewed heavily toward
famous players and recent decades, with 1,002 ATP and 731 WTA distinct players. Its
`charting-*-matches.csv` files carry **no winner, no score, and no player IDs**; results
must be derived by replaying the point-by-point files.

What it uniquely provides:

- **Shot-by-shot data** — shot type, direction, depth, error type. Nothing else public
  has this.
- **Per-set as well as per-match aggregates** in `charting-*-stats-Overview.csv`, including
  `serve_pts`, `aces`, `dfs`, `first_in`, `first_won`, `second_won`, `bp_saved`, and
  `return_pts_won`. This is a superset of what the tour CSVs carry, and it is exactly the
  input the simulation engine needs.
- **Currency for elite players** in the window where every other source has run out.

It is crowdsourced by volunteers, one match at a time. The license notice above is not
boilerplate: thousands of person-hours went into it.

### Known coverage gaps

These are real and must be disclosed on the `/methodology` page rather than papered over:

- **Futures and ITF have no serve statistics in any year.** ~935,000 matches of results,
  scores, and rankings with no point-level data. Sufficient for win/loss, head-to-head, and
  Elo; **insufficient for the simulator**.
- **Challenger-level statistics begin around 2010.** 0% coverage in 2005, 84% by 2015, 99.7%
  by 2022.
- **WTA data is historically poorer than ATP.** 19% serve-stat coverage in 2005 against
  ATP's 89%, converging to 89% by 2022. WTA parity means parity of *treatment*; the data
  gap is a fact about the sources and the site says so.
- **Lower tiers stop at 2021.** Complete through the 2021 season; no located source after.
- **ATP tour: 17 Jan 2026 → present.** No full-schema source located.
- **WTA tour: 2025 → present.** No full-schema source located.
- **Match statistics before roughly 1991** do not exist in any source at any tier. Results,
  scores, and rankings only. The UI must explain this rather than render zeros.

## If you are reusing this project

You may. Under ShareAlike you must license your derived data under CC BY-NC-SA 4.0, you
must attribute Jeff Sackmann / Tennis Abstract, and you may not use it commercially. If
you intend a commercial product, you need to obtain the data under different terms —
this project cannot grant them to you.
