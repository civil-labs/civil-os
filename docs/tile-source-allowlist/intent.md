# Intent: publish only the tile sources civil-os means to serve

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from PR 7 (compose) findings at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The tile server (Martin) is set up in `services/tile-server/config.yaml` with
only a connection string, so it **auto-publishes every geometry table and tile
function it finds in the database**. The compose stack's `/tiles/catalog` lists
these sources:

| Source | Content |
|---|---|
| `get_parcel_tiles` | the intended parcel tiles (`core.get_parcel_tiles`), which already hide voided parcels |
| `parcel_geometry.1` | raw parcel geometry, **including voided parcels** and every legal-time version |
| `parcel_geometry_history.1` | the full system-time history of parcel geometry |
| `improvement_geometry.1`, `improvement_geometry_history.1` | raw and historical improvement geometry |
| `address_attributes`, `address_attributes_history` | address point coordinates and their history |

That causes three problems:

- **It contradicts the owner's decision** that current requests hide voided
  parcels (PR 5). Anyone with a valid token can still fetch voided parcels and
  past versions through `/tiles/parcel_geometry.1/...`.
- **It exposes history** that no API is meant to serve yet.
- **The public surface is set by the schema, not by intent.** Any new geometry
  table, or any function with a matching signature, appears on the tile API
  without a decision or a review.

Production deployments use the same `config.yaml`, so this applies to every
live jurisdiction.

## Proposed outcome

1. **The tile server serves only an explicit list of sources**, starting with the
   ones civil-os uses on purpose (`get_parcel_tiles`, plus any others the
   frontends need).
2. **Voided parcels and history are not reachable through `/tiles/`.** Exposing
   them later is a separate feature, as it is for the RPC APIs.
3. **Adding a tile source takes a reviewed change** to the tile server config,
   not just a schema change.

## Constraints

- Tile URLs the current frontends use keep working, so their sources must be
  found before anything is switched off.
- The gateway keeps owning auth and CORS for `/tiles/`; this changes only what
  Martin publishes.
- The Helm chart, the gitops repo and the jurisdiction repos are not edited from
  here. If clusters override the tile config, the owner handles that rollout.

## Success criteria

- `/tiles/catalog` in the compose stack lists only the allowed sources.
- An e2e test: `get_parcel_tiles` returns a non-empty tile, and a request for
  `parcel_geometry.1` (or any table source) returns 404.
- An integration or e2e test shows the voided fixture parcel is absent from
  every published source.

## Open questions (carry into spec)

- Which tile sources do the current frontends actually request? Check frontend
  code and production access logs before turning anything off.
- Should improvement and address tiles become intentional function sources that
  filter voided rows, or be removed?
- Do production clusters mount their own Martin config over the image's? If so,
  where does the allow-list live?
- Should the tile catalog itself (`/tiles/catalog`) stay reachable through the
  gateway?
