# Plan: fix the neighborhoods read API

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `96927c1`, #50), which implements [`intent.md`](intent.md) (#34) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-16 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One PR**, on branch `fix/neighborhoods-read-api`, titled
  `fix!: return neighborhood definitions and their neighborhoods`, as the spec
  says. The agent never pushes to `main`. The owner merges.
- **Each step is one commit** with a conventional-commit message, and every
  commit builds (`go build ./...`).
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Verification before "done".** The checks below are run locally, and their
  output goes in the PR description.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated code is committed with them.
- **Synthetic data only.** No migration, no Atlas.

## Build constraint

Retyping field 1 of the mesh `GetNeighborhoodDefinitionsResponse` breaks the
reader (which builds the old type) and the gateway (which maps it) until
both are updated. Step 3 therefore changes the mesh and public protos, the
reader and the gateway in one commit. The parcel join fix (step 2) touches no
proto, so it lands first as its own `fix:` commit with its test.

## Steps

**1. Fixtures** (`test: fixtures for a non-legal neighborhood definition and neighborhood history`)

In `database/fixtures/synthetic.sql`:
- Definition 2, `Fixture market areas`, `is_legal = false`.
- Neighborhoods 3, `Fixture Market Area`, and 4, `Unused Fixture`.
- Restrict the existing definition-1 assignment insert to definition 1. It
  `CROSS JOIN`s every definition today, so definition 2 would get legal rows,
  which the composite foreign key rejects.
- Give parcel 10 a history under definition 1: a `CASE` so its current North row
  starts 2023-01-01, plus a South row for `[2020-01-01, 2023-01-01)`.
- Definition 2 assignments, all with `is_legal = false`:
  - parcels 1–5 → neighborhood 3, with a null range;
  - parcel 6 → neighborhood 3, for `[2020-01-01, 2023-01-01)`;
  - parcel 7 → neighborhood 1, with a null range.
- Update the header comment that describes the neighborhoods.

In `internal/testutil/fixtures.go`:
- `LegalNeighborhoodDefinition = 1` and `NonLegalNeighborhoodDefinition = 2`;
- `MarketAreaNeighborhood = 3` and `UnusedNeighborhood = 4`;
- `NeighborhoodHistoryParcel = 10`, `DatedNonLegalParcel = 6` and
  `SharedNeighborhoodParcel = 7`;
- `NeighborhoodDefinitionID(n)` and `NeighborhoodID(n)`.

`TestReferenceData` in `services/database-reader/integration_test.go` expects
one definition and two neighborhoods. Update those counts in this commit, so
the suite passes after every commit.

**2. Parcel join** (`fix: return a parcel's neighborhood under a non-legal definition`)

In `services/database-reader/parcels.go`, the `pnd` join becomes
`(pnd.legal_valid_range @> $1::timestamptz OR pnd.legal_valid_range IS NULL)`.

A new `services/database-reader/neighborhoods_integration_test.go` holds the
spec's "Reader integration: parcels" row, by parcel ID and by feature ID:
- parcel 3: neighborhood 1 under definition 1, 3 under definition 2, and
  unset with no definition;
- parcel 7 under definition 2: neighborhood 1;
- parcel 6 under definition 2: unset now, and 3 at 2022;
- parcel 10 under definition 1: North now, and South at 2022;
- parcel 12 under definition 2: unset.

These tests use only `ParcelReadService`, which has no proto change.

**3. Protos, reader and gateway** (`fix!: return neighborhood definitions and their neighborhoods`)

- **Mesh** `civil.mesh.neighborhoods.v1`:
  - field 1 of `GetNeighborhoodDefinitionsResponse` becomes
    `map<string, NeighborhoodDefinition>`;
  - `Neighborhood` gets `repeated string neighborhood_definition_ids = 3`;
  - `GetNeighborhoodsRequest` gets `optional string neighborhood_definition_id = 1`.
- **Public** `civil.public.neighborhoods.v1`:
  - the same changes, with a `uuid` rule on the request field;
  - `NO_SIDE_EFFECTS` on both RPCs;
  - gnostic descriptions as the spec words them;
  - the corrected `GetNeighborhoodsResponse` description;
  - `GetNeighborhoodDefinitionsRequest` stays empty.
- **Public** `civil.public.parcels.v1`: the description changes on
  `ParcelDetails.neighborhood_id`, and on `neighborhood_definition_id` in both
  parcel-details requests.
- `mise run generate`.
- **Reader** (`neighborhoods.go`):
  - definitions select `is_legal`;
  - `GetNeighborhoods` is the single query from the spec, with neighborhoods
    `LEFT JOIN` an aggregate over `parcel_neighborhood_definitions`, non-voided
    parcels and `neighborhood_definitions`;
  - `array_agg(DISTINCT nd.public_id::text ORDER BY nd.public_id::text)`,
    with `COALESCE` to an empty array;
  - an optional `$1::uuid` filter, `WHERE $1 IS NULL OR $1 = ANY(ids)`;
  - a non-UUID filter fails with `InvalidArgument` before the query, using
    `uuid.Parse`, as `pageByID` does.
- **Gateway** (`neighborhoods.go`): map `NeighborhoodDefinition` and the new
  field, and pass the filter through.
- **`TestReferenceData`**: switch to the new response type. Its existing name
  checks stay.

**4. Tests** (`test: neighborhood definitions and neighborhoods by definition`)

- Reader, in `neighborhoods_integration_test.go`:
  - **definitions:** 1 is legal and 2 isn't, each with its name;
  - **neighborhoods with no filter:** 1 `[1, 2]`, 2 `[1]`, 3 `[2]` and
    4 `[]`;
  - **definition 1:** neighborhoods 1 and 2, with 1 still listing both
    definitions;
  - **definition 2:** neighborhoods 1 and 3;
  - an unknown UUID returns an empty map;
  - a non-UUID returns `InvalidArgument`.
- Gateway:
  - `TestPublicRPCs` and `TestPublicRPCValidation` now cover `GetNeighborhoods`'s
    ID field. Check in `-v` output that both RPCs run;
  - a new test calls both RPCs with a Connect client built with
    `connect.WithHTTPGet()`, and checks that the mesh saw the call. That proves
    `NO_SIDE_EFFECTS` reaches the handler.
- e2e (`test/e2e/smoke_test.go`), a `neighborhoods` subtest:
  - definition 1 is legal;
  - `GetNeighborhoods` for definition 2 returns neighborhoods 1 and 3;
  - parcel 3 under definition 2 returns neighborhood 3;
  - one of these calls goes over Connect GET.

**5. Docs** (`docs: how to read neighborhoods`)

In `api/proto/public/README.md`:
- a "Neighborhoods" section: pick a definition with
  `GetNeighborhoodDefinitions` and `is_legal`, list its neighborhoods with
  `GetNeighborhoods`, and read a parcel's neighborhood with
  `neighborhood_definition_id` (plus `legal_as_of` for a legal definition);
- an updated `NeighborhoodService` row.

## Checks

- `mise run check`, and `buf format --diff --exit-code` in `api/`.
- Generated-code freshness: `git diff --exit-code api/gen/go internal/gen/go`
  after `generate`.
- **`buf breaking` must fail.** Record its output in the PR description, to
  show that only the expected rules fire: the field type change and the two
  idempotency changes.
- `mise run test:integration` with the test cache cleared, and
  `mise run test:e2e`.
- Check each intent success criterion, and list the evidence in the PR
  description. Also list what an un-upgraded client sees, as the spec's
  "Release" section asks.

## Risks

| Risk | Mitigation |
|---|---|
| Parcel 10's history changes an expectation elsewhere. | Its current neighborhood is unchanged (North). Step 1 runs the full integration and e2e suites before any code changes. |
| A non-legal assignment duplicates parcels in the parcel read. | The spec shows at most one row applies per parcel and definition, through the unique index and the exclusion constraint. Parcel 3 under definition 2 is checked to appear exactly once. |
| `TestPublicRPCValidation` used to skip `GetNeighborhoods` because it had no ID field. Now it doesn't. | Expected. Confirm in `-v` output. |
| A Connect GET with an empty request isn't exercised by any client today. | The new gateway test and the e2e GET call cover it. |
