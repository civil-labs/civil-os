# Spec: public read API for improvements by ID

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-25 |
| **Status** | Draft. Decisions signed off by the owner on 2026-11-25. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: protos, query behaviour, fixtures and tests.
The order of the work and how the PRs are split go in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Valuations | **Yes.** `GetImprovementsByIdRequest` takes an optional `valuation_id`, as `GetParcelsWithImprovementSummaryByParcelIdRequest` does. With one, each improvement carries `market_value` and `assessed_value` as decimal strings. The parcel summary already publishes these values summed per parcel, so this is no new kind of data. |
| Geometry | **Not in this feature** (owner, 2026-11-25). No extent and no footprint. Whether improvement footprints are public at all is for [`../tile-source-allowlist/intent.md`](../tile-source-allowlist/intent.md) to settle first, since Martin publishes `improvement_geometry` today. Adding an extent RPC later doesn't break anything. |
| Modifier overrides and `properties` | **`properties` is returned. The two overrides aren't.** `properties` is already public on `ParcelDetails`, with the same 5 KB limit. `depreciation_modifier_override` and `quality_adjustment_modifier_override` are inputs to valuation work, and no client of the read API needs them. Adding them later is a new optional field. |
| Reverse lookup by party | **In this feature** (owner, 2026-11-25), as **`ImprovementsService.GetImprovementsByPartyId`**, not on `PartiesService` as the intent put it. The RPC returns improvements, so it goes on their service ([`../reverse-lookups-by-returned-type/spec.md`](../reverse-lookups-by-returned-type/spec.md)), next to `GetImprovementsByAddressId`. |
| Units | **Square feet,** as `area_sq_ft`. Every area in the public API is in square feet (`total_area_sq_ft`, `land_area_sq_ft`). The reader converts with the factor the parcel reads use (`10.7639`). |

## Further decisions

| Question | Decision |
|---|---|
| Parcel links are a list | **`repeated ImprovementParcel parcels`** (owner, 2026-11-25). Today `parcel_improvements` has the exclusion constraint `no_overlapping_improvements`, so an improvement sits on at most one parcel at any legal time. That rule **will be relaxed** later (for example, a building across a lot line), so nothing in this API may depend on it. The field documents that there may be several, and no test asserts a length of one except as a fact about a fixture. The intent's fixture "an improvement on two parcels" can't be inserted while the constraint stands. It's replaced by an improvement that **moves** between parcels over time (improvement 18 below), which tests `legal_as_of` on the link instead. |
| `is_primary` | **A property of the link, documented as such.** It says whether this improvement is *its parcel's* primary improvement (`no_overlapping_primary_improvements` allows one per parcel at a time), not whether this is the improvement's main parcel. It always agrees with `ParcelImprovementsSummary.primary_improvement_id` for that parcel, and a test pins that. |
| Each link also carries `feature_id` | The parcel's geometry version at `legal_as_of`, as `AddressParcel` and `TransferredParcel` carry, so a client can highlight the parcel on the tile map without another call. The visibility check joins geometry anyway, so it costs nothing. |
| Parties are references | `{party_id, ownership_share}`, and the client reads names with `PartiesService.GetPartiesById`. This keeps `improvements.v1` from importing `parties.v1`, and follows the address and party spec ("Reverse lookups return references"). The transfers API inlines party details only because it needs names as of the transfer time, which doesn't apply here. |
| What `parties` means | **`improvement_parties` only:** owners recorded on the improvement itself. An empty list is valid and common. It means no separate ownership of the building is recorded, so the client should look at the parcel's owners. The reader never fills it from `parcel_parties`. `GetImprovementsByPartyId` follows the same rule: it doesn't return improvements on parcels the party owns. That's `GetParcelsByPartyId`. |
| Address | **`address_id` and `formatted_address`**, as `ParcelDetails` carries them, so a building card shows its address without another call. `improvement_attributes.address_id` is not null, but both fields are `optional`: they're unset when the address isn't visible at `t` (address rule 1 of the address and party spec), so a set ID can always be read with `GetAddressesById`. |
| Bathrooms | **`full_bathrooms`, `half_bathrooms` and `bathrooms`**, the three fields the parcel summary has after [`../full-half-bathrooms/spec.md`](../full-half-bathrooms/spec.md). `bathrooms` is `full + 0.5 * half`, with an unset count taken as 0, and is unset only when both counts are unset. Summing a parcel's improvements gives its summary totals. |
| Type, condition and quality | Returned as IDs: `type_id` (always set, since types can't be voided), `condition_id` and `quality_id` (optional). The condition and quality lookups resolve names at now, as `GetImprovementConditions` does today. So `condition_id` and `quality_id` are set only when that lookup would return them: not voided, with an attributes row at now. That keeps "every ID can be read" true. |
| Quality lookup | **`GetImprovementQualities`**, the same shape as `GetImprovementConditions`: no request fields, resolved at `CURRENT_TIMESTAMP`, a map keyed by ID with `name` and `quality_adjustment_modifier`. Adding `legal_as_of` to the type, condition and quality lookups is a separate change, and it would have to cover all three together. |
| Valuation time | Values come from the `improvement_valuations` row for the requested valuation whose `legal_valid_range` contains `t`, as `parcel_valuations` is read. The unique index on `(improvement_id, valuation_id)` allows one row per valuation, so in practice `t` only decides whether that row is in range. An unknown `valuation_id`, or none, leaves both values unset rather than failing, as the parcel read does. The parcel summary's improvement totals don't apply this filter today. [`../valuation-legal-time/intent.md`](../valuation-legal-time/intent.md), written in this spec's PR, sets one rule for both reads, and this RPC follows whatever it decides. |
| Response map | `GetImprovementsById` returns `map<string, Improvement>` keyed by the requested ID, as `GetAddressesById` and `GetPartiesById` do. Hidden and unknown IDs are absent, not errors. |
| Order inside an improvement | `parcels` by `parcel_id`, `parties` by `party_id`, ascending. That's deterministic, and doesn't imply any meaning. |
| Response list limits | **No `max_items` on `parcels` or `parties`.** A cap stated in the proto but not enforced would be a false promise once the one-parcel rule is relaxed. Both are `unique`. |
| Idempotency | Every new RPC sets `idempotency_level = NO_SIDE_EFFECTS`. |

## Public API: additions to `civil.public.improvements.v1`

Field comments follow [`../response-validation-rules/intent.md`](../response-validation-rules/intent.md):
each states the values the field can take, since buf.validate rules don't
reach the TypeScript SDK.

```proto
service ImprovementsService {
  // ...existing: GetImprovementTypes, GetImprovementConditions, GetImprovementsByAddressId...
  rpc GetImprovementQualities(GetImprovementQualitiesRequest) returns (GetImprovementQualitiesResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetImprovementsById(GetImprovementsByIdRequest) returns (GetImprovementsByIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
  rpc GetImprovementsByPartyId(GetImprovementsByPartyIdRequest) returns (GetImprovementsByPartyIdResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message ImprovementQuality {
  string id = 1;                              // uuid
  string name = 2;
  double quality_adjustment_modifier = 3;     // not null in the schema
}

// improvement_attributes and its links, as of legal_as_of.
message Improvement {
  string improvement_id = 1;                  // uuid, improvements.public_id
  string type_id = 2;                         // uuid; GetImprovementTypes
  optional string condition_id = 3;           // uuid; set only when GetImprovementConditions returns it
  optional string quality_id = 4;             // uuid; set only when GetImprovementQualities returns it
  optional string address_id = 5;             // uuid; unset when the address isn't visible at legal_as_of
  optional string formatted_address = 6;      // max_len 256; set exactly when address_id is
  optional double area_sq_ft = 7;             // >= 0
  optional int32 full_bathrooms = 8;          // >= 0
  optional int32 half_bathrooms = 9;          // >= 0
  // full_bathrooms + 0.5 * half_bathrooms, an unset count taken as 0.
  // >= 0 in steps of 0.5; unset only when both counts are unset.
  optional double bathrooms = 10;
  optional int32 bedrooms = 11;               // >= 0
  optional int32 units = 12;                  // >= 0
  optional int32 year_built = 13;
  optional int32 effective_year_built = 14;
  optional string properties = 15;            // JSON object as text, max_bytes 5120
  // The visible parcels the improvement sits on at legal_as_of, ordered by
  // parcel_id. Never empty: an improvement with no visible parcel is hidden.
  // Can hold several entries; don't assume one.
  repeated ImprovementParcel parcels = 16;
  // Owners recorded on the improvement itself (improvement_parties) at
  // legal_as_of, ordered by party_id. Often empty: that means no separate
  // ownership of the improvement is recorded, not that it has no owner.
  repeated ImprovementShare parties = 17;
  // Decimal strings, the existing money pattern (numeric(19,4)). Set only when
  // a valuation_id is requested and has a value for this improvement at legal_as_of.
  optional string market_value = 18;
  optional string assessed_value = 19;
}

message ImprovementParcel {
  string parcel_id = 1;                       // uuid
  optional int64 feature_id = 2;              // the parcel's geometry version at legal_as_of
  // Whether this improvement is the primary improvement of *this parcel*.
  // At most one improvement per parcel has it at a time. It is not a
  // "main parcel" flag for the improvement.
  bool is_primary = 3;
}

message ImprovementShare {
  string party_id = 1;                        // uuid; readable with GetPartiesById
  string ownership_share = 2;                 // decimal string in (0, 1], up to 4 places;
                                              // valid_improvement_ownership guarantees the range
}

// An improvement the party holds at legal_as_of (improvement_parties).
message PartyImprovement {
  string improvement_id = 1;                  // uuid; readable with GetImprovementsById
  string ownership_share = 2;                 // as on ImprovementShare
}
```

### Requests and responses

| Request | Fields and validation |
|---|---|
| `GetImprovementQualitiesRequest` | none |
| `GetImprovementsByIdRequest` | `repeated string improvement_ids`: unique, 1..100, uuid; `optional Timestamp legal_as_of`; `optional string valuation_id`: uuid |
| `GetImprovementsByPartyIdRequest` | `string party_id`: uuid, required; `legal_as_of`; `optional int32 first` (1..100, 25 when unset); `optional string after` (1..512 characters) |

| Response | Fields |
|---|---|
| `GetImprovementQualitiesResponse` | `map<string, ImprovementQuality> improvement_qualities` |
| `GetImprovementsByIdResponse` | `map<string, Improvement> improvements`, keyed by improvement ID |
| `GetImprovementsByPartyIdResponse` | `repeated PartyImprovementEdge edges` (`cursor`, `node`), `civil.public.pagination.v1.PageInfo page_info` |

- **`legal_as_of`** has the same validation and description as on
  `GetParcelsWithImprovementSummaryByParcelIdRequest` (after year 0, defaults to
  now). Attributes, parcel links, parties, the address, parcel visibility and
  valuations are all resolved with `legal_valid_range @> t`.
- **`GetImprovementsByPartyId` on a hidden or unknown party** returns no edges,
  not an error, as the other reverse lookups do.

## Pagination

`GetImprovementsByPartyId` paginates exactly as the reverse lookups in
[`../address-party-read-api/spec.md`](../address-party-read-api/spec.md#pagination):
forward only, `first` validated at the gateway and clamped by the reader, and
the shared `civil.mesh.pagination.v1.IdCursor`. The filter hash covers the
RPC name, `party_id` and `legal_as_of`. The keyset is the improvement's
`public_id` ascending.

## Visibility rules

Checked in the reader's SQL at time `t`. Rules for addresses, parties and
parcels are the address and party spec's rules 1–3.

1. **An improvement** is visible when it isn't voided, has an
   `improvement_attributes` row at `t`, and has at least one
   `parcel_improvements` link at `t` to a visible parcel. That's the address
   and party spec's rule 4, unchanged.
2. **A parcel link** is listed only when the link row contains `t` and the
   parcel is visible. An improvement with several links lists only the visible
   ones.
3. **A party share** is listed only when its `improvement_parties` row
   contains `t` and the party is visible. A hidden party never hides the
   improvement, and an improvement left with no shares is still returned.
4. **`GetImprovementsByPartyId`** returns an edge when the requested party is
   visible and has an `improvement_parties` row at `t` on a visible improvement.

## Mesh API: `civil.mesh.improvements.v1`

`ImprovementsReadService` gains the three RPCs, with the same messages and no
validation or gnostic annotations, as the other mesh packages have. The gateway
validates, maps the messages one-to-one in `services/gateway/improvements.go`,
and fills the default `first`. The reader implements them in
`services/database-reader/improvements.go`.

## Reader query shape

- **`GetImprovementsById`:** a CTE picks the visible improvements
  (`public_id = ANY($1::uuid[])`, rule 1). The main query joins their
  attributes at `t`, the address (left join, rule 1 of the address and party
  spec), the condition and quality (left joins, each to its attributes at
  now), and `improvement_valuations` (left join on the requested valuation and
  `t`). Two follow-up queries, keyed by `= ANY` on the CTE's IDs, load the
  visible parcel links (with `feature_id` from `parcel_geometry` at `t`) and the
  visible party shares. The Go code assembles the messages, as the transfers
  reader does.
- **`GetImprovementsByPartyId`:** one query keyed on the party's internal ID,
  with rule 4, the keyset condition, `ORDER BY i.public_id`, and `LIMIT first + 1`.
- **`GetImprovementQualities`:** the conditions query, with the quality tables.

The indexes already exist: `idx_improvement_attributes_improvement_id`,
`idx_parcel_improvements_improvement_id`, `idx_improvement_parties_improvement_id`,
`idx_improvement_parties_party_id`, `idx_improvement_valuations_improvement_id_valuation_id`
and the `public_id` unique indexes. No migration is needed.

## Fixtures

New rows go in `database/fixtures/synthetic.sql`, and new constants in
`internal/testutil/fixtures.go`. The header comment's list of shapes is
updated. Every value is invented.

What's there already: improvements 1–15 on parcels 1–15 (primary), 16 as a
secondary building on parcel 3, and 17 on voided parcel 20. All have one
attributes row from 2020-01-01 and a valuation 1 row. Improvement 1 has a half
bath. Quality 1 exists but has **no attributes row**, and there are **no
`improvement_parties` rows** at all.

| Fixture | Rows | Exercises |
|---|---|---|
| Quality attributes | quality 1 `Fixture Standard`, modifier 1.0; new quality 2 `Fixture Premium`, modifier 1.2, used by no improvement; both from 2020-01-01 | `GetImprovementQualities` returns two entries; every current `quality_id` resolves |
| Improvement 2's parties | parties 2 and 3 at 0.5 each, from 2020-01-01 | shares matching the parcel's owners |
| Improvement 3's party | party 3 at 1.0, from 2020-01-01 | `GetImprovementsByPartyId(3)` returns 2 and 3 in ID order |
| Improvement 16's parties | party 5 at 0.5 and voided party 11 at 0.5, from 2020-01-01 | an owner that differs from the parcel's (party 3); rule 3 drops party 11 and keeps the improvement |
| Improvement 6's party history | party 10 at 1.0, 2020-01-01..2023-09-01; party 6 at 1.0 from 2023-09-01 | `legal_as_of` on shares; matches transfer 3, in which party 10 grants improvement 6 to party 6 |
| Improvement 17's party | party 10 at 1.0, from 2020-01-01 | `GetImprovementsByPartyId(10)` never returns 17 (rule 1) |
| Improvement 4's attribute history | an earlier row 2020-01-01..2023-06-01: condition 3, 100 m², 1 full bath, 2 bedrooms, effective year built 1984; the current row then starts 2023-06-01 | a `legal_as_of` before the change returns the earlier values (intent success criterion) |
| Improvement 18, which moves | a shed: type 1, address 12, 20 m², no condition, quality, bath, bedroom or unit counts, year built 2001, one attributes row from 2020-01-01. Not the primary improvement: on parcel 11 for 2020-01-01..2024-01-01, then on parcel 12 from 2024-01-01. No valuation row and no parties. | links at `t`; unset optional fields; `bathrooms` unset; no values even with `valuation_id` |

**Effect on existing reads.** Improvement 18 adds 20 m² to parcel 12's
summary now, and to parcel 11's before 2024. Improvement 4's history changes
parcel 4's summary before 2023-06-01. `GetImprovementsByAddressId(12)` gains
improvement 18. The plan checks every existing test that pins these parcels'
totals or addresses. Improvement 6's party history doesn't touch any existing
read, since nothing reads `improvement_parties` today.

## Tests

| Layer | Coverage |
|---|---|
| Gateway handler tests | `GetImprovementsById`: non-UUID IDs, 0 or 101 IDs, duplicates, a non-UUID `valuation_id`, and `legal_as_of` before year 0 are rejected without calling the mesh. `GetImprovementsByPartyId`: a missing or non-UUID `party_id`, `first` of 0 and 101 rejected; an unset `first` reaches the mesh as 25; cursors pass through unchanged. Mapping round-trips of a fully populated `Improvement`, `ImprovementQuality` and `PartyImprovement`. |
| Reader integration: by ID | Improvement 1 has every field, `area_sq_ft` of 125 m² converted, 2 full and 1 half bath, `bathrooms` 2.5, and one primary link to parcel 1 with feature 1001. Improvement 16 has a non-primary link to parcel 3, and one share, party 5 at `0.5000`. Improvement 2 has parties 2 and 3 at `0.5000`. Improvement 6 has party 6 now and party 10 at 2022. Improvement 4 at 2022 has condition 3, 1076.39 sq ft, 1 full bath, 2 bedrooms; now it has the current values. Improvement 18 is on parcel 12 now and parcel 11 at 2022, with `condition_id`, `quality_id`, the counts and `bathrooms` unset. Improvement 17, an unknown ID, and an improvement voided in the test are absent. An improvement whose address is voided in the test has `address_id` and `formatted_address` unset. With valuation 1, improvement 1 has market `82000.0000` and assessed `73800.0000`; without one, and for improvement 18 with one, both are unset. |
| Reader integration: agreement with the parcel summary | For parcels 1–15 now, and parcels 4, 11 and 12 at 2022: summing `GetImprovementsById` over the summary's `improvement_ids` gives its area, bedrooms, full, half and total bathrooms and units, and the one improvement whose link to that parcel has `is_primary` is `primary_improvement_id`. With valuation 1, the values sum to the summary's totals. The test is written per parcel, so it stays true when an improvement can sit on several parcels. |
| Reader integration: by party | Party 3 returns 2 (`0.5000`) then 3 (`1.0000`). Party 5 returns 16. Party 10 returns 6 at 2022 and nothing now. Party 6 returns 6 now and nothing at 2022. Party 11 and an unknown ID return nothing. With `first = 1`, party 3 walks two pages, with Relay page info. A cursor reused for another party, another RPC, or another `legal_as_of` fails with `InvalidArgument`. `first = 500` sent directly to the reader is clamped to 100. |
| Reader integration: qualities | Two entries, with names and modifiers; a quality voided in the test is absent. |
| e2e (`test/e2e`) | Through the gateway with a bearer token: `GetImprovementsById` for improvements 1 and 16, `GetImprovementsByPartyId` for party 3, and `GetImprovementQualities`. |

## Docs to update

- `api/proto/public/README.md`: the three new RPCs.
- `docs/architecture.md`: the reader's improvements mesh service gains three
  RPCs. No new env vars.
- `intent.md` isn't edited. This spec records where it departs from it: the
  party lookup's service, and the two-parcel fixture.

## Known limits

- **Lookups resolve at now.** Condition and quality names, and whether their
  IDs are set, come from now even when `legal_as_of` is in the past. That's
  how the existing type and condition lookups work.
- **Summary totals when an improvement sits on several parcels.** Once the
  one-parcel rule is relaxed, the parcel summary will add a shared
  improvement's full area, counts and values to every parcel it sits on, so
  summing summaries across parcels counts it twice. That's for the feature
  that relaxes the rule to settle, for example with a share per link. This API
  returns each improvement once and doesn't change.
- **Martin's raw table sources** publish `improvement_geometry` regardless of
  this API ([`../tile-source-allowlist/intent.md`](../tile-source-allowlist/intent.md)).

## Out of scope

Geometry and extents; the modifier overrides; `legal_as_of` on the type,
condition and quality lookups; writing improvements; `system_as_of`; an
improvements-by-parcel RPC (the summary's `improvement_ids` and
`GetImprovementsById` already cover it); changing `AddressImprovement.parcel_ids`;
relaxing the one-parcel constraint; backward pagination and `totalCount`.
