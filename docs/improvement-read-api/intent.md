# Intent: public read API for improvements by ID

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Draft, spawned by [`../address-party-read-api/spec.md`](../address-party-read-api/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The public API returns improvement IDs in several places but offers no way to
read an improvement:

| Reference | Where it appears |
|---|---|
| `improvement_ids`, `primary_improvement_id` | `ParcelImprovementsSummary` |
| `improvement_id` | `RealPropertyTransfer.improvements` |
| `improvement_id` | `AddressImprovement`, from `GetImprovementsByAddressId` (address and party read API) |

`ImprovementsService` only lists the improvement types and conditions. A
client can see totals on a parcel's improvement summary (area, bedrooms,
bathrooms, units, values) and the primary improvement's year built and
condition. It can't read one improvement's own record, or see a secondary
building at all except as an ID.

The schema already holds everything:

- `core.improvements`, with `public_id` and `is_voided`;
- `improvement_attributes` at a legal time:
  - the address and the type, condition and quality;
  - `area_sq_m`, `bedrooms`, `bathrooms`, `units`;
  - `year_built` and `effective_year_built`;
  - the depreciation and quality modifier overrides;
  - `properties`;
- `parcel_improvements`: the parcels an improvement sits on, and whether it's
  the primary one;
- `improvement_parties`: the improvement's own owners and shares, separate
  from the parcel's;
- `improvement_geometry`: a 3D footprint (`MultiPolygonZ`, EPSG:4326 in
  `geom_web`);
- `improvement_valuations`: per-valuation market and assessed values.

The records app needs this to complete the walk the address and party read
API starts. A user who opens an address or a transfer and sees a building
should be able to open that building.

## Proposed outcome

1. **`ImprovementsService.GetImprovementsById`**, served by the reader through
   the mesh improvements read service. It takes up to 100 improvement IDs and
   an optional `legal_as_of`. It returns, per visible improvement:
   - its attributes, with the address, type, condition and quality as IDs;
   - the visible parcels it sits on, each with `is_primary`;
   - its visible parties, each with its `ownership_share`.
2. **The same patterns as the address and party read API:**
   - a map keyed by ID;
   - unknown, voided and attribute-less improvements left out;
   - an improvement on only voided parcels hidden;
   - `legal_as_of` defaulting to now;
   - `NO_SIDE_EFFECTS`.
3. **Improvement qualities become readable**, as types and conditions already
   are, so the quality ID isn't a dead end.
4. **Fixtures and tests:**
   - an improvement with its own party that differs from the parcel's owner;
   - an improvement with attribute history;
   - an improvement on two parcels.

   Each gets handler tests, reader integration tests and an e2e smoke read.

## Constraints

- **Read-only, and it adds to the API without breaking anything:** a new RPC
  on an existing service, and new messages.
- **No new kind of data.** Improvement totals and primary-improvement fields
  are already public on the parcel summary. This makes each improvement
  addressable by ID.
- **Only synthetic data.**

## Success criteria

- Every improvement ID the public API returns can be read with
  `GetImprovementsById`, unless it's hidden by the visibility rules.
- An improvement's parties can differ from its parcel's, and both are
  returned correctly.
- A `legal_as_of` before an attribute change returns the earlier values.

## Open questions (carry into spec)

- **Valuations.** Should the response take an optional `valuation_id` and
  return market and assessed values, as the parcel read does?
- **Geometry.** Should it return an extent of the 3D footprint, as parcels get
  through `GetParcelExtentsWGS84ByParcelId`, or nothing?
- **Modifier overrides and `properties`.** Are
  `depreciation_modifier_override`, `quality_adjustment_modifier_override` and
  `properties` public, or internal to valuation work?
- **Reverse lookups.** Should `PartiesService` gain `GetImprovementsByPartyId`
  for `improvement_parties`, as it has `GetParcelsByPartyId`?
- **Units.** Areas in square feet, as the parcel reads return them, or square
  metres as stored?
