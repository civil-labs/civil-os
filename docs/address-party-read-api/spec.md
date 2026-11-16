# Spec: public read API for addresses and parties, and parcel extents

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Decisions signed off by the owner on 2026-11-16. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: the protos, the visibility rules, the
fixes to existing reads, the fixtures and the tests. The order of the work and
how the PRs are split go in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Names | Packages `civil.public.addresses.v1` and `civil.public.parties.v1`, services `AddressesService` and `PartiesService`, messages `Address` and `Party`. Mesh: `civil.mesh.addresses.v1.AddressesReadService` and `civil.mesh.parties.v1.PartiesReadService`. This follows `landuses` for the package and `RealPropertyTransfersService` for the plural service name. *Amended 2026-11-16, at the owner's request:* the mesh services take the plural from the start, following [`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md), which replaces the intent's `AddressReadService` and `PartyReadService`. They won't need renaming when that feature renames the existing mesh services. Until then the mesh is mixed: the two RPCs added to `ParcelReadService` and `RealPropertyTransferReadService` keep those services' current names. |
| Reverse lookup for addresses | **Three RPCs, each paginated:** `GetParcelsByAddressId`, `GetImprovementsByAddressId` and `GetPartiesByAddressId`. One RPC returning all three kinds would need either three independent cursors in one response or one cursor over mixed kinds. Both are awkward for clients, and Relay has no pattern for either. Each RPC takes one `address_id`. A shared mailing address, such as a management company's, can have hundreds of parties, so all three are paginated, even though a parcel or improvement list is usually short. |
| Country and administrative area | **Both code and name**, as small nested messages: `Country { code, name }` and `AdministrativeArea { code, name }`. A code alone is useless to a display client without a lookup API, and a name alone can't be matched reliably. No public IDs: no RPC reads these tables, so an ID would be a dead end. Adding `country_id` later is a new optional field and doesn't break anything. |
| Transfers by party | **In this feature:** `RealPropertyTransfersService.GetTransfersByPartyId`. It's the next hop from a party in the records app, and it reuses the transfers query, cursor and pagination as they are. Adding an RPC doesn't break anything. |
| Improvements | **Its own intent**, [`../improvement-read-api/intent.md`](../improvement-read-api/intent.md), written in this spec's PR. Reading an improvement properly covers its type, condition, quality, areas, parcel links and `improvement_parties`. That's a design of its own, comparable to this one. To keep the reverse lookup from being a dead end until then, each improvement it returns carries the `parcel_ids` it sits on, so a client can always move on to a parcel. |
| Extent message | **A new message, `ExtentWGS84`**, in `civil.public.parcels.v1`, with the same four fields and descriptions as `GetEstimatedParcelsExtentWGS84Response`. That response can't be changed to use the new message without breaking clients, so it stays as it is. **No `...ByFeatureId` variant:** a feature ID names one geometry version, and a client that has one got it from a loaded tile, so it already knows where the parcel is. **`legal_as_of` is included**, as on the other parcel reads, and costs nothing. |
| Parcel search | **Not here, and the address lookup isn't meant to grow into it.** Lookups by ID can't be extended into a text search. A search needs its own RPC, ranking and indexes (for example trigram indexes on `formatted_address` and party names, which need a migration). [`../parcel-search/intent.md`](../parcel-search/intent.md) records the need and is written in this spec's PR. |

## Further decisions

These come up while specifying and aren't in the intent's list.

| Question | Decision |
|---|---|
| Fixing the existing reads | **In this feature, as `fix:` changes.** The existing reads return voided parties and voided addresses, and they don't resolve address and party attributes at the legal time (see [Fixes to existing reads](#fixes-to-existing-reads)). The intent's first success criterion says any `address_id` or `party_id` the API returns can be read. A `party_ids` entry for a voided party breaks that criterion, because the new lookup correctly hides it. The new history fixtures would also make today's results nondeterministic. None of these fixes changes a proto. The owner chose this over a separate `fix:` intent on 2026-11-16. The plan puts the fixes, with the fixtures that expose them, in their own PR, merged before the new RPCs. |
| Which parcels count as visible | **The parcel reads' existing rule:** a parcel is visible at time `t` when it isn't voided and has both a geometry row and an attributes row at `t`. The reverse lookups and the extent lookup apply the same rule, so every `parcel_id` they return can be read with `GetParcelsWithImprovementSummaryByParcelId`. A parcel without geometry is therefore invisible across the API. That's today's behaviour, and changing it is out of scope. |
| Records without attributes at `t` | **Left out of the response**, as `GetParcelsWithImprovementSummaryByParcelId` documents: "If a parcel does not have data for that time, no data for that parcel is returned." An address or party with no attributes row at `t` is treated like an unknown ID. |
| Lookups by ID return maps | `GetAddressesById`, `GetPartiesById` and `GetParcelExtentsWGS84ByParcelId` return a map keyed by the requested ID, as `ParcelsService` does. Unknown, voided and attribute-less IDs are absent from the map, not reported as errors. |
| Reverse lookups return references | Edges carry small reference messages (IDs, plus a share or feature ID where the link has one), not full records. The exception is `GetPartiesByAddressId`, which returns full `Party` records because a party is small. Clients load full parcels with the existing batch read, whose 100-ID limit matches the maximum page size. |
| Order of reverse lookups | Ascending by the returned record's public ID. That's deterministic and keyset-friendly. It isn't meaningful to a reader, but neither is any order available without joining attributes, and a client sorts a page for display anyway. |
| Idempotency | Every new RPC sets `idempotency_level = NO_SIDE_EFFECTS`, as the transfers RPCs do, so it can be called with Connect GET. |

## Public API: `civil.public.addresses.v1`

```proto
service AddressesService {
  rpc GetAddressesById(GetAddressesByIdRequest) returns (GetAddressesByIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetParcelsByAddressId(GetParcelsByAddressIdRequest) returns (GetParcelsByAddressIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetImprovementsByAddressId(GetImprovementsByAddressIdRequest) returns (GetImprovementsByAddressIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetPartiesByAddressId(GetPartiesByAddressIdRequest) returns (GetPartiesByAddressIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

// address_attributes as of legal_as_of.
message Address {
  string address_id = 1;                       // uuid, addresses.public_id
  string formatted_address = 2;                // not null in the schema
  optional string address_line_1 = 3;
  optional string address_line_2 = 4;
  optional string address_line_3 = 5;
  optional string locality = 6;
  optional string sublocality = 7;
  optional string postal_code = 8;
  optional AdministrativeArea administrative_area = 9;
  optional Country country = 10;
  optional google.type.LatLng coordinates = 11; // from the 4326 point; unset when null
}

message Country {
  string code = 1;   // ISO 3166-1 alpha-2, as stored (fixtures use ZZ)
  string name = 2;
}

message AdministrativeArea {
  string code = 1;   // as stored, up to 10 characters
  string name = 2;
}

// A parcel whose attributes, at legal_as_of, name this address.
message AddressParcel {
  string parcel_id = 1;            // uuid
  optional int64 feature_id = 2;   // the parcel's geometry version at legal_as_of
}

// An improvement whose attributes, at legal_as_of, name this address.
message AddressImprovement {
  string improvement_id = 1;       // uuid
  // The visible parcels it sits on at legal_as_of (parcel_improvements),
  // ordered by parcel_id. Never empty: an improvement with no visible
  // parcel is hidden (visibility rule 4).
  repeated string parcel_ids = 2;
}
```

`google.type.LatLng` comes from googleapis, which the module already depends
on for `google.type.Interval`.

## Public API: `civil.public.parties.v1`

```proto
service PartiesService {
  rpc GetPartiesById(GetPartiesByIdRequest) returns (GetPartiesByIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetParcelsByPartyId(GetParcelsByPartyIdRequest) returns (GetParcelsByPartyIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

// party_attributes as of legal_as_of.
message Party {
  string party_id = 1;                       // uuid, parties.public_id
  string name = 2;                           // not null in the schema
  // Unset when the mailing address is voided or has no attributes at
  // legal_as_of, so a set ID can always be read with GetAddressesById.
  optional string mailing_address_id = 3;    // uuid
  optional string mailing_address = 4;       // its formatted_address
}

// A parcel the party holds at legal_as_of (parcel_parties).
message PartyParcel {
  string parcel_id = 1;            // uuid
  optional int64 feature_id = 2;
  string ownership_share = 3;      // decimal string in (0, 1], up to 4 places;
                                   // the valid_parcel_ownership check guarantees the range
}
```

`Party` lives in `civil.public.parties.v1`, and `addresses.v1` imports it for
`GetPartiesByAddressId`. `parties.v1` doesn't import `addresses.v1`: the
mailing address is two flat fields, as `ParcelDetails` carries `address_id` and
`formatted_address`, so the two packages don't import each other.

## Public API additions to existing services

```proto
// civil.public.parcels.v1
service ParcelsService {
  // ...existing RPCs...
  rpc GetParcelExtentsWGS84ByParcelId(GetParcelExtentsWGS84ByParcelIdRequest) returns (GetParcelExtentsWGS84ByParcelIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

// The WGS84 bounding box of one parcel's geometry (geom_web) at legal_as_of.
message ExtentWGS84 {
  double min_x = 1;   // Minimum Longitude / West
  double min_y = 2;   // Minimum Latitude / South
  double max_x = 3;   // Maximum Longitude / East
  double max_y = 4;   // Maximum Latitude / North
}

message GetParcelExtentsWGS84ByParcelIdResponse {
  // Keyed by the UUID parcel ID. Parcels that aren't visible at legal_as_of
  // (voided, or no geometry or attributes then) are absent.
  map<string, ExtentWGS84> extents = 1;
}

// civil.public.transfers.v1
service RealPropertyTransfersService {
  // ...existing RPCs...
  rpc GetTransfersByPartyId(GetTransfersByPartyIdRequest) returns (GetTransfersByPartyIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}
// GetTransfersByPartyId{Request,Response}: the same shape as
// GetTransfersByParcelId, with party_ids in place of parcel_ids.
```

## Requests and responses

| Request | Fields and validation |
|---|---|
| `GetAddressesByIdRequest` | `repeated string address_ids`: unique, 1..100, uuid; `optional Timestamp legal_as_of` |
| `GetPartiesByIdRequest` | `repeated string party_ids`: unique, 1..100, uuid; `legal_as_of` |
| `GetParcelExtentsWGS84ByParcelIdRequest` | `repeated string parcel_ids`: unique, 1..100, uuid; `legal_as_of` |
| `GetParcelsByAddressIdRequest`, `GetImprovementsByAddressIdRequest`, `GetPartiesByAddressIdRequest` | `string address_id`: uuid, required; `legal_as_of`; `optional int32 first` (1..100, 25 when unset); `optional string after` |
| `GetParcelsByPartyIdRequest` | `string party_id`: uuid, required; `legal_as_of`; `first`; `after` |
| `GetTransfersByPartyIdRequest` | `repeated string party_ids`: unique, 1..100, uuid; `time_range` with the existing CEL rule; `first`; `after` |

- **`legal_as_of`** has the same validation and description as on
  `GetParcelsWithImprovementSummaryByParcelIdRequest` (after year 0, defaults
  to now). Every attribute, link and geometry row is resolved with
  `legal_valid_range @> t`.
- **By-ID responses** are `map<string, Address> addresses`,
  `map<string, Party> parties` and `map<string, ExtentWGS84> extents`.
- **Paginated responses** are connections, as in the transfers spec:
  `repeated <Node>Edge edges` and `civil.public.pagination.v1.PageInfo page_info`,
  with edges `AddressParcelEdge`, `AddressImprovementEdge`, `PartyEdge`
  (node `Party`) and `PartyParcelEdge`. `GetTransfersByPartyId` reuses
  `RealPropertyTransferEdge`.
- **A reverse lookup on a hidden or unknown ID** (a voided address, a voided
  party, or a random UUID) returns no edges, not an error. That's
  consistent with unknown IDs in by-ID lookups, and doesn't reveal whether a
  voided record exists.

## Pagination

As in [`../real-property-transfers-api/spec.md`](../real-property-transfers-api/spec.md#pagination):
forward only, `first` validated to 1..100 at the gateway and clamped by the
reader, opaque base64url cursors owned by the reader, and keyset queries.

The reverse lookups share one mesh cursor message, since each sorts by a single
UUID:

```proto
// civil.mesh.pagination.v1
message IdCursor {
  uint32 version = 1;       // 1
  string last_id = 2;       // public_id of the last edge returned
  bytes filter_hash = 3;    // SHA-256 of the canonical request filter
}
```

- **The filter hash** covers the RPC name, the requested ID and `legal_as_of`
  (whether it's set, and its value). A cursor from `GetParcelsByAddressId`
  can't be reused on `GetPartiesByAddressId`, or for another address. When
  `legal_as_of` is unset, each page resolves "now" separately. That's no
  weaker than the transfers walk, which has no snapshot either.
- **The keyset condition** is `public_id > $last_id`, ordered by
  `public_id ASC LIMIT first + 1`.
- **`GetTransfersByPartyId`** uses the existing `TransferCursor` and
  ordering. Its RPC name goes into the hash, so a cursor from
  `GetTransfersByParcelId` fails with `InvalidArgument` there.

## Visibility rules

These rules are checked in the reader's SQL at time `t` (`legal_as_of`, or now):

1. **An address** is visible when it isn't voided and has an
   `address_attributes` row at `t`.
2. **A party** is visible when it isn't voided and has a `party_attributes` row
   at `t`. Its `mailing_address_id` and `mailing_address` are set only when
   the mailing address is visible (rule 1).
3. **A parcel** is visible when it isn't voided and has a `parcel_geometry` row
   and a `parcel_attributes` row at `t`, as the parcel reads require today.
4. **An improvement** is visible when it isn't voided, has an
   `improvement_attributes` row at `t`, and has at least one
   `parcel_improvements` link at `t` to a visible parcel. That's how the
   transfers spec treats improvement 17.
5. **A link** is returned only when both ends are visible: a `parcel_parties`
   row at `t` for `GetParcelsByPartyId`, and an attributes row naming the
   address at `t` for the address reverse lookups.
6. **Transfers by party:** a transfer matches when the requested party
   appears as a grantor or grantee on one of the transfer's visible parcel or
   improvement entries, by the transfers spec's rules 1–4. A voided party
   matches nothing. The returned transfer is built exactly as the other
   transfer lookups build it, with all its visible entries and parties.

## Fixes to existing reads

These change behaviour, not protos. Each is a `fix:` change, and `buf breaking`
doesn't see them.

| # | Read | Today | After |
|---|---|---|---|
| F1 | `ParcelDetails.address_id`, `formatted_address` (both parcel lookups) | `address_attributes` joined with no time condition; voided addresses returned | Attributes at `t`. Both fields unset when the address isn't visible (rule 1). |
| F2 | `ParcelDetails.party_ids`, `primary_owner_name`, `primary_owner_address` | Voided parties included; `party_attributes` and the mailing `address_attributes` joined with no time condition, so a party with history appears twice and the primary owner is arbitrary | Only parties visible at `t` (rule 2), once each, in the existing order (share descending, then internal ID). `primary_owner_address` unset when the primary owner's mailing address isn't visible. |
| F3 | `GetEquityComparables`, `GetSalesComparables` | `parcel_attributes`, `address_attributes` and `parcel_geometry` joined with no time condition, so a parcel with history yields several rows and one is kept arbitrarily | Resolved at now (these RPCs have no `legal_as_of`), with rule 1 applied to the address. A redrawn parcel matches a polygon only by its current geometry. |
| F4 | `TransferParty.mailing_address` | A voided mailing address returned | Unset when the address is voided. It's still resolved at the transfer time. |

Today's fixtures don't show any of these: no address, party or geometry has
history yet, and no party is voided. The new fixtures below do, and the
existing reader tests are extended to pin each fix.

## Mesh API

`civil.mesh.addresses.v1.AddressesReadService` and
`civil.mesh.parties.v1.PartiesReadService` carry the same RPCs and messages as
their public services. The new RPCs on `ParcelReadService` and
`RealPropertyTransferReadService` follow the same pattern. None of them has
validation or gnostic annotations, as in the other mesh packages. Mesh
packages don't import public ones, so `LatLng` is used from `google.type` in
both, and mesh copies of `Country`, `AdministrativeArea` and the reference
messages live in the mesh packages.

The gateway validates, maps the messages one-to-one in
`services/gateway/addresses.go` and `parties.go` (and in the existing
`parcels.go` and `transfers.go` for the two new RPCs), and registers both new
handlers. The reader implements them in `services/database-reader/addresses.go`
and `parties.go`.

## Reader query shape

- **By ID:** one query per RPC, `public_id = ANY($1::uuid[])`, with the
  visibility rule and the joins at `t`. Country and administrative area are
  `LEFT JOIN`s. `ST_X` and `ST_Y` give the coordinates, and
  `ST_XMin`/`ST_YMin`/`ST_XMax`/`ST_YMax` on `geom_web` give the extent.
- **Reverse lookups:** one query each, keyed on the internal ID of the
  requested record, with the keyset condition and `LIMIT first + 1`.
  `GetImprovementsByAddressId` aggregates the visible parcel IDs per
  improvement with `array_agg(... ORDER BY public_id)`.
- **Transfers by party:** a new filter CTE, a party's
  `real_property_transfer_party_parcels` and `..._party_improvements` rows on
  visible entries, which feeds the existing transfer assembly code.

The indexes already exist: `idx_parcel_attributes_address_id`,
`idx_improvement_attributes_address_id`, `idx_party_attributes_address_id`,
`idx_parcel_parties_party_id`, the `public_id` unique indexes, and
`idx_parcel_geometry_parcel_id`. The transfer party tables are indexed by
party too (`idx_rpt_party_parcels_party_id`,
`idx_rpt_party_improvements_party_id`), so no migration is needed.
*Corrected 2026-11-16, while planning:* this paragraph said those indexes
were missing and asked for an `EXPLAIN` check.

## Fixtures

New rows go in `database/fixtures/synthetic.sql`, and new helpers or
constants in `internal/testutil/fixtures.go`. The header comment's list of
shapes is updated. Every value is invented.

Most of what the intent asks for exists already:

| Intent asks for | Already in the fixtures |
|---|---|
| A party holding several parcels with different shares | Party 3: parcel 2 at 0.5, parcels 3 and 13 at 1.0 |
| A parcel with several parties | Parcel 2: parties 2 and 3 at 0.5 each |
| An address shared by a parcel and a party's mailing address | Address 3: parcel 3, improvements 3 and 16, and party 3's mailing address |

The following are added:

| Fixture | Rows | Exercises |
|---|---|---|
| Voided party 11 | `parties` 11, voided 2025-06-01; attributes `Fixture Owner 11`, mailing address 11, from 2020-01-01; `parcel_parties` on parcel 4 at 0.5 from 2020-01-01 | rule 2 in the new reads; F2 (parcel 4's `party_ids` is party 4 only); `GetPartiesByAddressId(11)` leaves it out; `GetTransfersByPartyId(11)` returns nothing |
| Party 8's history | attributes `Fixture Owner 8 Formerly`, mailing address 18, 2020-01-01..2023-01-01; the current row (`Fixture Owner 8`, address 8) then starts 2023-01-01 | `legal_as_of` returns the earlier name and mailing address; `GetPartiesByAddressId(18)` at 2022 includes party 8 and now doesn't; F2 (parcels 8 and 18 return one `party_ids` entry). Party 8 is in no transfer, so the transfer tests don't change. |
| Address 5's history | attributes `5 Old Fixture Way`, formatted `5 Old Fixture Way, Testville ZZ000`, no coordinates, 2020-01-01..2023-01-01; the current row then starts 2023-01-01 | `legal_as_of` on an address; unset `coordinates`; F1 (parcel 5 returns one address) |
| Parcel 9 redrawn | an earlier geometry, the west half of its square, feature 1109, 2020-01-01..2024-01-01; the current geometry (feature 1009) then starts 2024-01-01 | the extent is the current box, not the earlier one; `legal_as_of` 2022 returns the earlier box and feature 1109; F3 |
| Parcel 21, no geometry | `parcels` 21, attributes with address 2 and land use 1 from 2020-01-01, owned by party 2 at 1.0; no `parcel_geometry` row | absent from extents, from `GetParcelsByPartyId(2)` and from `GetParcelsByAddressId(2)` (rule 3) |

`fx` already has 21 rows, so addresses 1–21 exist. The parcel inserts filter
`n <= 20` and need extending for parcel 21, with no geometry.
`testutil.ParcelCount` and every test that loops over all parcels are checked
in the plan.

## Tests

| Layer | Coverage |
|---|---|
| Gateway handler tests | For every new RPC: non-UUID IDs, too many IDs, duplicates, a missing `address_id` or `party_id`, `first` of 0 and 101, and a `legal_as_of` before year 0 are rejected without calling the mesh. An unset `first` reaches the mesh as 25, and cursors pass through unchanged. Mapping round-trips of a fully populated `Address`, `Party`, `ExtentWGS84`, and each reference message. |
| Reader integration: addresses | Address 3 has every field, `ZZ` / `Testland` and `ZZ-FX` / `Fixture Province`, and coordinates at the centre of parcel 3's square. Address 5 now has the current line, and at 2022 it has the old line and no coordinates. Unknown and voided IDs are absent (an address is voided in the test, which is committed, since each test gets its own database copy). `GetParcelsByAddressId(3)` returns parcel 3 with feature 1003. `GetImprovementsByAddressId(3)` returns improvements 3 and 16, each with `parcel_ids` of parcel 3. `GetImprovementsByAddressId(20)` returns nothing, because improvement 17 is only on voided parcel 20. `GetPartiesByAddressId(3)` returns party 3, `(11)` returns nothing, and `(18)` returns party 8 at 2022 and nothing now. `GetParcelsByAddressId(2)` returns parcel 2 but not parcel 21. |
| Reader integration: parties | Party 3 has its name, mailing address 3 and ID. Party 8 is `Fixture Owner 8` / address 8 now, and `Fixture Owner 8 Formerly` / address 18 at 2022. Voided party 11 and unknown IDs are absent. A party whose mailing address is voided in the test is returned with both mailing fields unset. `GetParcelsByPartyId(3)` returns parcels 2 (`0.5000`), 3 and 13 (`1.0000`) in ID order. `(2)` returns parcels 2 and 12 but not 21. `(10)` returns parcel 10 only, without voided parcel 20. `(11)` returns nothing. |
| Reader integration: extents | Parcel 1's box equals its fixture square. Parcel 9 now has the full square, and at 2022 the west half. Parcel 20 (voided) and parcel 21 (no geometry) are absent. Every returned box contains the parcel's current geometry (`ST_Contains(ST_MakeEnvelope(...), geom_web)` in the test). |
| Reader integration: transfers by party | Party 1 returns transfers 5 then 2. Party 10 returns transfer 3 only: transfer 5's parcel 20 entry is voided, so party 10 isn't on a visible entry there. Party 11 returns nothing. A `time_range` excludes as by parcel. Pagination and a cursor reused from `GetTransfersByParcelId` behave as in the transfers spec. |
| Reader integration: pagination | `GetParcelsByPartyId(3)` with `first = 1` walks parcels 2, 3 and 13 over three pages, with `has_next_page` and `has_previous_page` set as in Relay. A cursor reused across RPCs, across addresses or parties, or with a different `legal_as_of` fails with `InvalidArgument`. A malformed cursor, or one with an unknown version, fails the same way. `first = 500` sent directly to the reader is clamped to 100. |
| Reader integration: fixes | F1: parcel 5 returns one current address. F2: parcel 4's `party_ids` is party 4 only, and parcels 8 and 18 return `Fixture Owner 8` once. F3: equity and sales comparables return parcel 9's current feature ID, and parcel 5's current address. F4: a transfer party whose mailing address is voided in the test has `mailing_address` unset. |
| e2e (`test/e2e`) | One smoke read per new RPC through the gateway with a bearer token: address 3, the parcels, improvements and parties at address 3, party 3, party 3's parcels, parcel 1's extent, and party 1's transfers. |

## Docs to update

- `api/proto/public/README.md`: add `AddressesService` and `PartiesService`,
  and the two new RPCs on the existing services.
- `docs/architecture.md`: list the reader's two new mesh services. In
  "Read/write split", note that new mesh services use the public domain
  name (`AddressesReadService`), pending the rename of the existing ones in
  [`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md).
- `CLAUDE.md`, "Mesh naming": the same note, so agents name new mesh
  services in the plural.
- No new env vars, so the config table doesn't change.

## Known limits

- **Antimeridian.** A parcel that crosses 180° gets a box spanning the whole
  globe, because the extent is a plain min/max. Real jurisdictions don't cross
  it. The synthetic grid is at 140° W.
- **Martin's raw table sources.** Martin publishes `parcel_geometry` as a
  table source, so parcel 9's earlier geometry is visible there.
  That's already true of any geometry history, and it belongs to
  [`../tile-source-allowlist/intent.md`](../tile-source-allowlist/intent.md).
- **Parcels without geometry** are invisible everywhere, as today. If the
  records app needs them, that's its own change to the parcel visibility
  rule.

## Out of scope

Writing addresses, parties or geometry; full geometry (GeoJSON or WKT); parcel,
address or party search ([`../parcel-search/intent.md`](../parcel-search/intent.md));
reading improvements by ID ([`../improvement-read-api/intent.md`](../improvement-read-api/intent.md));
`...ByFeatureId` extents; `system_as_of`; lookup APIs for countries and
administrative areas; backward pagination and `totalCount`; and the tile
allowlist.
