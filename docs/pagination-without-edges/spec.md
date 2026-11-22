# Spec: paginate without edges

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-21 |
| **Status** | Draft. Field names, the clean break, the reference messages, `start_cursor` and field numbering are proposals awaiting sign-off in review. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: the new response shape for every paginated
RPC, how the protos change, what the reader and gateway do, the README, the
release and the tests. The order of the work and how the PRs are split go in
`plan.md`.

## Correction to the intent

The intent's table lists `GetTransfersByImprovementId`. No such RPC exists.
The third transfers lookup is `GetTransfersByFeatureId`. The seven RPCs are:

| Service | RPC | Edge today | New field |
|---|---|---|---|
| `RealPropertyTransfersService` | `GetTransfersByParcelId` | `RealPropertyTransferEdge` | `repeated RealPropertyTransfer transfers` |
| `RealPropertyTransfersService` | `GetTransfersByFeatureId` | `RealPropertyTransferEdge` | `repeated RealPropertyTransfer transfers` |
| `RealPropertyTransfersService` | `GetTransfersByPartyId` | `RealPropertyTransferEdge` | `repeated RealPropertyTransfer transfers` |
| `AddressesService` | `GetParcelsByAddressId` | `AddressParcelEdge` | `repeated AddressParcel parcels` |
| `AddressesService` | `GetImprovementsByAddressId` | `AddressImprovementEdge` | `repeated AddressImprovement improvements` |
| `AddressesService` | `GetPartiesByAddressId` | `civil.public.parties.v1.PartyEdge` | `repeated civil.public.parties.v1.Party parties` |
| `PartiesService` | `GetParcelsByPartyId` | `PartyParcelEdge` | `repeated PartyParcel parcels` |

`ParcelsService.SearchParcelsAddresses` already has no edges and doesn't
change.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Field names | **Proposed: the plural of the item type** (`transfers`, `parcels`, `improvements`, `parties`), as in the table above. It's the protobuf convention for list responses (Google AIP-158 uses `repeated Book books`), it says what a field holds without a trip to the message, and `GetTransfersById` already returns `repeated RealPropertyTransfer transfers`, so a client reads transfers the same way from every transfers RPC. A uniform name (`results`, `items`) would let generic client code read any page without help. The records app's `PagedList` does that today through `page.edges`. Under this proposal it takes a function from a page to its rows instead, one short arrow function at each of its five call sites. `SearchParcelsAddresses` already broke uniformity with `hits`, so a uniform name wouldn't be uniform anyway without breaking search too. |
| Clean break or deprecation | **Proposed: clean break**, in one release. The records app is the only known client and reads edges in one place (`PagedList.tsx`). It pins the SDK version, so it keeps building against 0.4.x until it moves. A deprecation release would need the reader and gateway to fill both shapes, tests for both, and a second breaking release to remove `edges`, all to protect one client whose change is a few lines. |
| The reference messages | **They stand alone, unchanged.** An edge only adds `cursor`. Each item already carries its own ID (`transfer_id`, `parcel_id`, `improvement_id`, `party_id`), which is all a client needs to read it again or key it in a list. Nothing on an edge is lost except the cursor, which `page_info` covers. Comments that call them a "node" or refer to an edge are reworded. |
| `start_cursor` | **Proposed: kept.** Nothing reads it from these RPCs, but it costs one string per page, and removing it is a break of its own. `SearchParcelsAddresses` sets it, and its integration tests read it. Dropping it would leave `PageInfo` shaped for this release only, and backward pagination, if it's ever added, would need it back. |

## Further decisions

| Question | Decision |
|---|---|
| Field numbers | **The new field takes number 3. Number 1 and the name `edges` are reserved.** Reusing 1 would change the field's type in place. A client built against the old shape would then decode each item as an edge without any error: a `RealPropertyTransfer`'s `transfer_id` (field 1, a string) would read as the edge's `cursor`, and its `transfer_time` (field 2, a message) as the `node`. With a new number, an old client sees an empty `edges` list and an unchanged `page_info`. That's still wrong, but empty rather than corrupted, and easier to diagnose. `page_info` keeps number 2. |
| The mesh | **Changes in the same way, with the same numbering.** The mesh promises no compatibility, but the gateway and the reader roll out separately in a cluster. During a rollout, a new gateway can talk to an old reader. A reused field number would turn that into silently corrupted results. A new one turns it into empty pages for the minutes the rollout takes. The mesh `Edge` messages are deleted, and `TransferCursor`'s comment stops calling it an edge cursor. |
| Cursors | **Their encoding doesn't change.** `TransferCursor`, the ID cursor and the search cursor keep their version, fields and filter hash. The reader encodes only the first and last item's cursor, for `start_cursor` and `end_cursor`, instead of one per item. So an `end_cursor` from before the release still works after it. That isn't promised (the intent says it doesn't need to be), but it means a client mid-walk during the rollout won't fail. |
| `PageInfo` comments | **Stop citing Relay.** The public message's header currently reads "Pagination state for a connection, following the GraphQL Cursor Connections Specification". It becomes "Pagination state for a page of results". The field comments already say "first result" and "last result" and stay as they are. The mesh `PageInfo` gets the same rewording. These are comment-only changes. |
| SDK examples | **Nothing to change.** `sdk/js/README.md` has no pagination example, and nothing else under `sdk/` mentions edges. The README's pagination section (below) is the example. |

## Public API

Every response changes in the same way. `GetTransfersByParcelIdResponse` as
an example:

```proto
// Before
message RealPropertyTransferEdge {
  string cursor = 1;
  RealPropertyTransfer node = 2;
}

message GetTransfersByParcelIdResponse {
  repeated RealPropertyTransferEdge edges = 1;
  civil.public.pagination.v1.PageInfo page_info = 2;
}

// After
message GetTransfersByParcelIdResponse {
  reserved 1;
  reserved "edges";
  // The page's transfers, in the RPC's order. Fewer than `first` only on the
  // last page.
  repeated RealPropertyTransfer transfers = 3;
  // Pass page_info.end_cursor as `after` to read the next page.
  civil.public.pagination.v1.PageInfo page_info = 2;
}
```

Each RPC's order is the one it has today, as documented on the RPC. The
item comment names that order where the response doesn't already, for
example "newest first" for transfers.

Messages deleted from the public module:

- `civil.public.transfers.v1.RealPropertyTransferEdge`
- `civil.public.addresses.v1.AddressParcelEdge`
- `civil.public.addresses.v1.AddressImprovementEdge`
- `civil.public.parties.v1.PartyEdge`
- `civil.public.parties.v1.PartyParcelEdge`

Requests don't change: `first`, `after`, their validation and their limits
stay as they are.

## Mesh API

The same change in `civil.mesh.transfers.v1`, `civil.mesh.addresses.v1` and
`civil.mesh.parties.v1`, with the same field names and numbers, and the same
five `Edge` messages deleted. `GetTransfersByFeatureIdResponse` on the mesh
changes too.

## Reader

- **`pageByID`** (`services/database-reader/id_cursor.go`) returns the items
  and the `PageInfo`, not a slice of cursors. It encodes a cursor for the
  first and last item only. The address and party handlers put the items
  straight into the response.
- **`pageTransfers`** (`transfers.go`) does the same: it returns
  `[]*RealPropertyTransfer` and the `PageInfo`, encoding two cursors.
- Errors don't change. A cursor that fails to encode is still
  `Internal: cursor encoding error`, and a bad `after` is still
  `InvalidArgument`.
- Queries, ordering, keyset conditions, page sizes and the `NOT p.is_voided`
  filters don't change. No schema change, so no migration.

## Gateway

The `map*Edge` functions in `services/gateway/{transfers,addresses,parties}.go`
are replaced by mapping the items: `mapSlice(meshRes.Msg.Transfers,
mapTransfer)` and so on. The item mappers already exist inside the edge
mappers, and the `PageInfo` mapping doesn't change.

## Public README

The pagination section's **Response** bullet becomes:

> **Response:** `repeated <Thing> <things>` (the page's results, named for
> what they are, such as `transfers` or `parcels`; search's are `hits`) and a
> `civil.public.pagination.v1.PageInfo page_info`. Page with
> `page_info.end_cursor`. Results carry no cursor of their own.

The sentence about older RPCs still returning edges, and the link to this
intent, are removed. The rest of the section, including "Only forward
pagination (`first` / `after`) is supported for now", stays.

## Release

- **One PR for the protos, the reader, the gateway, the tests and the
  README**, so `main` never serves two shapes. The plan may split it into
  more PRs only if none of them leaves a public RPC with edges after another
  has lost them.
- **The PR title takes a `!`:** `feat!: return paginated results without
  edges`. `buf breaking` against the PR's base fails on the deleted messages
  and fields, and the `!` lets it pass with a warning
  ([`../breaking-check-baseline/spec.md`](../breaking-check-baseline/spec.md)).
- **The squash commit carries a `BREAKING CHANGE:` footer** with the
  migration note, so release-please puts it in `api/proto/public/CHANGELOG.md`:

  > Paginated responses no longer wrap results in edges. Read
  > `response.transfers` (or `parcels`, `improvements`, `parties`) instead of
  > `response.edges[].node`, and keep paging with `page_info.end_cursor`.
  > Per-result cursors are gone.

- **Versions:** the public API goes from 0.4.1 to 0.5.0, and the npm SDK
  with it. The root package's next version is a minor bump too, since the
  gateway's responses change.

## The records app

It's updated in its own repo, after `@civil-labs/civil-os-api` 0.5.0 is
published, and isn't part of this work. The change it needs:

- bump the SDK to 0.5.0;
- `PagedList`'s `Page<T>` drops `edges`, and the component takes a
  `rows: (page) => T[]` prop, such as `(p) => p.transfers`;
- its five call sites pass that prop.

**Deployment order:** a deployment must not run this civil-os release with a
records app still on SDK 0.4.x, because the app would show every paginated
list as empty. The records app release that moves to 0.5.0 has to be ready
before a deployment picks up this civil-os version. Both are pinned in the
deployment repos, which aren't changed from here.

## Tests

- **Integration tests** (`services/database-reader/*_integration_test.go`)
  that read `Edges[i].Node` read the items. Every paginated RPC has a test
  that walks every page through `end_cursor` with a small `first` and
  checks the IDs and order against the fixtures' expected list. The existing
  walks for transfers and the address and party lookups are kept. Where one is
  missing, it's added.
- **The tests that compared `end_cursor` with the last edge's cursor**
  instead check that passing `end_cursor` as `after` returns the next item
  and nothing already seen.
- **Cursor unit tests** (`*_cursor_test.go`) don't change, because the
  encoding doesn't.
- **Gateway handler tests** (`services/gateway/transfers_test.go` and the
  others) check the items and `PageInfo` are copied through.
- **e2e** (`test/e2e/smoke_test.go`): the transfers, address and party
  checks read the new fields, and the multi-page walk for `GetParcelsByPartyId`
  stays.
- **A proto check:** `grep -rE 'Edge\b|edges' api/proto` finds nothing outside
  `reserved` lines. It's run by hand for the PR. It's not a CI step, since the
  success criterion only has to be met once.
