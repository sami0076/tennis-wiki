# ADR-0008: Three published images, linux/amd64 only

- **Status:** Accepted
- **Date:** 2026-09-10
- **Context:** issue #99, and [#100](https://github.com/sami0076/tennis-wiki/issues/100),
  which pins the manifests to what this produces

## Context

`deploy/docker/api.Dockerfile` and `web/Dockerfile` both built, and `make site` ran the
whole stack from them, but nothing published anything. There was no artefact a cluster
could pull, and no answer to two questions the k3s manifests cannot be written without.

**What runs the migrations and the ingest?** The API image is `cmd/api` and its
certificates, deliberately — no toolchain, no source, no CSV reader. Loading 1.6 million
matches into an empty cluster needs `goose`, `cmd/ingest` and `cmd/rate`, and checking the
result afterwards needs `cmd/dataqual` and `cmd/validate`. None of them shipped anywhere.

**Which architecture?** The usual advice is to publish `linux/amd64` and `linux/arm64` and
let the puller choose, on the reasoning that Arm is the cheaper way to rent a server.

## Decision

**Three images on GHCR — `api`, `web` and `tools` — built for `linux/amd64` only, tagged
by short SHA and by semver on a release tag, never `latest`.**

**One tools image, not one per command.** `cmd/ingest`, `cmd/rate`, `cmd/dataqual` and
`cmd/validate` are one module, one compile pass and very nearly one set of compiled
packages. Four images would be four copies of the same layers in the registry, for an
isolation no operator gains anything from: they run one at a time, as Jobs, against the
same database, run by the same person. A Job picks a binary by `command`, not by `image`.
`goose` joins them because it is what applies the schema; it is pinned in the Makefile, so
CI and a developer's machine install the same one.

The image carries `migrations/` and `configs/` as well as the binaries, because `cmd/ingest`
resolves `configs/sources.json` relative to the working directory and goose is pointed at a
directory of SQL. That makes a tools image a self-contained statement of the schema and the
source registry at one commit, which is what a migration Job wants to pin to.

**`linux/amd64`, and no arm64 job.** The site runs on a Hetzner **CX33** — 4 vCPU, 8 GB,
80 GB NVMe, Intel. Hetzner's June 2026 price adjustment raised the Arm line harder than the
Intel one: **CAX21 is 10.49 EUR/month against CX33's 8.49** for identical specs, so the
advice above now has the arithmetic backwards for this host. One native runner, no buildx
emulation. **If the host ever changes, this paragraph is the one to revisit.**

**GHCR rather than Docker Hub.** Free for a public repository, no account or credential
beyond the `GITHUB_TOKEN` the workflow already has, no pull rate limit to plan around, and
it keeps the deployment inside the same permissions boundary as the code.

**No `latest` tag at all**, rather than one published alongside the immutable tags. A tag
that moves cannot say what is running and cannot be rolled back, and the failure mode is a
manifest that looks pinned and is not. The short SHA is the tag; #100 pins by digest, which
the publishing job prints.

## Why not the alternatives

**A fifth binary in the API image**, so one image does everything. It would put a CSV
reader, a rating engine and every migration inside the process serving public traffic, and
grow the image the site actually runs by the size of all of them. The API image is 29 MB
because it contains one thing.

**An init container running the migrations before the API rolls.** It couples a schema
change to a deploy in both directions: a migration that takes minutes blocks the rollout,
and a rollback of the Deployment does not roll back the schema. #100 makes it a Job for
that reason, and a Job needs an image it can name.

**Building the images on the server.** No registry, no CI, no cache — and no way to run the
same artefact twice or go back to the one that worked.

## Consequences

- The manifests in #100 pin by digest, and a rollback is picking an earlier one.
- A migration or a `configs/` edit produces a new tools image. The migration Job and the
  API Deployment are then two different commits' images unless they are rolled together,
  and they should be rolled together.
- Adding a command under `cmd/` means adding it to `tools.Dockerfile`, or it ships nowhere.
- The publishing job is gated on the paths filter, so a documentation change publishes
  nothing and a Go change does not rebuild the frontend.
- Image sizes are recorded in [`docs/performance.md`](../performance.md). A static binary
  that suddenly ships 900 MB means the Dockerfile is wrong, and the recorded number is how
  that gets noticed rather than discovered.
- An arm64 host would need the job changed, not merely re-run.
