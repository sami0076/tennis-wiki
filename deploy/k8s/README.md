# Deploying Deucepoint

Manifests for a single-node k3s cluster on a Hetzner CX33. What runs here is the
API, Redis and Postgres — three things, not four. The frontend is static files on
a CDN and is deliberately not in the cluster.

This directory is the manifests. Standing up the cluster is #101.

```
base/     applied together, in filename order
jobs/     applied by hand, one at a time
secrets.example.yaml   copied to secrets.yaml and filled in; never committed
```

`base/` is its own directory so `kubectl apply -f` over it cannot pick up the
placeholder Secret or start an hour-long ingest by accident.

## Before anything

Three things this directory assumes and does not install: **k3s** (which brings
Traefik and the `local-path` storage class), **cert-manager**, and a DNS record
pointing at the node.

Two values are placeholders and must be replaced together:

| | |
|---|---|
| `api.deucepoint.example` | `base/10-config.yaml` and `base/50-ingress.yaml` |
| `email: REPLACE_ME` | `base/50-ingress.yaml`, for expiry warnings |

`.example` is reserved by RFC 2606, so nothing here can resolve to a host that
belongs to someone else while the real name is undecided.

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
intended order, not a fault.

The load takes about an hour: roughly 40 minutes for the match stage, 23 for the
reference stage, then 1m 36s for the ratings. Every figure here is from
[`docs/performance.md`](../../docs/performance.md).

## Rolling out a new image

Images are pinned by digest, not by tag, so the manifest alone answers what is
running. The publishing job in `ci.yml` prints the digest for every image it
pushes; take it from there, or ask the registry:

```sh
docker buildx imagetools inspect ghcr.io/sami0076/tennis-wiki/api:<short-sha> \
  --format '{{.Manifest.Digest}}'
```

Then edit `base/40-api.yaml`, and both Jobs if the tools image moved. Rolling
back is putting the previous digest back and applying again.

## Re-running things

A Job's pod template is immutable, so applying over a finished one is rejected
rather than re-run. Delete first:

```sh
kubectl -n deucepoint delete job migrate --ignore-not-found
kubectl -n deucepoint apply -f deploy/k8s/jobs/migrate.yaml
```

Both Jobs clean themselves up 24 hours after finishing.

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
