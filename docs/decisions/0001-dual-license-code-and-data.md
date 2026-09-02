# ADR-0001: Dual-license the repository — MIT for code, CC BY-NC-SA 4.0 for data

- **Status:** Accepted
- **Date:** 2026-09-02

## Context

The build specification (§3.3) states that the data license is non-negotiable: the source
data is CC BY-NC-SA 4.0, attribution is required, commercial use is not permitted, and
derivative work must carry the same license. The repository was initialised with an MIT
license, which permits commercial use without restriction.

These are in tension only if "the repository" is treated as a single work. It is not. It
contains two things with different authorship and different obligations:

1. **Source code** — Go services, React frontend, SQL migrations. Authored here. Not an
   adaptation of anyone's data. A parser is not a derivative of the thing it parses.
2. **Data** — ingested CSVs, the seed fixtures the specification requires us to commit
   (§11.2), the normalised database, and the Elo ratings computed from it. All of this
   *is* adapted material under CC BY-NC-SA 4.0, and ShareAlike binds it.

Applying one license to both either over-restricts the code or under-restricts the data.
Under-restricting the data is a license violation.

## Decision

Dual-license along the code/data boundary.

- `LICENSE` — MIT, scoped explicitly to source code, with a header pointing to the data
  license so nobody can reasonably miss it.
- `DATA_LICENSE.md` — CC BY-NC-SA 4.0, covering ingested data, derived datasets, computed
  ratings, seed fixtures, and API responses containing that data.

Attribution to Jeff Sackmann / Tennis Abstract appears in the site footer on every page,
in the README, and in `DATA_LICENSE.md`.

## Alternatives considered

**Relicense everything CC BY-NC-SA 4.0.** Rejected: Creative Commons explicitly recommends
against using CC licenses for software, because they do not address patent grants, source
distribution, or compatibility with existing open-source licenses. It would also make the
code unusable to anyone who wanted to learn from it, which defeats the portfolio purpose.

**AGPL-3.0 for the code**, to signal non-commercial intent throughout. Rejected for now:
AGPL restricts commercial use of the *code*, but the code is not the part carrying the
restriction. It would add copyleft obligations for readers of a portfolio project without
protecting the data any better than `DATA_LICENSE.md` already does. Worth revisiting if
the codebase becomes the valuable artifact rather than the site.

**MIT only, ignoring the data question.** Rejected. This is the violation the upstream
author has publicly complained about.

## Consequences

- Someone may take the code and build a commercial tennis site. They will have to source
  data themselves under terms permitting that; we grant them nothing. This is an
  acceptable and clean boundary.
- Every future data export, API response, or downloadable dataset inherits ShareAlike. Any
  feature that ships bulk data must carry the license notice.
- The non-commercial commitment is structural, not incidental: no ads, no paid tier, no
  sponsorship, for the life of the project. This is recorded in `DATA_LICENSE.md` as a
  binding constraint on the product, not a preference.
