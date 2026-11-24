# Intent: full and half bathrooms

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-24 |
| **Status** | Draft, from the owner's intent brainstorm on 2026-11-24. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

civil-os records one whole number of bathrooms per improvement, and so it can't represent a
half bath:

| Where | Today |
|---|---|
| `core.improvement_attributes.bathrooms` | `integer` |
| Public and mesh `ParcelImprovementSummary.total_bathrooms` | `int32`, the sum over a parcel's improvements |
| Per-feature bathroom values (map) | `int`, the same sum per parcel feature |
| opencama-record-keeper's parcel pane | Shows "Total bathrooms" |

Half baths matter for valuation. Appraisal models and comparable-sales selection weigh a
half bath differently from a full one, and a house with 2 full and 1 half bath isn't the
same as one with 2 or 3 full baths.

Source CAMA systems commonly record half baths as a decimal total (`2.5`, `3.5` and so
on), and a migration now in progress has tens of thousands of such buildings; its
improvements load is blocked on this change. The existing loads put their sources' full
bathroom counts into `bathrooms`, so today's values are full baths.

## Proposed outcome

1. **Two counts per improvement.** `improvement_attributes.bathrooms` is renamed
   `full_bathrooms`, keeping today's values, which are full baths. A new
   `half_bathrooms integer` column is added. Both are nullable and, when set, at least 0.
2. **A breaking public API change, with no deprecation window.** On
   `ParcelImprovementSummary` (public and mesh):
   - `total_bathrooms` is **removed**;
   - `total_full_bathrooms` and `total_half_bathrooms` are added, summed over the parcel's
     improvements as `total_bathrooms` was;
   - `total_bathroom_equivalent` is added: a decimal, `total_full_bathrooms + 0.5 ×
     total_half_bathrooms`, computed by the reader and not stored. Valuation models and
     comparables use it.

   The change ships as a breaking release of the public API (`feat!:`).
3. **The reader, gateway, fixtures and tests** move to the new columns and fields.
4. **opencama-record-keeper's parcel pane** shows full and half baths (for example "2 full,
   1 half") in the same cycle, so the client doesn't break when the API ships.

## Constraints

- **No guessing.** Existing values are treated as full baths, which is what they are.
  `half_bathrooms` starts null for every existing row, not 0.
- **One schema change, in the Atlas HCL.** The history table's matching column is renamed
  and added too, and the history trigger function is updated, as earlier column changes
  were. The owner generates and validates the migration (agents don't run Atlas).
- **Breaking-change process.** The PR is marked breaking, so `buf breaking` against the
  PR's base is expected to fail and is acknowledged, as with earlier breaking public API
  changes.
- **The mesh API changes with the public one;** nothing outside civil-os uses it.

## Success criteria

- A building recorded as `2.5` can be stored as 2 full and 1 half bath, and a parcel
  with that building alone reports `total_full_bathrooms = 2`,
  `total_half_bathrooms = 1` and `total_bathroom_equivalent = 2.5`.
- Existing data, including the synthetic fixtures, reads back with the same full-bath
  totals as before, and no half baths.
- The record keeper's parcel pane shows full and half baths against the new API.
- Integration tests cover the sums and the equivalent, including parcels whose
  improvements have null counts.

## Out of scope

- **The per-feature bathroom values used for map shading.** They keep summing full baths
  until a later change decides how the map should weigh half baths.
- **Quarter and three-quarter baths.** No source loaded so far records them.
- **Updating the data loads.** The ELTs that write `bathrooms` switch to `full_bathrooms`
  and `half_bathrooms` in their own repo, after this ships.

## Open questions (carry into spec)

1. **The equivalent's type in the API:** `double`, or a decimal string, as amounts are
   elsewhere.
2. **Null handling in the sums:** whether a parcel whose improvements all have null half
   baths reports `total_half_bathrooms` as 0 or unset, which affects the equivalent.
3. **Field numbers** for the new fields, and whether `total_bathrooms`'s number is
   reserved.
4. **The release version:** the next public API version for a breaking change, after
   `public-api-v0.5.0`.
