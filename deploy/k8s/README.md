# Deploying Deucepoint

Manifests for a single-node k3s cluster on one VPS. What runs here is the
API, Redis and Postgres — three things, not four. The frontend is static files on
a CDN and is deliberately not in the cluster.

This directory is the manifests. The host, the deploy and the runbook are
[`docs/deployment.md`](../../docs/deployment.md).

```
base/     applied together, in filename order
jobs/     applied by hand, one at a time
secrets.example.yaml   copied to secrets.yaml and filled in; never committed
```

`base/` is its own directory so `kubectl apply -f` over it cannot pick up the
placeholder Secret or start an hour-long ingest by accident. The one scheduled
thing, `base/60-load-weekly.yaml`, is in there because a CronJob is state the
cluster should hold, not a run to start.

## Before anything

Three things this directory assumes and does not install: **k3s** (which brings
Traefik and the `local-path` storage class), **cert-manager**, and a DNS record
pointing at the node.

The name is `api.deucepoint.net`, in `base/10-config.yaml` (the CORS origin is
the site, `deucepoint.net`) and `base/50-ingress.yaml`. Both files change
together if it ever moves.

## First time

```sh
cp deploy/k8s/secrets.example.yaml secrets.yaml
# fill in a generated password, in both fields
kubectl apply -f deploy/k8s/base/
kubectl apply -f secrets.yaml

kubectl -n deucepoint apply -f deploy/k8s/jobs/migrate.yaml
kubectl -n deucepoint wait --for=condition=complete job/migrate --timeout=5m

kubectl -n deucepoint apply -f deploy/k8s/jobs/load.yaml
kubectl -n deucepoint logs -f job/load -c ingest
```

The API pods crash-loop until the migration has run — they open the pool and
verify it at startup, and an unmigrated database fails that check. That is the
intended order, not a fault. The migrate Job's first attempts fail the same way
while Postgres initialises its volume; it retries on its own, and the `wait`
above covers it.

The load takes about an hour: roughly 40 minutes for the match stage, 23 for the
reference stage, then 1m 36s for the ratings. Every figure here is from
[`docs/performance.md`](../../docs/performance.md).

## The pinned digests lag the repository by one commit

This is inherent to pinning in-tree: the image for a commit does not exist until
CI has built that commit, so a manifest can only name an image built from an
earlier one. CI builds `api` and `tools` only when Go, the migrations or the
Dockerfiles change, so the newest image is usually older than `main`; that is
fine as long as it is not older than what the manifests ask of it.

## Rolling out a new image

Images are pinned by digest, not by tag, so the manifest alone answers what is
running. The publishing job in `ci.yml` prints the digest for every image it
pushes; take it from there, or ask the registry:

```sh
docker buildx imagetools inspect ghcr.io/sami0076/tennis-wiki/api:<short-sha> \
  --format '{{.Manifest.Digest}}'
```

Then edit `base/40-api.yaml`, and the Jobs plus `base/60-load-weekly.yaml` if
the tools image moved. Rolling back is putting the previous digest back and
applying again.

## Re-running things

A Job's pod template is immutable, so applying over a finished one is rejected
rather than re-run. Delete first:

```sh
kubectl -n deucepoint delete job migrate --ignore-not-found
kubectl -n deucepoint apply -f deploy/k8s/jobs/migrate.yaml
```

All six Jobs clean themselves up 24 hours after finishing. `jobs/reconcile.yaml` is
identity reconciliation and the ratings without the load in front of them, for after a
scoring change; its header says how to dry-run it first. `jobs/events.yaml` is the
events stage on its own, for after a change to `configs/event_overrides.json`.

`jobs/load-force.yaml` is `jobs/load.yaml` with `--force`, for when the database is
missing rows the sources do carry. The ordinary ingest asks each source conditionally
and skips a 304, so a file the ledger recorded from a run that read a shorter version of
it is never looked at again, and the season it was mid-way through stays mid-way through.
Nothing else here can break that: no other Job, and not the CronJob, passes `--force`.
Its header says how to narrow the run to one tour and a season or two, which is minutes
rather than the hour a full forced read takes.

The weekly CronJob needs none of this -- it mints a new Job each Monday. To run it
off-schedule: `kubectl -n deucepoint create job --from=cronjob/load-weekly catchup-now`.

## What the probes mean

`/api/v1/health` round-trips a real query and is wired to **readiness**: a pod
pointed at an unreachable or unmigrated database takes itself out of rotation
instead of serving errors.

`/api/v1/live` touches nothing and is wired to **liveness**. The distinction is
the point — `/health` on a liveness probe would turn a five-second Postgres
hiccup into a restart of every pod, at the moment the database can least afford
the reconnections.

Redis being down is **not** unhealthy. Every cache failure is a miss and the
handler does the work anyway, so nothing here probes Redis on the API's behalf.

## Checking a change

CI validates every file in this directory against the Kubernetes schemas on each
push. Against a real cluster:

```sh
kubectl apply --dry-run=server -f deploy/k8s/base/
```
