# Spec: real property transfers API

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft. Decisions marked **Proposed** need the owner's sign-off. Accepted when merged. |
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
| Queries | **Proposed:** lookups by transfer ID, by parcel ID and by feature ID, each with an optional time range. Polygon and amount-range searches are deferred. Adding an RPC later isn't a breaking change, and `GetSalesComparables` already covers the polygon case. |
| Names | **Proposed:** package `civil.public.transfers.v1`, service `RealPropertyTransfersService`, message `RealPropertyTransfer`. Mesh: `civil.mesh.transfers.v1.RealPropertyTransferReadService`. The short package name follows `landuses`. The full service and message names match the schema and keep "transfer" unambiguous in the SDK. |
| Party details | **Proposed:** inline. Return `party_id`, `name` and `mailing_address`, resolved **as of the transfer time** (`party_attributes.legal_valid_range @> transfer_timestamp`). The API already exposes owner names through `primary_owner_name`, so this exposes no new kind of data. Both fields are optional, because a party may have no attributes row for that time. |
| Amounts | **Proposed:** decimal strings (`numeric(19,4)`), as every other money field in the public API uses. The field is `optional`, because `transfer_amount` is nullable (gifts, nominal transfers). |
| Codes | **Proposed:** returned only, with no code filters in v1. A filter can be added later as an optional request field without breaking anything. |
| History | **Proposed:** no `system_as_of` in v1. History stays with the history APIs, consistent with `docs/tile-source-allowlist/intent.md`. Transfers are events and have no legal-time range, so there's no `legal_as_of` either. |
| Tile documentation | **Proposed:** the public API README and `docs/architecture.md` describe `/tiles/get_parcel_tiles/{z}/{x}/{y}` and its TileJSON at `/tiles/get_parcel_tiles` in prose, as an HTTP endpoint behind the same bearer token. Whether that TileJSON is a guaranteed contract is up to the tile-source-allowlist feature, not this one. |

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
  optional string transfer_amount = 3;           // ^-?[0-9]{1,15}(\.[0-9]{1,4})?$
  optional Deed deed = 4;                        // unset when book, page and uri are all null
  repeated TransferredParcel parcels = 5;        // at least one parcel or one improvement
  repeated TransferredImprovement improvements = 6;
  repeated TransferCode codes = 7;
}

message Deed {
  optional string book = 1;
  optional string page = 2;
  optional string uri = 3;   // string.uri when set
}

message TransferredParcel {
  string parcel_id = 1;      // uuid
  optional int64 feature_id = 2;   // current geometry's feature_id
  repeated TransferParty grantors = 3;
  repeated TransferParty grantees = 4;
}

message TransferredImprovement {
  string improvement_id = 1; // uuid
  repeated TransferParty grantors = 2;
  repeated TransferParty grantees = 3;
}

message TransferParty {
  string party_id = 1;                   // uuid
  optional string name = 2;              // max_len 128, as of transfer_time
  optional string mailing_address = 3;   // max_len 256, formatted_address as of transfer_time
  string transferred_share = 4;          // ^(0\.[0-9]{1,4}|1(\.0{1,4})?)$, > 0
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

Each response contains `repeated RealPropertyTransfer transfers` (max 500) and
`bool truncated`.

- **Order:** `transfer_time` descending, then `transfer_id` ascending, so the
  results are deterministic.
- **Cap:** **Proposed:** at most 500 transfers per response. If more match,
  the first 500 are returned with `truncated = true`. The existing APIs don't
  paginate, so this spec doesn't introduce pagination. `truncated` keeps the
  limit visible, and a page token can be added later without breaking anything.
- **Time range:** `time_range` is half-open, `[start_time, end_time)`, and
  either bound may be omitted. `GetTransfersById` has no range.
- **Unknown IDs** are left out of the results, not reported as errors, as with
  `GetParcelsWithImprovementSummaryByParcelId`.
- **Scope of the parcel and feature lookups:** these return every transfer that
  involves the requested parcel. The returned transfer still lists **all** of
  its visible parcels and improvements, not only the requested ones.

## Visibility rules

These apply to every RPC and are checked in the reader's SQL:

1. A **parcel** entry is hidden when `parcels.is_voided`. Requesting a voided
   parcel ID or its feature ID returns nothing for it.
2. An **improvement** entry is hidden when the improvement is voided. It is
   also hidden when its current `parcel_improvements` links lead only to voided
   parcels, as with improvement 17 in the `ImprovementsService` reads.
3. A **party** entry is hidden when `parties.is_voided`.
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

## Reader query shape

One query per RPC: a CTE selects the matching `real_property_transfer_id`s,
with the request filter and visibility rule 4 applied, then the order and a
`LIMIT 501` (the extra row sets `truncated`). Three follow-up queries, each
keyed by `= ANY($1)` on those IDs, load the parcel entries, improvement
entries and codes. The Go code assembles the messages, so no `json_agg`
nesting is needed. Party names join `party_attributes` and `address_attributes`
on `legal_valid_range @> rpt.transfer_timestamp`.

The schema's existing indexes are enough: `idx_rpt_party_parcels_parcel_id`,
`idx_rpt_party_parcels_rpt_id`, and the unique index on
`(real_property_transfer_id, real_property_transfer_code_id)`. No migration is
needed.

## Fixtures

New prefixes go in `database/fixtures/synthetic.sql` and
`internal/testutil/fixtures.go`: `90000000` transfers, `91000000` transfer code
types, `92000000` transfer codes. Every value is invented. Deed URIs use the
reserved `.invalid` TLD (`https://deeds.example.invalid/FX-1`).

| Code type 1 `Fixture Qualification` | Codes: 1 `Fixture Qualified`, 2 `Fixture Unqualified`. Code 3 `Fixture Untyped` has no type. |
|---|---|

| Transfer | Time | Amount | What it covers | Exercises |
|---|---|---|---|---|
| 1 | 2021-03-01 | 250000.0000 | parcel 5: party 9 → party 4 (1.0); codes 1, 3 | basic read, typed and untyped codes, deed |
| 2 | 2022-06-15 | 310000.0000 | parcel 2: party 1 → parties 2 and 3 (0.5 each) | several grantees, matches `MultiOwnerParcel` |
| 3 | 2023-09-01 | 520000.0000 | parcels 6 and 7 plus improvements 6 and 7: party 10 → party 6 (parcel 6), party 7 (parcel 7); code 2 | multi-parcel transfer, improvement entries |
| 4 | 2024-02-01 | 90000.0000 | voided parcel 20 and improvement 17 only | rule 4: never returned |
| 5 | 2024-05-01 | 180000.0000 | parcels 19 and 20: party 1 → parties 9 (parcel 19) and 10 (parcel 20) | rule 1: returned with only parcel 19 |
| 6 | 2025-01-10 | null, no deed | parcel 5: party 4 → party 5 (1.0) | null amount, no deed, second transfer on one parcel (ordering) |

The latest grantee on each parcel is its current `parcel_parties` owner, so
transfer reads and parcel reads agree. The fixtures don't model ownership
history before that, and no test relies on it.

**Effect on `GetSalesComparables`:** it reads `real_property_transfer_party_parcels`
today. With the fixtures it returns a latest sale for parcels 2, 5, 6, 7 and
19. No test checks sale fields at the moment, so nothing should break. A
reader integration test should pin the new values.

## Tests

| Layer | Coverage |
|---|---|
| Gateway handler tests (`handlers_test.go`) | validation on every RPC: non-UUID IDs, too many IDs, duplicates, an inverted `time_range`. The mesh is never called for invalid input. Mapping round-trip of a fully populated transfer. |
| Reader integration (`integration_test.go`) | transfer 2 has two grantees at 0.5; transfer 3 lists both parcels and both improvements when queried by parcel 6 alone; transfer 4 is missing by ID, by parcel 20 and by feature 1020; transfer 5 lists only parcel 19; parcel 5 returns 6 then 1; `time_range` excludes transfer 1 at 2022-01-01; a null amount and no deed are unset, not empty strings; unknown IDs return an empty list. |
| Truncation | integration test with a temporary insert of 501 transfers on one parcel, rolled back afterwards: `truncated = true`, 500 results. |
| e2e (`test/e2e`) | `GetTransfersByParcelId` for parcel 2 through the gateway with a bearer token returns transfer 2. `/tiles/get_parcel_tiles/...` still works. |
| Removal | the generated-code freshness check, plus a grep in CI or review that `sales.v1` and `tiles.v1` are gone from `openapi.yaml` and `sdk/js/gen`. |

## Docs to update

- `api/proto/public/README.md`: remove the "defined but not served" paragraph,
  add the transfers service, and document the tile endpoint in prose.
- `docs/architecture.md`: list the reader's new mesh service and describe the
  tile endpoint.
- No new env vars, so the config table doesn't change.

## Out of scope

Writing transfers, polygon and amount searches, code filters, `system_as_of`,
renaming `GetSalesComparables`, and the tile allowlist itself.
