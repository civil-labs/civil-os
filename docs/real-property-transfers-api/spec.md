# Spec: real property transfers API

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft. Decisions signed off by the owner. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: protos, query behaviour, fixtures and tests.
The order of the work and how the PRs are split go in `plan.md`.

## Removals

| Removed | Where |
|---|---|
| `civil.public.tiles.v1` (`TileService`) | `api/proto/public/civil/public/tiles/`, generated Go, npm SDK, `openapi.yaml` |
| `civil.public.sales.v1` (`SalesService`) | `api/proto/public/civil/public/sales/`, generated Go, npm SDK, `openapi.yaml` |
| `civil.mesh.sales.v1` (`SaleReadService`) | `api/proto/mesh/civil/mesh/sales/`, `internal/gen/go` |

Nothing serves these today, so no Go code outside `gen/` should need changes.
`buf breaking` will flag the public removals. That is expected, and the PR
title carries `!`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Queries | Lookups by transfer ID, by parcel ID and by feature ID, each with an optional time range. Polygon and amount-range searches are deferred. Adding an RPC later isn't a breaking change, and `GetSalesComparables` already covers the polygon case. The parcel and feature lookups are paginated (see [Pagination](#pagination)). |
| Names | Package `civil.public.transfers.v1`, service `RealPropertyTransfersService`, message `RealPropertyTransfer`. Mesh: `civil.mesh.transfers.v1.RealPropertyTransferReadService`. The short package name follows `landuses`. The full service and message names match the schema and keep "transfer" unambiguous in the SDK. |
| Party details | Inline. Return `party_id`, `name` and `mailing_address`, resolved **as of the transfer time** (`party_attributes.legal_valid_range @> transfer_timestamp`). The API already exposes owner names through `primary_owner_name`, so this exposes no new kind of data. Both fields are optional, because a party may have no attributes row for that time. |
| Party structure | Each party appears **once per transfer**, in `RealPropertyTransfer.parties`, with its ID, name and address. Each parcel and improvement entry lists its grantors and grantees as `TransferShare`s (`party_id` and `transferred_share`) that point into that list. The schema records shares per parcel and per improvement (`real_property_transfer_party_parcels`, `..._party_improvements`), and they can differ between parcels on one transfer, so they can't be flattened onto the transfer without losing who received what. Listing party details once keeps "who sold and who bought" a direct read, and keeps a many-parcel transfer from repeating the same names and addresses on every entry. |
| Missing parties | An empty `grantors` or `grantees` list on a parcel or improvement entry is valid, and the entry is still returned. It means the records are incomplete: no grantor or grantee row was recorded, or every one recorded belongs to a voided party. A conveyance always has both sides, so this is a data problem to be resolved, not a real state. Returning the entry with the empty list lets accounting and reconciliation work find it. Hiding the entry would make the transfer look complete when it isn't. The API documents this on both fields. |
| Amounts | Decimal strings (`numeric(19,4)`), as every other money field in the public API uses. The field is `optional`, because `transfer_amount` is nullable (gifts, nominal transfers). The schema has no check on the sign, so a negative amount can be stored. A negative amount is a data error, and the API returns it as stored rather than hiding or rejecting the transfer, so reconciliation can find it, as with missing parties. A check constraint is a separate feature ([`../transfer-amount-check/intent.md`](../transfer-amount-check/intent.md)). |
| Recordings | A transfer is the event, and the documents recorded with the clerk are the evidence for it. The transfer refers to its recordings and doesn't model deeds itself: `repeated Recording recordings` replaces a single `Deed`. `Recording` is defined in its own package, `civil.public.recordings.v1`, with no service yet, as `PageInfo` is in `civil.public.pagination.v1`. A recording's details are the same whether it is returned on a transfer or on its own, so the transfers API and a future recordings API share one message. Defining it in the transfers package now and moving it later would change the field's type, which `buf breaking` treats as a breaking change. A list is used because one transfer can be evidenced by several documents (a deed and a corrective deed), and turning a singular field into a list after release would break clients. In v1 the list is built from the `deed_book`, `deed_page` and `deed_uri` columns, so it holds at most one entry, and is empty when all three are null. A later recordings feature with its own intent may move this metadata into a shared `recordings` table, which type-specific tables (transfers, mortgages, liens, ...) link to. It would then add `recording_id`, `document_number`, `document_type` and `recorded_time` as new optional fields, which doesn't break anything. No schema change is made here. |
| Codes | Returned only, with no code filters in v1. A filter can be added later as an optional request field without breaking anything. |
| History | No `system_as_of` in v1. History stays with the history APIs, consistent with `docs/tile-source-allowlist/intent.md`. Transfers are events and have no legal-time range, so there's no `legal_as_of` either. |
| Tile documentation | The public API README and `docs/architecture.md` describe `/tiles/get_parcel_tiles/{z}/{x}/{y}` and its TileJSON at `/tiles/get_parcel_tiles` in prose, as an HTTP endpoint behind the same bearer token. Whether that TileJSON is a guaranteed contract is up to the tile-source-allowlist feature, not this one. |

## Public API: `civil.public.transfers.v1`

```proto
service RealPropertyTransfersService {
  rpc GetTransfersById(GetTransfersByIdRequest) returns (GetTransfersByIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetTransfersByParcelId(GetTransfersByParcelIdRequest) returns (GetTransfersByParcelIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetTransfersByFeatureId(GetTransfersByFeatureIdRequest) returns (GetTransfersByFeatureIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message RealPropertyTransfer {
  string transfer_id = 1;                        // uuid, real_property_transfers.public_id
  google.protobuf.Timestamp transfer_time = 2;   // transfer_timestamp
  optional string transfer_amount = 3;           // decimal string, up to 15 digits and 4 places;
                                                 // returned as stored, even if negative (see Amounts)
  repeated civil.public.recordings.v1.Recording recordings = 4;  // 0 or 1 entries in v1
  repeated TransferredParcel parcels = 5;        // at least one parcel or one improvement
  repeated TransferredImprovement improvements = 6;
  repeated TransferCode codes = 7;
  // Every visible party on any parcel or improvement entry, once each,
  // ordered by party_id. Entries refer to these by party_id.
  repeated TransferParty parties = 8;
}

// api/proto/public/civil/public/recordings/v1/recording.proto
// Messages only: no service until the recordings feature.
package civil.public.recordings.v1;

// A document recorded with the county clerk (a deed, a corrective deed, ...).
// Recording metadata only, never what the document says.
message Recording {
  optional string book = 1;
  optional string page = 2;
  optional string uri = 3;   // string.uri when set
  // Added by the recordings feature: recording_id, document_number,
  // document_type, recorded_time.
}

message TransferredParcel {
  string parcel_id = 1;      // uuid
  optional int64 feature_id = 2;   // current geometry's feature_id
  // An empty grantors or grantees list is valid. It means the transfer's
  // records are incomplete for this parcel (no row recorded, or only voided
  // parties), which is a data problem to reconcile. The entry is still
  // returned so that the gap is visible.
  repeated TransferShare grantors = 3;
  repeated TransferShare grantees = 4;
}

message TransferredImprovement {
  string improvement_id = 1; // uuid
  // Empty lists are valid, as on TransferredParcel.
  repeated TransferShare grantors = 2;
  repeated TransferShare grantees = 3;
}

// A party's share of one parcel or improvement in this transfer.
message TransferShare {
  string party_id = 1;            // uuid; always present in RealPropertyTransfer.parties
  string transferred_share = 2;   // decimal string in (0, 1], up to 4 places; the database's
                                  // valid_share_transfer check guarantees the range
}

message TransferParty {
  string party_id = 1;                   // uuid
  optional string name = 2;              // max_len 128, as of transfer_time
  optional string mailing_address = 3;   // max_len 256, formatted_address as of transfer_time
}

message TransferCode {
  string code_id = 1;        // uuid
  string name = 2;
  optional string description = 3;
  optional TransferCodeType type = 4;   // the code type column is nullable
}

message TransferCodeType {
  string code_type_id = 1;   // uuid
  string name = 2;
  optional string description = 3;
}
```

### Requests and responses

All three requests share a filter. Responses return a flat list, so a transfer
that covers several requested parcels appears once. Clients group the results
by `parcels[].parcel_id` if they need to.

| Request | Fields and validation |
|---|---|
| `GetTransfersByIdRequest` | `repeated string transfer_ids`: unique, 1..100, uuid |
| `GetTransfersByParcelIdRequest` | `repeated string parcel_ids`: unique, 1..100, uuid; `google.type.Interval time_range` with the same CEL rule as `GetSalesComparablesRequest` |
| `GetTransfersByFeatureIdRequest` | `repeated int64 feature_ids`: unique, 1..100, > 0; `time_range` as above |

`GetTransfersByParcelIdRequest` and `GetTransfersByFeatureIdRequest` also
carry the pagination fields (`first`, `after`), and their responses are
connections (`edges`, `page_info`). See [Pagination](#pagination).
`GetTransfersById` isn't paginated: its 100-ID limit already bounds the
response, which contains `repeated RealPropertyTransfer transfers`.

- **Order:** `transfer_time` descending, then `transfer_id` descending, so the
  results are deterministic. Both keys sort the same way so the keyset
  condition can be a single row comparison (see [Pagination](#pagination)).
- **Time range:** `time_range` is half-open, `[start_time, end_time)`, and
  either bound may be omitted. `GetTransfersById` has no range.
- **Unknown IDs** are left out of the results, not reported as errors, as with
  `GetParcelsWithImprovementSummaryByParcelId`.
- **Scope of the parcel and feature lookups:** these return every transfer that
  involves the requested parcel. The returned transfer still lists **all** of
  its visible parcels and improvements, not only the requested ones.

## Pagination

This is the first paginated API in civil-os, and it sets the pattern for the
rest. It follows the GraphQL [Cursor Connections
Specification](https://relay.dev/graphql/connections.htm) (Relay), adapted to
protobuf. The shared messages live in their own package so later APIs can
import them.

```proto
// api/proto/public/civil/public/pagination/v1/pagination.proto
package civil.public.pagination.v1;

// Relay PageInfo. Cursors are opaque: clients must not parse or build them.
message PageInfo {
  bool has_next_page = 1;
  bool has_previous_page = 2;
  optional string start_cursor = 3;   // cursor of the first edge; unset when there are no edges
  optional string end_cursor = 4;     // cursor of the last edge; unset when there are no edges
}
```

```proto
// civil.public.transfers.v1
message GetTransfersByParcelIdRequest {
  repeated string parcel_ids = 1;
  google.type.Interval time_range = 2;
  optional int32 first = 3;    // 1..100 (protovalidate); 25 when unset
  optional string after = 4;   // an end_cursor or edge cursor from a previous page
}

message RealPropertyTransferEdge {
  string cursor = 1;
  RealPropertyTransfer node = 2;
}

message GetTransfersByParcelIdResponse {
  repeated RealPropertyTransferEdge edges = 1;
  civil.public.pagination.v1.PageInfo page_info = 2;
}
// GetTransfersByFeatureId{Request,Response}: the same, with feature_ids.
```

**Page size.** ConnectRPC has no pagination support of its own, so the
maximum is a protovalidate rule on `first` (`int32: {gte: 1, lte: 100}`). The
gateway already runs the `validate` interceptor on every handler, so a request
over the limit fails with `InvalidArgument` before it reaches the mesh. Too
large a value is rejected, not silently reduced, so the error is explicit even
for a client that checks the page length instead of `has_next_page`. When
`first` is unset the gateway sends the default of 25. The mesh has no
validation, so the reader also clamps `first` to 1..100 as a second line of
defence.

The limits are part of the public contract (they're in the proto) and are
deliberately not configurable per instance:

- **Compatibility.** A frontend app must work against every civil-os
  instance. An instance with a lower maximum would reject requests that work
  elsewhere, and one with a higher maximum would let apps come to depend on
  pages other instances refuse. A per-instance value also can't appear in
  `openapi.yaml` or the SDK.
- **Availability.** The page size isn't an access-control boundary: a caller
  can read every matching transfer by following cursors. But a production
  instance misconfigured with a very large maximum would let a single cheap
  request pull large nested results. A fixed limit removes that setting.

If an instance ever needs to reduce load, the compatible way is to return
fewer edges than `first` with `has_next_page = true`, which Relay allows,
rather than to lower the maximum.

**Direction.** Forward only in v1 (`first` / `after`). Relay
makes `last` / `before` optional, and adding them later is a non-breaking
change (new optional fields). As Relay allows for forward pagination,
`has_previous_page` is true exactly when `after` was set.

**Cursors.** The reader owns the cursor format, since it knows the sort keys.
The gateway passes cursors through unchanged. A cursor is the base64url
encoding (no padding) of a small mesh-internal protobuf message:

```proto
message TransferCursor {
  uint32 version = 1;                          // 1
  google.protobuf.Timestamp transfer_time = 2;
  string transfer_id = 3;                      // public_id
  bytes filter_hash = 4;                       // SHA-256 of the canonical request filter
}
```

- The filter hash covers the RPC name, the sorted, de-duplicated IDs and the
  time range. A cursor used with a different filter fails with
  `InvalidArgument`, as does one that doesn't decode or has an unknown
  version. `first` isn't hashed, so a client may change the page size between
  pages.
- Cursors aren't signed or encrypted. Their content is only sort keys from
  rows the caller can already read, so tampering with one can't reveal
  anything a valid request couldn't.
- Cursors don't expire. They stay valid as long as the version is supported.

**Consistency.** This is keyset pagination, not offset pagination, so rows
inserted or voided between pages don't shift the pages. There's no snapshot:
a transfer inserted behind the cursor is never seen by that walk, and one
voided after a page was served disappears from later pages.

**Not included:** `totalCount` (it costs a second count query, and can be
added later as an optional field), and Relay's `edges`-less `nodes`
shorthand.

## Visibility rules

These apply to every RPC and are checked in the reader's SQL:

1. A **parcel** entry is hidden when `parcels.is_voided`. Requesting a voided
   parcel ID or its feature ID returns nothing for it.
2. An **improvement** entry is hidden when the improvement is voided. It is
   also hidden when its current `parcel_improvements` links lead only to voided
   parcels, as with improvement 17 in the `ImprovementsService` reads.
3. A **party** is hidden when `parties.is_voided`: it is left out of
   `parties` and of every `grantors` and `grantees` list. A parcel or
   improvement entry left with an empty list is still returned (see "Missing
   parties" above). Rule 3 never hides a parcel, improvement or transfer.
   Since a party only appears in `parties` if it's on a visible entry, a party
   on only voided parcels is left out too.
4. A **transfer** is returned only if at least one parcel or improvement entry
   is still visible after rules 1 and 2. A transfer that involves only voided
   parcels doesn't appear, even when it's requested by ID.

## Mesh API: `civil.mesh.transfers.v1`

`RealPropertyTransferReadService` has the same three RPCs and messages as the
public service, without validation or gnostic annotations, as the other mesh
packages do. The gateway validates requests with protovalidate, maps the
messages one-to-one in `services/gateway/transfers.go`, and registers the
handler next to the others. The reader implements the service in
`services/database-reader/transfers.go`.

The mesh copies of `PageInfo` and `Recording` go in `civil.mesh.pagination.v1`
and `civil.mesh.recordings.v1`, so mesh packages never import public ones. `TransferCursor` is defined in
`civil.mesh.transfers.v1` and never appears in the public API. Clients only
see the encoded string.

## Reader query shape

One query per RPC: a CTE selects the matching `real_property_transfer_id`s,
with the request filter and visibility rule 4 applied. The paginated RPCs add
the keyset condition `(rpt.transfer_timestamp, rpt.public_id) < ($t, $id)`
when `after` is set, then `ORDER BY rpt.transfer_timestamp DESC, rpt.public_id
DESC LIMIT first + 1`. The extra row only sets `has_next_page` and isn't
returned. Three follow-up queries, each
keyed by `= ANY($1)` on those IDs, load the parcel entries, improvement
entries and codes. A parcel or improvement entry exists only through its
party rows, so the follow-up queries select every row for a visible parcel or
improvement, including rows of voided parties, and the Go code drops the
voided parties afterwards. That keeps an entry whose parties are all voided,
with empty lists. `parties` is built from the shares that remain. The Go code assembles the messages, so no `json_agg`
nesting is needed. Party names join `party_attributes` and `address_attributes`
on `legal_valid_range @> rpt.transfer_timestamp`.

The schema's existing indexes are enough: `idx_rpt_party_parcels_parcel_id`,
`idx_rpt_party_parcels_rpt_id`, and the unique index on
`(real_property_transfer_id, real_property_transfer_code_id)`. No migration is
needed.

## Fixtures

New prefixes go in `database/fixtures/synthetic.sql` and
`internal/testutil/fixtures.go`: `90000000` transfers, `91000000` transfer code
types, `92000000` transfer codes. Every value is invented. Recording URIs use the
reserved `.invalid` TLD (`https://recordings.example.invalid/FX-1`).

| Code type 1 `Fixture Qualification` | Codes: 1 `Fixture Qualified`, 2 `Fixture Unqualified`. Code 3 `Fixture Untyped` has no type. |
|---|---|

| Transfer | Time | Amount | What it covers | Exercises |
|---|---|---|---|---|
| 1 | 2021-03-01 | 250000.0000 | parcel 5: party 9 → party 4 (1.0); codes 1, 3 | basic read, typed and untyped codes, one recording |
| 2 | 2022-06-15 | 310000.0000 | parcel 2: party 1 → parties 2 and 3 (0.5 each) | several grantees, matches `MultiOwnerParcel` |
| 3 | 2023-09-01 | 520000.0000 | parcels 6 and 7 plus improvements 6 and 7: party 10 → party 6 (parcel 6), party 7 (parcel 7); code 2 | multi-parcel transfer, improvement entries |
| 4 | 2024-02-01 | 90000.0000 | voided parcel 20 and improvement 17 only | rule 4: never returned |
| 5 | 2024-05-01 | 180000.0000 | parcels 19 and 20: party 1 → parties 9 (parcel 19) and 10 (parcel 20) | rule 1: returned with only parcel 19 |
| 6 | 2025-01-10 | null, no recording | parcel 5: party 4 → party 5 (1.0) | null amount, no recording, second transfer on one parcel (ordering) |
| 7 | 2022-01-15 | 140000.0000 | parcel 8: party 3 as grantor, no grantee row | empty `grantees` list, entry still returned |

The latest grantee on each parcel is its current `parcel_parties` owner, so
transfer reads and parcel reads agree. Parcel 8 is the exception on purpose:
transfer 7 records no grantee. The existing `parcel_parties` ranges all start
on 2020-01-01, before the transfers that grant them. That mismatch is
deliberate: the fixtures don't model ownership history, so don't "fix" the
dates without adding tests that rely on them.

**Effect on `GetSalesComparables`:** it reads `real_property_transfer_party_parcels`
today. It doesn't check grantee rows, so with the fixtures it returns a latest sale
for parcels 2, 6, 7, 8 and 19. Parcel 5 isn't one: its latest transfer is 6,
which has no amount, and the handler skips candidates without a price. No test checks sale fields at the moment, so nothing should break. A
reader integration test should pin the new values.

## Tests

| Layer | Coverage |
|---|---|
| Gateway handler tests (`handlers_test.go`) | validation on every RPC: non-UUID IDs, too many IDs, duplicates, an inverted `time_range`. The mesh is never called for invalid input. Mapping round-trip of a fully populated transfer. |
| Reader integration (`integration_test.go`) | transfer 2 has two grantee shares at 0.5 and three entries in `parties`; transfer 3 lists parties 6, 7 and 10 once each in `parties`, with party 10 as grantor on both parcels; every `party_id` in a share is in `parties`; transfer 7 returns parcel 8 with one grantor and an empty `grantees` list; a party voided by the test (committed: each test gets its own copy of the fixture database from `testutil.FixtureDatabase`, so nothing needs restoring) disappears from `parties` and from its share lists while its parcel entry stays; transfer 3 lists both parcels and both improvements when queried by parcel 6 alone; transfer 4 is missing by ID, by parcel 20 and by feature 1020; transfer 5 lists only parcel 19; parcel 5 returns 6 then 1; `time_range` excludes transfer 1 at 2022-01-01; a null amount is unset, not an empty string; transfer 6 has an empty `recordings` list and transfer 1 has exactly one; unknown IDs return an empty list. |
| Pagination (reader integration) | parcel 5 with `first = 1` returns transfer 6 with `has_next_page = true` and `has_previous_page = false`; its `end_cursor` as `after` returns transfer 1 with `has_next_page = false` and `has_previous_page = true`; a page past the end has no edges and no `start_cursor` / `end_cursor`; a cursor from parcel 5 reused for parcel 2, a malformed cursor and an unknown version all fail with `InvalidArgument`; `first = 500` sent directly to the reader is clamped to 100. The fixtures are enough, so no bulk insert is needed. |
| Pagination (gateway handler) | `first = 0` and `first = 101` are rejected without calling the mesh; an unset `first` reaches the mesh as 25; cursors pass through unchanged. |
| e2e (`test/e2e`) | `GetTransfersByParcelId` for parcel 2 through the gateway with a bearer token returns transfer 2. `/tiles/get_parcel_tiles/...` still works. |
| Removal | the generated-code freshness check, plus a grep in CI or review that `sales.v1` and `tiles.v1` are gone from `openapi.yaml` and `sdk/js/gen`. |

## Docs to update

- `api/proto/public/README.md`: remove the "defined but not served" paragraph,
  add the transfers service, and document the tile endpoint in prose.
- `docs/architecture.md`: list the reader's new mesh service and describe the
  tile endpoint.
- No new env vars, so the config table doesn't change.

## Known limits

**Large multi-parcel transfers.** `first` limits the number of transfers in a
page, not the size of each one. A portfolio sale or a developer selling a
whole subdivision can be one transfer covering hundreds of parcels, each with
its own grantors and grantees. Lookups by parcel and feature return every
visible parcel on each matching transfer, so a few such transfers can dominate
a page, whatever its size. v1 accepts this: real transfers bound the size, and
nothing in the synthetic fixtures approaches it. If real data shows it's a
problem, the planned fix is to cap the parcels and improvements listed per
transfer, with a flag that the list was cut short and an RPC that pages
through one transfer's entries. Both additions are new fields or RPCs, so they
don't break clients.

## Out of scope

Writing transfers, polygon and amount searches, code filters, `system_as_of`,
backward pagination (`last` / `before`), `totalCount`, retrofitting pagination
onto the existing APIs, a `recordings` table or API (county clerk documents),
renaming `GetSalesComparables`, and the tile allowlist itself.
