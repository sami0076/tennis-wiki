# ADR-0009: Postgres in the cluster, on the node's own disk

- **Status:** Accepted
- **Date:** 2026-09-11
- **Context:** issue #100, [ADR-0008](0008-published-images-and-target-architecture.md)

## Context

The site needs a database that survives a pod restart, on a host chosen in ADR-0008: one
Hetzner CX33, 4 vCPU, 8 GB, 80 GB NVMe, 8.49 EUR/month.

The database is not small and not growing. `docs/performance.md` measures **2,054 MB**
before ratings and a **449 MB** `ratings` table after them — about 2.5 GB, and it stops
there, because ADR-0006's whole finding is that the sources have stopped moving. There is
no growth curve to plan for, only a fixed size to fit.

The default advice is to keep state out of Kubernetes and rent a managed database. Two
things make that advice not apply here:

**Hetzner does not sell one.** Managed Postgres is not in its catalogue, so "managed" means
a second provider, a second bill, and the database on the far side of the public internet
from the API that queries it — the same queries `docs/performance.md` times at 3–77ms
warm.

**Elsewhere, 2.5 GB does not fit the free tiers.** Neon and Supabase cap theirs at 0.5 GB,
and the paid tiers that would hold this database cost more per month than the entire server
(checked while writing #100). Paying several times the cost of the host to move 2.5 GB of
static data off it is not a trade worth making.

## Decision

**Postgres runs in the cluster: a single-replica StatefulSet with a PVC on the k3s
`local-path` storage class, which on a single node is that node's own NVMe.**

No Hetzner block volume — the node has 80 GB and the data is 2.5 GB, so a volume would be a
second line item for capacity already paid for. 20 GiB is claimed, which is the data, its
WAL, and room for a dump taken beside it.

**One replica, and no pretence otherwise.** `local-path` is node-local storage: the volume
cannot move, so a second replica could not schedule anywhere useful even if there were a
second node. What this buys is durability across pod and process restarts, which is the
actual requirement. It does not buy failover, and the manifests do not imply it does.

**The settings are sized, not copied.** The compose file carries a comment saying its
settings are not production settings. `shared_buffers=1GB` against a 3 GiB limit leaves the
rest of the container's memory to the page cache, which is what keeps a 2.5 GB database
resident — `docs/performance.md` measures the Elo leaderboard at **77ms warm against 842ms
with a cold buffer pool**, and that difference is the entire reason to care about sizing on
this box. `work_mem=32MB` is kept because it is the value every timing in that document was
measured with; lowering it would quietly invalidate the published numbers.

`synchronous_commit` goes back **on**. Compose turns it off to make a 1.6-million-row ingest
bearable on a laptop, which trades away committed transactions on a crash. That is the right
trade for a laptop and the wrong one for the live site.

## Why not the alternatives

**A managed database anyway, on a third provider's free tier.** 2.5 GB does not fit in
0.5 GB. Trimming the data to fit would mean giving up full-depth coverage, which is
[ADR-0003](0003-full-depth-player-coverage.md) and the reason the project exists.

**SQLite, or a file.** The read path is window functions over 5.1 million ranking rows and
3.2 million match-player rows, written as sqlc-generated Postgres queries per
[ADR-0005](0005-no-orm-sqlc-over-gorm-and-ent.md). This is not a port; it is a rewrite of
the part of the system that the performance document is about.

**Postgres on the host, outside Kubernetes, with the API in the cluster.** It would work,
and it splits the deployment into two things with two update stories, two backup stories and
two ways to be configured. Everything else about this site is in one place.

**A Hetzner block volume rather than local-path.** A monthly charge for capacity the node
already has, to gain a portability that a single-node cluster cannot use.

## Consequences

- **The node is the failure domain.** Losing it loses the database, and the answer is a
  restore, not a failover. #102 is where backup and recovery get written down; until then,
  the honest statement is that there is no tested restore.
- **Everything the data needs is reproducible**, which softens the above more than it
  looks: the schema is in `migrations/`, the sources are in `configs/sources.json`, and an
  hour of `jobs/load.yaml` rebuilds the database from nothing. A backup shortens the
  outage; it is not the only path back.
- **The memory budget has one big claimant.** Postgres requests 2 GiB and is limited to
  3 GiB of the node's 8 GB. The API (2 × 512 MiB), Redis (384 MiB) and the k3s control
  plane have to fit in what is left, which is what the requests and limits in
  `deploy/k8s/base/` are actually solving.
- **`/dev/shm` has to be set explicitly.** A container gets 64 MB, and Postgres puts
  parallel query workers there; at 1.6 million matches `cmd/dataqual` failed on exactly
  that. The StatefulSet mounts a 1 GiB in-memory `emptyDir`, which is the same fix the
  compose file's `shm_size` is.
- **Scaling out is a different decision, not this one.** If the site ever needs a second
  node or a replica, this ADR is superseded rather than amended: local-path is the part
  that would have to go first.
