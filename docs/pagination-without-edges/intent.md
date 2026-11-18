# Intent: paginate without edges

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-18 |
| **Status** | Draft, spawned by [`../parcel-search/spec.md`](../parcel-search/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

Every paginated public RPC returns Relay-style connections. Each result is
wrapped in an `<Thing>Edge { cursor, node }`, next to a `PageInfo`. The
transfers spec chose this shape to follow the GraphQL Cursor Connections
Specification, and the address and party lookups copied it:

| Service | RPCs | Edge |
|---|---|---|
| `RealPropertyTransfersService` | `GetTransfersByParcelId`, `GetTransfersByImprovementId`, `GetTransfersByPartyId` | `RealPropertyTransferEdge` |
| `AddressesService` | `GetParcelsByAddressId`, `GetImprovementsByAddressId`, `GetPartiesByAddressId` | `AddressParcelEdge`, `AddressImprovementEdge`, `PartyEdge` |
| `PartiesService` | `GetParcelsByPartyId` | `PartyParcelEdge` |

The cursor on each edge buys nothing in this API:

- **Paging needs only `page_info.end_cursor`.** That's what the public
  README tells clients to use.
- **Every cursor is a keyset position**, not a reference to a row. A client
  that edits its cached list, or whose last row is later voided, can still
  continue from `end_cursor`. Per-edge cursors only add resuming from the
  middle of a page, which no client does.
- **The libraries that rely on edges are GraphQL tools** (Relay, Apollo,
  urql), and they can't read Connect responses. Connect clients, such as
  `@connectrpc/connect-query`'s infinite queries, read the next cursor from
  the response, and `page_info` serves them.

Meanwhile the edges cost something:

- an extra message and a `.node` hop for every result, in every client;
- one encoded cursor per result, each repeating the filter hash, which is most
  of the cursor;
- a pattern the public README tells every new paginated RPC to copy.

[`../parcel-search/spec.md`](../parcel-search/spec.md) already drops edges
for `SearchParcelsAddresses`. The public API is still at 0.x, with seven RPCs
on the edge shape and the records app as its known client, so changing it is cheapest
now, before more RPCs and clients depend on it.

## Proposed outcome

1. **Every paginated public RPC returns `repeated <Thing> <things>`** next to
   `page_info`. `start_cursor` and `end_cursor` stay as they are.
2. **The `<Thing>Edge` messages are deleted**, along with their mesh
   counterparts and the reader's per-edge cursor encoding.
3. **The public README's pagination section** describes one pattern, without
   edges, and stops citing the Relay spec as the model to follow.
4. **The records app and the JS SDK's examples** move over in step, with a
   migration note in the release.

## Constraints

- **This breaks the public API**, because it removes and renumbers fields.
  The PR title takes a `!`, `buf breaking` has to be satisfied the way the
  repo handles intended breaks (`docs/breaking-check-baseline`), and
  release-please cuts a new minor version under 0.x.
- **Pagination behaviour doesn't change:** the same cursors, ordering, keyset
  conditions, page size limits and errors. `end_cursor` from the old shape
  doesn't need to stay valid across the release.
- **Every service in one release**, so the API never offers two pagination
  shapes at once outside the window this intent leaves open.
- **No deployment-repo changes** from here. The records app is updated in its
  own repo.

## Success criteria

- No public or mesh proto has an `Edge` message or an `edges` field.
- Every paginated RPC's integration and e2e tests walk all pages through
  `end_cursor` and get the same results as before.
- The public README describes one pagination pattern.

## Open questions (carry into spec)

- **Field names.** Use a plural of the node type (`transfers`, `parcels`,
  `parties`), or a uniform name such as `results` or `items` across every
  RPC?
- **Clean break or deprecation.** Remove `edges` in one release, or add the
  new field next to it, mark `edges` deprecated, and remove it one release
  later? At 0.x, with one known client, the clean break looks cheaper. Does the
  records app's release timing allow it?
- **The reference messages.** `AddressParcel`, `PartyParcel` and the others
  were edges' nodes. Do any of them need fields that the edge carried, or do
  they stand alone unchanged?
- **`start_cursor`.** Keep it for a future backward pagination, or drop it,
  since nothing reads it?
