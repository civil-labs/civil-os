# Spec: full and half bathrooms

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-24 |
| **Status** | Decisions made by the owner on 2026-11-24 in the spec brainstorm. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: the schema change, the protos, the reader and gateway
changes, the fixtures, the tests and the client change. The order of the work and how the
PRs are split go in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| The equivalent's type | **`double`**, like `total_area_sq_ft`. Halves are exact in a double, and clients get a number they can compute with. |
| Null handling in the sums | **0, like the other totals.** The totals are non-optional and use `COALESCE(SUM(…), 0)`, as `total_bedrooms` and `total_units` do. A parcel with no half-bath values reports 0 half baths. |
| Field numbers | **`total_bathrooms` keeps number 4** and becomes the derived total. The two counts are new fields, 12 and 13. See [Public API](#public-api). |
| The release version | Automatic. release-please bumps the minor version for a breaking change before 1.0 (`bump-minor-pre-major`), so this ships as the next public API minor release after `public-api-v0.5.0`. |

## Correction to the intent

The intent calls the message `ParcelImprovementSummary`. Its name in both the public and
mesh protos is **`ParcelImprovementsSummary`**; this spec uses that.

## Departures from the intent

Both were decided by the owner in the spec brainstorm:

1. **`total_bathrooms` isn't removed; its meaning changes.** The intent removed it and added
   `total_bathroom_equivalent`. Instead, `total_bathrooms` keeps its name and number and
   becomes the derived total, `full + 0.5 × half`, as a `double`.
2. **The per-parcel and per-feature bathroom RPCs are in scope.** The intent left the map
   values unchanged. `GetBathroomsByParcelId`, `GetBathroomsByFeatureId` and the
   `PARCEL_ATTRIBUTE_BATHROOMS` comparables criterion return the same derived total
   instead, so "bathrooms" means one thing everywhere in the API.
3. **`primary_improvement_id` becomes `optional`.** *Amended 2026-11-24, by the owner,
   during implementation.* The e2e response check found that a parcel with no improvements
   returns `ParcelImprovementsSummary.primary_improvement_id` as `""`, which breaks the
   field's existing `string.uuid` rule. It becomes `optional string` (public and mesh, same
   number), unset when the parcel has no improvements, and its comment and OpenAPI
   description say so. It ships in the same breaking release.

## Schema

In the Atlas HCL (`database/schema/schema.pg.hcl`). The owner generates and validates the
migration; agents don't run Atlas or write migration files.

| Table | Change |
|---|---|
| `core.improvement_attributes` | Rename `bathrooms` to `full_bathrooms` (`integer`, nullable), keeping its values. Add `half_bathrooms integer` (nullable). Add a check that each is `>= 0` when set. |
| `core.improvement_attributes_history` | The same rename and new column, without the checks. |
| `core.record_improvement_attributes_history()` | Copies `OLD.full_bathrooms` and `OLD.half_bathrooms` in place of `OLD.bathrooms`. |

The rename must be a rename in the generated migration (`ALTER TABLE … RENAME COLUMN`), not
a drop and an add, or today's values are lost. The owner checks this when generating it.
Existing rows keep their counts as full baths, and `half_bathrooms` is null.

## Public API

In `civil.public.parcels.v1`:

**`ParcelImprovementsSummary`**

| Field | Before | After |
|---|---|---|
| 4 | `int32 total_bathrooms` (sum of `bathrooms`), with a `gte: 0` rule | `double total_bathrooms`: `total_full_bathrooms + 0.5 × total_half_bathrooms`. Rule `double = {gte: 0.0}`. |
| 12 | none | `int32 total_full_bathrooms`: the sum of `full_bathrooms`. Rule `int32 = {gte: 0}`. |
| 13 | none | `int32 total_half_bathrooms`: the sum of `half_bathrooms`. Rule `int32 = {gte: 0}`. |

**Comments and rules.** API consumers learn a field's possible values in two places, and
each field uses both:

- **The comment** becomes JSDoc in the TypeScript SDK and a doc comment in Go; it's the only
  thing SDK users see. It states what the field holds and every value it can take.
- **The `buf.validate` rule** becomes a JSON Schema constraint in the published OpenAPI
  (`protoc-gen-connect-openapi` turns `gte: 0` into `minimum: 0`), for clients generated
  from it. A rule can't say "in steps of 0.5", so the comment carries that.

The fields, with their comments:

```proto
// The number of bathrooms, counting a half bath as 0.5:
// total_full_bathrooms + 0.5 * total_half_bathrooms.
// Always >= 0, in steps of 0.5. 0 when the parcel has no improvements or no
// bathroom counts are recorded.
double total_bathrooms = 4 [(buf.validate.field).double = {gte: 0.0}];

// The sum of the full bathrooms across the parcel's improvements.
// Always >= 0. 0 when the parcel has no improvements or none records full baths.
int32 total_full_bathrooms = 12 [(buf.validate.field).int32 = {gte: 0}];

// The sum of the half bathrooms across the parcel's improvements.
// Always >= 0. 0 when the parcel has no improvements or none records half baths.
int32 total_half_bathrooms = 13 [(buf.validate.field).int32 = {gte: 0}];
```

The `values` fields of `GetBathroomsByParcelIdResponse` and
`GetBathroomsByFeatureIdResponse` get the same note as `total_bathrooms`: the derived
total, always >= 0, in steps of 0.5. Their rule is
`(buf.validate.field).map.values.double = {gte: 0.0}`.

**The rules are checked by tests, not at runtime.** The gateway's protovalidate
interceptor checks requests only. The e2e smoke tests below validate the public responses
they receive, so a rule the reader breaks fails CI instead of misleading a client. Checking every
response in the API that way is [`../response-validation-rules/intent.md`](../response-validation-rules/intent.md).

**Per-parcel and per-feature values**

| Message | Before | After |
|---|---|---|
| `GetBathroomsByParcelIdResponse` | `map<string, int32> values = 1` | `map<string, double> values = 1`: each parcel's derived total |
| `GetBathroomsByFeatureIdResponse` | `map<int64, int32> values = 1` | `map<int64, double> values = 1`: each feature's derived total |

`PARCEL_ATTRIBUTE_BATHROOMS = 8` keeps its name and number. The equity and sales
comparables, the only RPCs that read it (as a `criteria` attribute), return the derived
total. *Corrected 2026-11-24:* this said "filters and sorts", which no RPC offers on it.

### Why this is breaking

`buf breaking` reports field 4's and the two maps' type changes. The PR title is marked with
`!`, and the failure is expected. What clients see:

- **Go clients** get compile errors (`int32` to `float64`), which point them at every use.
- **JSON and TypeScript clients** don't. `totalBathrooms` and the map values are `number`
  either way, so they keep working and start receiving `2.5` where a parcel has half baths.
  That's the intended value.
- **Clients on binary protobuf built against the old schema** see field 4 with a different
  wire type. Protobuf runtimes, Go's included, keep a field with an unexpected wire type as
  an unknown field instead of reading it, so those clients see `total_bathrooms` as 0 until
  they regenerate.

## Mesh API

In `civil.mesh.parcels.v1`, the same changes, with the same numbers:

- `ParcelImprovementsSummary`: `double total_bathrooms = 4`, `int32 total_full_bathrooms = 12`,
  `int32 total_half_bathrooms = 13`;
- `GetBathroomsByParcelIdResponse` and `GetBathroomsByFeatureIdResponse`: `double` values.

The mesh API has no compatibility promise; nothing outside civil-os uses it.

## Reader

`services/database-reader/parcels.go`. Every sum of `attr.bathrooms` changes:

- **Summaries** (the two `imp_totals` / `imp_agg` queries): select
  `COALESCE(SUM(attr.full_bathrooms), 0)` and `COALESCE(SUM(attr.half_bathrooms), 0)`, and
  compute `total_bathrooms` from them in Go, so the formula is in one place:
  `float64(full) + 0.5*float64(half)`.
- **`getBathroomsByParcelId` and `getBathroomsByFeatureId`**: return
  `COALESCE(SUM(attr.full_bathrooms), 0) + 0.5 * COALESCE(SUM(attr.half_bathrooms), 0)` as
  `float8`, in `map[…]float64`.
- **The comparables' `PARCEL_ATTRIBUTE_BATHROOMS` criterion**: the per-parcel value is the
  derived total.

Voided parcels stay hidden (`NOT p.is_voided`), as today.

## Gateway

`services/gateway/parcels.go` maps the three summary fields and the two `double` maps from
mesh to public. Nothing else changes.

## Fixtures

`database/fixtures/synthetic.sql`:

- the `improvement_attributes` insert uses `full_bathrooms`, keeping today's values;
- at least one synthetic improvement gets `half_bathrooms = 1`, on a parcel whose expected
  totals the tests check;
- the improvements with `half_bathrooms` left null stay that way, so null handling is
  tested.

`internal/testutil` gains the expected values for that parcel if the tests need them.

## Tests

**e2e smoke tests** (`mise run test:e2e`): every public response they receive from the
summary and bathroom RPCs passes `protovalidate.Validate`, including the 2.5-bath parcel and
a parcel with no improvements. *Clarified 2026-11-24:* this said the integration tests, but
those receive mesh messages, which have no rules; only the smoke tests receive public
responses from real data.

**Integration tests** (`mise run test:integration`):

- A parcel with 2 full and 1 half bath reports `total_full_bathrooms = 2`,
  `total_half_bathrooms = 1` and `total_bathrooms = 2.5` from the summary RPC, and 2.5 from
  `GetBathroomsByParcelId` and `GetBathroomsByFeatureId`.
- A parcel whose improvements have null half baths reports `total_half_bathrooms = 0` and
  `total_bathrooms` equal to its full baths, matching today's totals for the existing
  fixtures.
- A parcel with no improvements reports 0 for all three.
- A filter or sort on `PARCEL_ATTRIBUTE_BATHROOMS` orders a 2.5-bath parcel between a 2- and a
  3-bath parcel.
- The legal-time test (`legal_time_integration_test.go`) writes and reads the renamed
  columns.
- **A new migration test** (in `database/migrations_integration_test.go`) applies the
  migrations up to the one before this change, inserts improvement attributes with
  `bathrooms` set, applies this change's migration, and finds the values in
  `full_bathrooms`, with `half_bathrooms` null. The existing tests only apply every
  migration to an empty database, so they wouldn't catch a drop-and-add.

**Handler tests:** the gateway maps the new fields and the `double` maps.

**Generated code:** `mise run generate` is rerun, and `api/gen`, `internal/gen` and
`sdk/js/gen` are committed.

## opencama-record-keeper

In its own repo, in the same cycle, against the new SDK:

- the parcel pane's "Total bathrooms" row becomes **"Bathrooms"**, built from the two
  counts: `"2 full, 1 half"`, or `"3 full"` when there are no half baths, or the usual empty
  marker when both are 0;
- any other use of `totalBathrooms` or the bathroom value maps is checked. Their values are
  now the derived total, which is what a display wants.

It ships after the civil-os release that carries this change.

## Docs to update

- The public API's changelog is written by release-please from the `feat!:` commit.

## Out of scope

- Quarter and three-quarter baths.
- Data loads: the ELTs that write `bathrooms` switch to `full_bathrooms` and
  `half_bathrooms` in their own repo, after this ships.
- Writer RPCs: the writer doesn't write improvement attributes today.
