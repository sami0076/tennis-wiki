# Deployment

How the site is deployed, how to change it, how to reload the data, and what it costs.
The manifests themselves are documented in [`deploy/k8s/README.md`](../deploy/k8s/README.md);
this page is everything around them.

## What is where

| | Where | What |
|---|---|---|
| `deucepoint.net` | Cloudflare Pages | The SPA, built from `web/` on every push to `main` |
| `api.deucepoint.net` | One VPS, k3s | The API, Redis and Postgres, from `deploy/k8s/` |
| Images | GHCR, public | `api`, `tools`, `web`, built by CI on `main`, pinned by digest |
| DNS and TLS | Cloudflare DNS; Let's Encrypt via cert-manager | `api` is an A record, DNS-only, so the certificate is issued on the node |

The host is a GreenCloud BudgetKVM in Staten Island: 4 EPYC Rome cores, 8 GB, 60 GB NVMe,
8 TB of transfer, Ubuntu 24.04, k3s v1.36. It replaced the Hetzner CX33 the Phase 4 issues
priced, same shape at 40% of the cost; ADR-0008 and ADR-0009 carry the amendment.

The topology is the one ADR-0009 chose: everything in one node's cluster, Postgres on the
node's own disk through `local-path`, the frontend deliberately outside the cluster on a CDN.

## The host, as it was set up

Done once, by hand, on 14 September 2026. Not in the manifests because k3s and cert-manager
are what the manifests assume rather than what they install.

```sh
# swap off for the kubelet; the 1 GB the panel created stays on disk, unused
swapoff -a && sed -i 's|^/swap.img|#/swap.img|' /etc/fstab
timedatectl set-timezone UTC

curl -sfL https://get.k3s.io | sh -                       # brings Traefik and local-path
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.2/cert-manager.yaml

ufw default deny incoming && ufw default allow outgoing
ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp
ufw allow from 10.42.0.0/16 && ufw allow from 10.43.0.0/16   # pods and services
ufw --force enable
```

SSH is key-only; the panel installed the key and disabled password login. The node's
hostname is `api.deucepoint.net`, which is also the node name k3s reports.

## Deploying the API

CI builds and pushes an image for every push to `main` that touches Go, the migrations or
the Dockerfiles. Nothing deploys it. Deploying is:

1. Take the digest from the CI log or the registry (`deploy/k8s/README.md` says how).
2. Put it in `base/40-api.yaml`, and in both Jobs if `tools` moved. Commit.
3. Copy and apply:

```sh
scp -r deploy/k8s deucepoint:/root/deploy/
ssh deucepoint kubectl apply -f /root/deploy/k8s/base/
ssh deucepoint kubectl -n deucepoint rollout status deploy/api
```

The Deployment rolls one pod at a time behind readiness, so the API stays up through it.
Rolling back is the previous digest and the same three commands.

If the change carries a migration, run the migrate Job first, then apply. The Job's
`kubectl wait` is in the README; an unmigrated database makes the new pods fail readiness
and the rollout stalls rather than serving errors, which is the intended failure.

## Deploying the frontend

Nothing to do. Cloudflare Pages watches `main`, runs `npm run build` in `web/` with
`VITE_API_BASE=https://api.deucepoint.net/api/v1`, and publishes `dist/`. Every pull
request gets a preview URL on the CI checks. A bad build does not replace the live one.

Two things live only in the Pages project settings and not in the repository: that
variable, and the custom domain.

## Reloading the data

The database is reproducible from public files, and that is the recovery plan (#102):
there is no backup to restore, because a rebuild produces the current schema from the
current sources in about an hour and a restore would produce an old one.

```sh
ssh deucepoint
kubectl -n deucepoint delete job load --ignore-not-found
kubectl -n deucepoint apply -f /root/deploy/k8s/jobs/load.yaml
kubectl -n deucepoint logs -f job/load -c ingest     # then -c rate
```

The Job is an init container running `ingest`, then a container running `rate`. Ingest is
idempotent and resumable: every file's ETag is in `ingest_files`, an unchanged file costs a
round trip, and a killed run picks up where it stopped. Ratings are recomputed from
scratch every time and never patched, so there is nothing to carry over.

**A partial refresh** — the current season from the live source — is the same Job. The
sources that changed are re-read; the 340 that did not are skipped. Run it whenever the
site should catch up; nothing schedules it yet.

**From empty** (a new node, a lost volume): the README's first-time sequence — secrets,
`base/`, migrate, load. The first `migrate` attempts fail while Postgres initialises its
volume and the Job retries; that is expected.

## Knowing it works

`deploy/smoke.sh` hits health, a player, a head-to-head and a simulation against a base
URL and fails on the first non-200. Run it after every deploy and every load:

```sh
deploy/smoke.sh https://api.deucepoint.net
```

`/api/v1/coverage` is the claim the README rests on; check it after a load and make sure
the dates are the ones the sources carry.

## Knowing it is down

Nothing inside the cluster can report that the cluster is gone, so the watcher is outside
it: the `Uptime` workflow (`.github/workflows/uptime.yml`) runs `deploy/uptime.sh` from a
GitHub runner every fifteen minutes. It asks `/api/v1/health` for `"database":"ok"` and
`deucepoint.net` for the page, retries once after thirty seconds so a blip is not a page,
and fails the run otherwise. GitHub emails a failed scheduled run to whoever last
committed the workflow file; that is the alert, and it reaches a phone.

Two things to know about it. A scheduled workflow is switched off after sixty days without
a push to the repository, and GitHub says so by email when it does; the `Run workflow`
button turns it back on. And the check is of the two public names, not the node — a check
that passes says the site is up, and one that fails says only that it is not.

## Runbook

**A pod is down.** `kubectl -n deucepoint get pods`. The API self-heals behind readiness;
Postgres is a StatefulSet and comes back on its volume; Redis comes back empty, which is
allowed — every cache miss is a query. If a pod is `Pending`, the node is out of memory:
`kubectl describe node` says which request did not fit.

**Disk is full.** `df -h /` on the node. Suspects in order: `/var/lib/rancher/k3s` (old
images — `k3s crictl rmi --prune`), the Postgres volume under `/var/lib/rancher/k3s/storage`
(a load doubles the database for its duration; 2.5 GB steady), and journald.

**An ingest stopped half way.** Re-apply the load Job. It resumes from the ledger; nothing
half-written survives, because each batch is a transaction.

**The certificate is expiring.** cert-manager renews thirty days out and writes to
`samisaleh07@gmail.com` if it cannot. `kubectl -n deucepoint get certificate` shows `Ready`;
`kubectl -n deucepoint get challenge` shows what is stuck. The HTTP-01 challenge needs port
80 open and `api.deucepoint.net` resolving to the node, DNS-only, not proxied.

**Logs** without a shell on a node are #102's open item; today it is
`ssh deucepoint kubectl -n deucepoint logs deploy/api`.

## What it costs

| | | |
|---|---|---|
| VPS | GreenCloud BudgetKVM NYC, annual | **$45.00 / year** |
| Domain | `deucepoint.net`, Cloudflare Registrar, at cost | **$11.86 / year** |
| Pages, GHCR, DNS, certificates | | 0 |

**$56.86 a year, $4.74 a month.** The Phase 4 issues estimated about 9 EUR a month on
Hetzner; this is half that. No part of it scales with visitors. The VPS is prepaid and non-refundable; if the project
ended tomorrow, the year is the loss, and the box is a general-purpose 8 GB machine until
then.
