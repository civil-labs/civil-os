# Intent: live updates when someone else edits

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-27 |
| **Status** | Draft. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

A frontend never hears about someone else's edit. A form or map shows what
it loaded until the user reloads. With the per-row `etag` proposed in the
address write spec (PR #97), a stale write now fails, but the user only
finds out when they save, after the work is done.

Geometry makes it worse. Maps draw parcels from Martin vector tiles
(`/tiles/get_parcel_tiles`), not from RPC responses, so even a client that
knew a parcel had changed has no way to redraw just that parcel. Martin
also keeps tiles in an in-memory cache by default
(`services/tile-server/config.yaml` doesn't set `cache_size_mb`), so a
reload can still show the old shape.

## Proposed outcome

Clients can subscribe to changes and redraw what changed, within a second
or two of the commit, for every kind of write: the public write APIs, ELT
loads and manual SQL.

- **Capture in the database.** Triggers on the attribute and identity
  tables write each change to an outbox table: a sequence number, the
  entity type, its `public_id`, the legal range that changed, a bounding
  box for geometry changes (old ∪ new) and the `trace_id`. A `NOTIFY`
  carrying only the sequence number wakes listeners. Both commit or roll
  back with the write.
- **One stream API.** The reader holds a `LISTEN` connection and serves a
  mesh `ChangesReadService.WatchChanges`. It replays events after a
  client's last sequence number, then follows live ones. The gateway serves
  it as a public Connect server stream and fans it out, so the gateway
  still never touches the database. Clients filter by entity type and by a
  viewport bounding box.
- **Events name records; they don't carry them.** An event has IDs and a
  bounding box, not attributes or geometry. Clients re-read through the
  existing RPCs, under the same auth, so the stream can't leak what a read
  wouldn't return.
- **Maps redraw changed parcels without reloading every tile.** The client
  fetches the changed features over RPC, draws them in a GeoJSON layer
  above the tiles, and hides the stale tile features. Tiles catch up
  later. Martin's cache stops serving stale tiles after a change.
- **Editors get early warning.** A form showing a record that has just
  changed says so before the user saves. The etag stays the check that
  stops a stale write; the stream only warns sooner.
- **Voids and retires** arrive as events, and the client drops the
  record.

## Alternatives considered

| Approach | Why not |
|---|---|
| The writer publishes after commit | Misses writes that skip the writer (ELT, manual SQL). A crash between commit and publish loses the event. |
| `NOTIFY` alone, with the change in its payload | Nothing is kept, so a client that reconnects can't catch up. Payloads are capped at 8 KB. |
| Logical replication / CDC (Debezium, wal2json) | Replication slots to run and watch, and a slot that stalls blocks WAL cleanup. Too heavy for one database per deployment. |
| Polling a `ListChanges(since)` RPC | Works everywhere, but adds load and delay. A reasonable fallback for clients that can't stream. |
| SSE or WebSockets | Outside the protobuf contract, with a second auth path. Connect server streaming works over HTTP/1.1 and gRPC-Web, so the browser doesn't need them. |
| Cache-busting the tile URL on every change | Reloads every tile in the viewport, and the map flickers. Still useful as the lazy catch-up behind the overlay. |
| CRDTs (Yjs, Automerge) for co-editing geometry | Merging concurrent vertex edits can produce invalid polygons. It doesn't fit bitemporal audit rows, and parcels share boundaries. |

## Questions for the spec

- **Outbox retention**, and what deletes old events. There's no scheduler
  in the stack. The request-ids intent (PR #97) needs one too.
- **How far back a client may resume**, and what it does past that: a full
  reload.
- **Changes scheduled for the future** become current with no write, so no
  event fires. Clients set a timer from `legal_valid_to`, or the server
  emits "became effective" events (the scheduler again)?
- **Martin's cache:** off (`cache_size_mb: 0`), short TTLs, or purged by
  the gateway over the event's bounding box? Same for `Cache-Control` on
  tile responses.
- **Which tables emit events** in v1: parcels and addresses only, or every
  `*_attributes` table?
- **How an event describes a legal-time change**, so a client viewing "as
  of now" can ignore changes that don't touch now.
- **Bulk loads.** An ELT run can change thousands of rows in one
  transaction. One event per row, or a coarser "this area changed" event?
- **Reader replicas.** Each holds its own `LISTEN` connection. Is that
  fine at the expected replica count?
- **Presence** ("someone is editing this parcel") over the same stream,
  advisory only, or left for later?
- **Schema change:** a new table and triggers, so the owner generates the
  migration ([`atlas-migrations` skill](../../.claude/skills/atlas-migrations/SKILL.md)).

## Out of scope

- Filtering the stream by permission. It comes with the future OPA
  intent. Until then, events carry IDs only.
- Concurrent editing of one geometry by several people, and topology-aware
  edits of shared boundaries.
- Live updates for anything other than civil-os records, such as instance
  metadata.
