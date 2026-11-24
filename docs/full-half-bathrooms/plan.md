# Plan: full and half bathrooms

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (#85), which implements [`intent.md`](intent.md) (#84) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-24 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The civil-os work is one PR (below) on its own branch. The agent
  never pushes to `main`; the owner merges.
- **Each step is one commit** with a conventional-commit message. Steps 1–2 change the
  schema and protos before the code that uses them, so `go build ./...` passes at every
  commit but the integration tests pass only from step 4.
- **Plan and code stay in sync.** If implementation departs from this plan, `plan.md` is
  updated in the same commit. If the departure changes the spec, the agent stops and asks
  the owner first.
- **Verification before "done".** The PR's checks are run locally, and their output goes in
  the PR description.
- **Generated code is never edited by hand.** Proto changes go through `mise run generate`,
  and `api/gen/go`, `internal/gen/go`, `sdk/js/gen` and `api/gen/openapi` are committed with
  the `.proto` change.
- **Agents never run Atlas** and never write in `database/migrations`
  (`.claude/skills/atlas-migrations/SKILL.md`). The owner generates the migration in step 1.
- **Synthetic data only.** This repo is public.

## Why one PR

The column rename breaks every query that reads `bathrooms`, and the proto type change
breaks every Go use of `TotalBathrooms`. Splitting the schema, protos and reader into
separate PRs would leave `main` with failing integration tests between them. The PR is
small enough to review as one: one HCL change, two protos, two Go files with code, the
fixtures and tests.

PR title: **`feat!: full and half bathrooms`**, on branch `feat/full-half-bathrooms`. The
squash commit carries the `!`, so release-please cuts the next public API minor version.

## Facts found while planning

- **Every reference to change** (outside generated code):
  - `database/schema/schema.pg.hcl`: `improvement_attributes.bathrooms` (line 1536),
    `improvement_attributes_history.bathrooms` (1705), and the trigger function
    `record_improvement_attributes_history` (6823, 6842).
  - `services/database-reader/parcels.go`:
    - the by-parcel summary's `imp_totals` sum (134, 199) and scan (319, 359, 425);
    - the by-feature path's `imp_agg` sum (646, 658), scan (577, 664) and the in-memory
      `Bathrooms` field (522, 845) used by filters, sorts and the numerical stats (889);
    - `getBathroomsByParcelId` (1607–1633) and `getBathroomsByFeatureId` (2162–2189).
  - `services/gateway/parcels.go`: the summary mapping (426) and the two bathroom RPCs (584,
    766), which pass the maps through.
  - `database/fixtures/synthetic.sql`: the `improvement_attributes` insert (318).
  - `services/database-reader/integration_test.go` (317–321, 403–407) and
    `legal_time_integration_test.go` (185–187).
- **Filters, sorts and numerical stats on `PARCEL_ATTRIBUTE_BATHROOMS`** already convert the
  in-memory value to `float64` (`parcels.go:889`). Changing the field to the derived total
  is enough; the stats RPCs need no other change.
- **The fixtures:** 17 improvements, each with 2 bathrooms except improvement 16 (0), which
  is the second improvement on parcel 3. Parcel 20 has improvement 17. No fixture sets a
  half bath.
- **The reader integration tests use the mesh API**, which has no validation rules. Only
  the e2e smoke tests (`test/e2e/smoke_test.go`, run by `mise run test:e2e` against the
  compose stack) receive public responses from real data, so the spec's response check
  goes there (see [Spec clarification](#spec-clarification)).
- **The migration tests** (`database/migrations_integration_test.go`) only apply every
  migration to an empty database. `testutil.MigrationFiles()` returns the files in order,
  so a test can apply some, insert rows, then apply the rest.
- **One existing data load writes `bathrooms`,** in civil-os-elt. Once a database is
  migrated, its next run fails until it writes `full_bathrooms`. See
  [Rollout](#rollout).
- **opencama-record-keeper** uses `totalBathrooms` in one place:
  `src/routes/ParcelPane.tsx:92`, the "Total bathrooms" row, tested in
  `ParcelPane.test.tsx`. It pins `@civil-labs/civil-os-api` 0.5.0.

## Spec clarification

The spec says "every response these tests receive from the summary and bathroom RPCs passes
`protovalidate.Validate`" under integration tests. The reader integration tests receive
mesh messages, which have no rules, so the check runs in the **e2e smoke tests**, which call
the gateway and receive public messages. The spec's Tests section is updated to say so in
step 5's commit. The behavior is unchanged.

## PR: `feat/full-half-bathrooms`

### Step 1: schema (`feat!: rename bathrooms to full_bathrooms and add half_bathrooms`)

In `database/schema/schema.pg.hcl`:

- `improvement_attributes`: rename the `bathrooms` column to `full_bathrooms`; add
  `half_bathrooms` (`integer`, `null = true`); add checks
  `chk_improvement_attributes_full_bathrooms` (`full_bathrooms IS NULL OR full_bathrooms >= 0`)
  and `chk_improvement_attributes_half_bathrooms` (the same for half).
- `improvement_attributes_history`: the same rename and new column, no checks.
- `record_improvement_attributes_history`: the column lists and `OLD.` values use
  `full_bathrooms, half_bathrooms`.

Then `mise run check`, and **stop for the owner**, per the Atlas skill, with:

- the HCL diff;
- the SQL expected from Atlas:
  ```sql
  ALTER TABLE core.improvement_attributes RENAME COLUMN bathrooms TO full_bathrooms;
  ALTER TABLE core.improvement_attributes ADD COLUMN half_bathrooms integer NULL,
    ADD CONSTRAINT chk_improvement_attributes_full_bathrooms CHECK (…),
    ADD CONSTRAINT chk_improvement_attributes_half_bathrooms CHECK (…);
  ALTER TABLE core.improvement_attributes_history RENAME COLUMN bathrooms TO full_bathrooms;
  ALTER TABLE core.improvement_attributes_history ADD COLUMN half_bathrooms integer NULL;
  CREATE OR REPLACE FUNCTION core.record_improvement_attributes_history() …
  ```
- **The rename risk:** Atlas may plan the column change as `DROP COLUMN bathrooms` and
  `ADD COLUMN full_bathrooms`, which loses data. If it does, the owner answers Atlas's
  rename prompt or edits the plan interactively; the agent never edits the generated file.
  Step 4's migration test fails on a drop-and-add.
- **Operational effects:** a rename and a nullable column without a default are
  catalogue-only. Adding a `CHECK` scans `improvement_attributes` under an `ACCESS
  EXCLUSIVE` lock, which takes seconds on hundreds of thousands of rows. The owner accepted
  the plain `CHECK` on 2026-11-24, instead of `NOT VALID` plus `VALIDATE CONSTRAINT`.

The owner runs `mise run db:diff` and `mise run db:validate`. The agent reviews the
generated SQL against the list above, and the migration and `atlas.sum` are committed with
the HCL.

### Step 2: protos (`feat!: full and half bathroom totals in the API`)

Public `civil/public/parcels/v1/parcels.proto`, as the spec lists:

- `ParcelImprovementsSummary`: `double total_bathrooms = 4` with `double = {gte: 0.0}`;
  `int32 total_full_bathrooms = 12` and `int32 total_half_bathrooms = 13`, each with
  `int32 = {gte: 0}`; the spec's comments, word for word.
- `GetBathroomsByParcelIdResponse` and `GetBathroomsByFeatureIdResponse`:
  `map<…, double> values = 1` with `map.values.double = {gte: 0.0}` and the comment.
- `PARCEL_ATTRIBUTE_BATHROOMS`: a comment that it's the derived total.

Mesh `civil/mesh/parcels/v1/parcels.proto`: the same fields, numbers and map types, no rules.

`buf format -w`, `mise run generate`, and commit the generated code. Go builds fail at this
commit's uses of `TotalBathrooms` and the maps, so step 2 also changes their types in the
reader and gateway mechanically (`int32` to `float64`), computing `total_bathrooms` from the
old full-bath sum for now. `buf breaking` fails as expected.

### Step 3: reader and gateway (`feat!: read full and half bathrooms`)

`services/database-reader/parcels.go`:

- both summary queries sum `attr.full_bathrooms` and `attr.half_bathrooms` with
  `COALESCE(…, 0)`, and scan them as `int32`;
- one helper, `bathroomTotal(full, half int32) float64`, returns
  `float64(full) + 0.5*float64(half)`; the summaries and the in-memory `Bathrooms` value
  (now `float64`) use it;
- `getBathroomsByParcelId` and `getBathroomsByFeatureId` select
  `(COALESCE(SUM(attr.full_bathrooms), 0) + 0.5 * COALESCE(SUM(attr.half_bathrooms), 0))::float8`
  into `map[…]float64`, keeping their voided-parcel and `legal_valid_range` filters.

`services/gateway/parcels.go`: map `TotalFullBathrooms` and `TotalHalfBathrooms`; the two
RPCs keep passing the maps through (the types now match).

Unit test: `bathroomTotal` (0/0, 2/0, 2/1, 3/2).

### Step 4: fixtures and integration tests (`test: full and half bathroom fixtures and tests`)

`database/fixtures/synthetic.sql`: the insert writes `full_bathrooms` with today's values,
and sets `half_bathrooms = 1` for improvement 1 only (parcel 1: 2 full, 1 half). Every other
improvement keeps `half_bathrooms` null. `internal/testutil` gets a constant for parcel 1's
expected totals if the tests need it.

`services/database-reader/integration_test.go`:

- the summary for parcel 1 has full 2, half 1, total 2.5; parcel 2 has full 2, half 0, total
  2 (null half baths); parcel 3 has full 2 (2 + 0), half 0; an unimproved parcel (16) has
  0, 0, 0; voided parcel 20, which has improvement 17, stays hidden;
- `GetBathroomsByParcelId` and `GetBathroomsByFeatureId` return 2.5 for parcel 1 and 2 for the
  other improved parcels (`checkValues` with `eqF`);
- a filter or sort on `PARCEL_ATTRIBUTE_BATHROOMS` puts parcel 1 (2.5) after a 2-bath parcel,
  and the numerical stats include 2.5.

`legal_time_integration_test.go`: the insert copies `full_bathrooms, half_bathrooms`.

`database/migrations_integration_test.go`, new
`TestBathroomsRenameKeepsValues`: find this change's migration (the file containing
`RENAME COLUMN "bathrooms"`, so the test doesn't hard-code a timestamp), apply every
migration before it, insert a minimal improvement with `bathrooms = 3`, apply the rest, and
check `full_bathrooms = 3` and `half_bathrooms IS NULL`. It fails on a drop-and-add.

### Step 5: e2e response check (`test: validate bathroom responses in the smoke tests`)

`test/e2e/smoke_test.go`: for parcel 1, call `GetParcelsWithImprovementSummaryByParcelId`,
`GetBathroomsByParcelId` and `GetBathroomsByFeatureId`; check the 2.5 total and run
`protovalidate.Validate` on each response. Do the same for an unimproved parcel. The spec's
Tests section is updated in this commit (see [Spec clarification](#spec-clarification)).

### Checks before pushing

- `mise run check`
- `buf format --diff --exit-code` (in `api/`)
- `mise run generate`, then `git diff --exit-code api/gen/go internal/gen/go`
- `mise run test:integration`
- `mise run test:e2e` (the gateway port 8080 must be free)
- `buf breaking`: expected to fail on field 4 and the two maps' types, and nothing else. The
  output goes in the PR description.

## opencama-record-keeper

A separate PR in that repo, **after** the civil-os public API release is published to npm:

- bump `@civil-labs/civil-os-api` to the new version;
- `ParcelPane.tsx`: replace the "Total bathrooms" row with **"Bathrooms"**, formatted by a
  small function: `"2 full, 1 half"`, `"3 full"` when half is 0, `"1 half"` when full is 0,
  and the pane's usual empty value when both are 0;
- `ParcelPane.test.tsx`: the three formats and the empty case;
- its own checks (`npm test`, typecheck, lint).

## Rollout

The migration renames a column that a data load writes. The order:

1. This PR merges; release-please opens the release PR; the owner merges it, publishing the
   images and the npm package.
2. **Before** a deployment's database is migrated, the data load that writes `bathrooms`
   (in civil-os-elt) is changed to write `full_bathrooms`, and released. Until the database
   is migrated, that version would fail instead, so the two ship together for each
   deployment: the agent prepares the civil-os-elt PR, and the owner schedules it with the
   deployment, which is done outside this repo.
3. The record keeper PR merges once the npm package is out. Until it does, the deployed
   record keeper keeps working and shows the derived total (for example `2.5`) under "Total
   bathrooms".

## Final verification

- All checks above pass, with output in the PR description.
- The record keeper shows "2 full, 1 half" for synthetic parcel 1 against the local stack.
- The spec's success criteria hold.

## Risks

| Risk | Mitigation |
|---|---|
| Atlas plans a drop and add, losing every bathroom count | The owner checks the generated SQL; `TestBathroomsRenameKeepsValues` fails on it. |
| The `CHECK` constraints lock a large table while they validate | Seconds at current sizes; accepted by the owner. |
| A data load writes `bathrooms` after its database is migrated | The rollout ships the load's change with each deployment's migration. |
| Old binary clients read `total_bathrooms` as 0 | Documented in the spec; the only known client uses JSON. |

## Options not taken

- **Several civil-os PRs** (schema, then protos, then reader): leaves `main` failing between
  them.
- **Computing `total_bathrooms` in SQL for the summaries:** the spec puts the formula in Go
  for the summaries, so there's one helper; the two value RPCs compute it in SQL because they
  return the sum directly.
- **Response validation in the reader integration tests:** they receive mesh messages, which
  have no rules.
