# Plan: public read API for improvements by ID

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `2327c89`, #91), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-25 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The work lands in three PRs (PR 1–PR 3 below). Each
  is branched from `main` **after the previous one has merged**, never from
  another PR's branch. The agent never pushes to `main`. The owner merges each
  PR.
- **Each step is one commit** with a conventional-commit message, and every
  commit builds (`go build ./...`).
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Verification before "done".** Each PR's checks are run locally, and their
  output goes in the PR description before the PR is reported as done.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated `api/gen/go`, `internal/gen/go`
  and SDK output are committed with the `.proto` change.
- **Synthetic data only.**
- **No Atlas, no migration.** The spec's indexes all exist (checked again
  while planning, in `database/schema/schema.pg.hcl`).
- **Every new query filters voided rows** and resolves every attributes, link
  and geometry row with `legal_valid_range @> t`, per `CLAUDE.md`'s known
  mistakes.
- **Nothing assumes one parcel per improvement.** No code path takes
  `parcels[0]`, no query uses `LIMIT 1` or a scalar subquery on
  `parcel_improvements`, and no test asserts a length of one except as a
  fact about a named fixture. The one-parcel constraint will be relaxed.

## Environment

Docker is reachable from the session (server 29.4.1 on 2026-11-25), so
`mise run test:integration` and `mise run test:e2e` run locally.

## PR overview

| PR | Branch | Title | Release effect |
|---|---|---|---|
| 1 | `feat/improvement-qualities-and-fixtures` | `feat: improvement qualities lookup and improvement read fixtures` | public-api minor and Go module |
| 2 | `feat/improvements-by-id` | `feat: read improvements by ID` | public-api minor and Go module |
| 3 | `feat/improvements-by-party` | `feat: improvements by party` | public-api minor and Go module |

PR 1 adds every new fixture and the qualities lookup. The lookup is the
smallest RPC that reads the new quality rows. Shipping the fixtures first runs
the whole existing suite against them with no new read code, so any change to
an existing expectation shows up on its own. PR 2 and PR 3 are vertical
slices: mesh proto, reader, public proto, gateway, tests and docs. Each is a
complete, additive public change, so a release cut between them is harmless.

## Findings from planning (no spec change)

- **Existing summary tests aren't affected by the new fixtures.**
  `TestParcelsWithImprovementSummary` and the per-parcel checks in
  `integration_test.go` query parcels 1, 3, 13 and 17 (`queried`), and the
  e2e bathrooms test queries parcels 1 and 17. Improvement 18 changes parcels
  11 and 12, and improvement 4's history changes parcel 4 before 2023-06-01,
  so none of these move. `TestLegalAsOf`'s improvement test adds a pre-2020
  row to improvement **5**, so it doesn't collide with improvement 4's new
  history. The migrations test counts primary improvements, which stays 15
  because improvement 18 isn't primary. PR 1 runs the full suites to confirm
  this.
- **Reusable reader pieces:** `pageByID` and the `IdCursor` helpers
  (`id_cursor.go`), `visibleParcelJoinsSQL` and the party visibility joins
  (`parties.go`), `visibleAddressCTE` (`addresses.go`), and `collectRows` /
  `dbError`. `visibleImprovementSQL` in `transfers.go` resolves links at
  `now()`, which is right for transfers. It isn't reused here, because these
  reads take `legal_as_of`.
- **Gateway test lists:** `TestAddressPartyPageSizeDefault`
  (`address_party_test.go`) checks a fixed list of paginated RPCs, so PR 3
  adds `GetImprovementsByPartyId` to it. `TestPublicRPCs` and the
  validation tests in `handlers_test.go` find new RPCs by reflection over
  `publicServices`, which already includes `ImprovementsService`.
- **Valuations follow the spec's rule:** the `improvement_valuations` row at
  `t`. If [`../valuation-legal-time/intent.md`](../valuation-legal-time/intent.md)
  settles on a different rule, that feature's plan changes this read too.

## PR 1: fixtures and qualities (`feat/improvement-qualities-and-fixtures`)

**1.1 Fixtures** (`test: fixtures for improvement parties, history and a moving improvement`)

In `database/fixtures/synthetic.sql`, using `pg_temp.fid(prefix, n)` and
`trace_id = 'fixture'`, all from 2020-01-01 unless stated otherwise:

- **Qualities.** Extend the qualities insert to two rows. Add
  `improvement_quality_attributes`: quality 1 `Fixture Standard` with
  modifier `1.0`, and quality 2 `Fixture Premium` with modifier `1.2`.
- **Improvement 18.**
  - Extend the improvements insert to `generate_series(1, 18)` (`FX-IMP-18`).
  - Give it its own attributes insert, because the shared insert's formulas
    don't fit it: type 1, address 12, `area_sq_m` 20, `year_built` 2001,
    and null condition, quality, `full_bathrooms`, `half_bathrooms`,
    `bedrooms`, `effective_year_built` and `units`.
  - Give it its own `parcel_improvements` rows, both with `is_primary =
    false`: parcel 11 for 2020-01-01..2024-01-01, and parcel 12 from
    2024-01-01.
  - The shared attributes, links and valuations inserts keep
    `generate_series(1, 17)`, so improvement 18 has no valuation row.
- **Improvement 4's history.** The shared attributes insert gets a `CASE` on
  `legal_valid_range`, so improvement 4's current row starts 2023-06-01.
  Add an earlier row for improvement 4: condition 3, quality 1, address 4,
  type 1, 100 m², 1 full bath, null half baths, 2 bedrooms, `year_built`
  1984, `effective_year_built` 1984, 1 unit, valid 2020-01-01..2023-06-01.
- **`improvement_parties`**, one insert from a `VALUES` list of
  (improvement, party, share, range):
  - improvement 2: party 2 at `0.5000` and party 3 at `0.5000`;
  - improvement 3: party 3 at `1.0000`;
  - improvement 16: party 5 at `0.5000` and voided party 11 at `0.5000`;
  - improvement 6: party 10 at `1.0000` for 2020-01-01..2023-09-01, then
    party 6 at `1.0000` from 2023-09-01;
  - improvement 17: party 10 at `1.0000`.
- **The header comment** gets each new shape: two qualities, improvement 18
  and its move, improvement 4's history, and the improvement parties.

**1.2 Test helpers** in `internal/testutil/fixtures.go`:
- New constants:
  - `ImprovementCount = 18`
  - `MovingImprovement = 18`, `MovingImprovementEarlierParcel = 11` and
    `MovingImprovementParcel = 12`
  - `HistoricalImprovement = 4`
  - `SecondaryImprovement = 16`
  - `OwnerChangeImprovement = 6`
  - `UnusedQuality = 2`
- New `time.Time` values: `ImprovementHistoryChange = 2023-06-01`,
  `ImprovementOwnerChange = 2023-09-01`, and `ImprovementMove = 2024-01-01`.
  `ImprovementMove` is the same instant as `GeometryChange` but a separate
  name, so the two stay independent.
- Helpers `ImprovementID(n)` and `ImprovementQualityID(n)`, matching
  `PartyID`.
- `ImprovementAreaSqM(n)`, the current area: `120 + 5n`, 30 for 16 and 20
  for 18.
- Update the comment on `ImprovedParcels`: active parcels 1..15 still have
  primary improvement n. Improvement 18 is a non-primary improvement on
  parcel 11, then on parcel 12.

**1.3 Mesh proto and reader: qualities** (`feat: serve improvement qualities from the database reader`)
- `civil.mesh.improvements.v1`: `ImprovementQuality` (`id`, `name`,
  `double quality_adjustment_modifier`), `GetImprovementQualities` on
  `ImprovementsReadService`, and its request and response. Run
  `mise run generate`.
- `services/database-reader/improvements.go`: `GetImprovementQualities`,
  copying the conditions query: `NOT q.is_voided`, attributes at
  `CURRENT_TIMESTAMP`, and a map keyed by `public_id`.

**1.4 Public proto and gateway** (`feat: public improvement qualities lookup`)
- `civil.public.improvements.v1`: the message and RPC
  (`NO_SIDE_EFFECTS`), with field comments as the spec writes them. Run
  `mise run generate`, and check that the SDK index exports it.
- `services/gateway/improvements.go`: the handler and
  `mapImprovementQuality`, used per map entry.
- `services/gateway/improvements_test.go` (new; PR 2 and PR 3 extend it): a
  `checkFullMapping` round trip for `ImprovementQuality`. *Amended while
  implementing:* `checkFullMapping` lives in the gateway's `_test.go` files
  for each service, not `mappers_test.go`.

**1.5 Tests** (`test: reader tests for qualities and the new fixtures`)
- **Reader integration:**
  - `GetImprovementQualities` returns two entries with their names and
    modifiers.
  - A quality voided in the test is absent. The test runs on its own
    fixture database, from `tu.FixtureDatabase(t)`.
- **Fixture pins** in a new `improvements_integration_test.go`, which PR 2
  extends. These go through existing RPCs, so they also protect the
  fixtures:
  - `GetImprovementsByAddressId(12)` returns improvements 12 and 18 now.
    At 2022 it returns them too, with improvement 18 on parcel 11.
  - Parcel 12's summary `improvement_ids` has improvements 12 and 18, with
    `primary_improvement_id` 12. At 2022 parcel 11's summary has 11 and 18.
  - Parcel 4's summary `total_area_sq_ft` is 140 m² now and 100 m² at 2022,
    both converted to square feet.
- **e2e:** one `GetImprovementQualities` read in `test/e2e/smoke_test.go`.

**1.6 Docs** (`docs: improvement qualities lookup`): add the RPC to
`api/proto/public/README.md`.

**Checks:**
- `mise run check`, and `buf format --diff --exit-code` in `api/`.
- Generated-code freshness.
- `buf breaking` passes without `!`.
- `mise run test:integration` passes with no existing test changed.
- `mise run up` loads the fixtures, and `mise run test:e2e` passes.

## PR 2: improvements by ID (`feat/improvements-by-id`)

**2.1 Mesh proto** (one commit with 2.2, `feat: serve improvements by ID from the database reader`,
because a new RPC on an existing service doesn't build until the reader
implements it):
- On `ImprovementsReadService`: `GetImprovementsById`, with the request
  (`improvement_ids`, `legal_as_of`, `valuation_id`) and the response
  (`map<string, Improvement> improvements`).
- Messages `Improvement`, `ImprovementParcel` and `ImprovementShare`, with
  the spec's field numbers and no validation.
- `mise run generate`.

**2.2 Reader** in `services/database-reader/improvements.go`:
- **Step 1, the main query.** `$1` is `t`, `$2` the ID list and `$3` the
  optional valuation ID.
  - A CTE `vis` selects improvements with `public_id = ANY($2::uuid[])`
    and `NOT i.is_voided`, joined to `improvement_attributes` at `t`, with
    `EXISTS` a link at `t` to a parcel that passes `NOT p.is_voided` plus
    `visibleParcelJoinsSQL` (rule 1).
  - Left joins:
    - the address: `NOT is_voided`, with `address_attributes` at `t`.
      Select its `public_id` only when the attributes matched, as
      `visiblePartyColumnsSQL` does for mailing addresses.
    - the condition and the quality, each with `NOT is_voided` and its
      attributes at `CURRENT_TIMESTAMP`. Select the ID only when the
      attributes matched.
    - `improvement_valuations` on the valuation whose `public_id = $3` and
      `legal_valid_range @> t`, giving `market_value::numeric(19,4)::text`
      and `assessed_value::numeric(19,4)::text`.
  - `area_sq_m * 10.7639` into `area_sq_ft`. Use the parcel reads' constant,
    shared if it's still a literal in several places.
  - `properties::text`.
- **Step 2, parcel links.** Key on `= ANY` of `vis`'s internal IDs:
  `parcel_improvements` at `t`, joined to visible parcels, giving
  `public_id`, `pg.feature_id` and `is_primary`. Order by improvement, then
  parcel `public_id` (rule 2).
- **Step 3, parties.** Key the same way: `improvement_parties` at `t`,
  joined to parties with `NOT is_voided` and `party_attributes` at `t`,
  giving `public_id` and `ownership_share::text`. Order by party
  `public_id` (rule 3).
- **Assembly in Go:**
  - Build the messages from the three steps.
  - Set `bathrooms` when either count is set, as `full + 0.5 * half`, with
    an unset count taken as 0. Put this in a small helper with unit tests in
    `bathrooms_test.go`.
  - An improvement missing from step 2's results can't happen, because rule
    1 checks the same join. If it does, log it and leave the improvement
    out, so the "never empty" promise holds.
- **Validation:** none in the reader, as in `GetPartiesById` and
  `GetAddressesById`: the gateway's protovalidate rules check the IDs.
  *Amended while implementing:* the plan said the other by-ID reads check
  UUIDs in the reader. They don't.
- **Area:** a named `sqFtPerSqM` constant for the new code. Replacing the
  existing `10.7639` literals in `parcels.go` and `zoning.go` is left out of
  this feature.

**2.3 Reader integration tests** (`test: reader tests for improvements by ID`),
in `improvements_integration_test.go`:
- Every case in the spec's "Reader integration: by ID" row.
- The "agreement with the parcel summary" row:
  - For each parcel, read the summary, call `GetImprovementsById` on its
    `improvement_ids` at the same `t`, and compare the sums of area,
    bedrooms, full, half and total bathrooms, and units.
  - Check that exactly one returned improvement has a link to *that* parcel
    with `is_primary`, and that it is `primary_improvement_id`.
  - With valuation 1, compare summed values as decimals (`math/big` or
    `shopspring` if it's already a dependency), not as floats.
  - Run for parcels 1–15 now, and parcels 4, 11 and 12 at 2022-06-01.
  - Write the loop per parcel, with no assumption about how many parcels an
    improvement sits on.
- Tests that void an improvement or address run on their own fixture
  database.

**2.4 Public proto** (one commit with 2.5, `feat: public improvements by ID`,
because the gateway doesn't build until it implements a new public RPC.
*Amended while implementing.*):
- The RPC and messages, with the spec's comments.
- Request rules: `improvement_ids` unique, 1..100, UUID items;
  `legal_as_of` with the existing rule and description; `valuation_id`
  `string.uuid`.
- Response fields carry the rules the spec gives: `gte: 0` on counts and
  area, `min_items: 1` on `parcels`, and the money `pattern` on the values.
  *Amended while implementing:* protovalidate allows `unique` only on lists
  of scalars, so "each once" on `parcels` and `parties` is stated in their
  comments instead.
  Ranges also go in comments, per
  [`../response-validation-rules/intent.md`](../response-validation-rules/intent.md).
- `mise run generate`.

**2.5 Gateway** (`feat: serve improvements by ID from the gateway`):
- `services/gateway/improvements.go`: a handler plus `mapImprovement`,
  `mapImprovementParcel` and `mapImprovementShare`, one-to-one.
- `improvements_test.go`: `checkFullMapping` for all three.
- `handlers_test.go` already reaches the RPC through `publicServices`.
  Confirm that `fillValid` fills `valuation_id` with a UUID.

**2.6 e2e and docs** (`docs: improvements by ID`):
- `test/e2e/smoke_test.go`: read improvements 1 and 16, and validate the
  response against its own protovalidate rules, as the bathrooms smoke test
  does.
- `api/proto/public/README.md`: the RPC.
- `docs/architecture.md`: the reader's improvements mesh service, if its
  RPCs are listed there.

**Checks:** as PR 1, plus `npm pack ./sdk/js` exports `Improvement`.

## PR 3: improvements by party (`feat/improvements-by-party`)

**3.1 Mesh proto and reader** (`feat: serve improvements by party from the database reader`):
- On `ImprovementsReadService`: `GetImprovementsByPartyId`, with
  `PartyImprovement`, `PartyImprovementEdge`, and the request (`party_id`,
  `legal_as_of`, `first`, `after`) and response (`edges`, `page_info`).
  `mise run generate`.
- **Reader:** `pageByID("GetImprovementsByPartyId", ...)` with a `read`
  function:
  - First, the party must be visible at `t` (`NOT is_voided`, plus
    `party_attributes` at `t`). If it isn't, the result is no edges.
  - `improvement_parties` at `t` for that party, joined to improvements
    that pass rule 1. Write rule 1 once in 2.2 as a shared SQL fragment, and
    reuse it here.
  - The keyset `i.public_id > $last`, `ORDER BY i.public_id LIMIT $limit`.
  - `ownership_share::text`.
- **Tests:** the spec's "Reader integration: by party" row, including the
  cursor reuse cases (another party, `GetImprovementsByAddressId`, another
  `legal_as_of`) and `first = 500` clamped to 100.

**3.2 Public proto, gateway, e2e and docs**, as in 2.4–2.6:
- the RPC, `PartyImprovement` and `PartyImprovementEdge`, with `party_id`
  required as a UUID, `first` 1..100, and `after` 1..512;
- `mapPartyImprovement`, with a `checkFullMapping` round trip;
- `GetImprovementsByPartyId` added to `TestAddressPartyPageSizeDefault`'s
  list;
- an e2e read for party 3, returning improvements 2 and 3;
- the README entry.

**Checks:** as PR 2.

## Final verification (before reporting the feature done)

Check against `main` after PR 3 merges, and list the evidence in PR 3's
description:
- Each intent success criterion:
  - every improvement ID returned by the parcel summary, the address lookup
    and the transfers API can be read, except improvement 17;
  - improvement 16's party differs from parcel 3's owner;
  - improvement 4 at 2022 has its earlier values.
- `openapi.yaml` has the three new paths.
- `npm pack ./sdk/js` exports the new messages.

## Risks

| Risk | Mitigation |
|---|---|
| Improvement 4's earlier attributes row overlaps its current one, so the exclusion constraint rejects the fixtures. | The `CASE` in 1.1 moves the current row's start to 2023-06-01 in the same insert. `mise run up` and the migrations test load the fixtures in PR 1, before any new code exists. |
| Improvement 18's two links share the boundary instant 2024-01-01. | `tstzrange` defaults to `[)`, so the ranges meet without overlapping. The pin tests read at 2022 and now, away from the boundary. |
| The `numeric(5,4)` share prints as `0.5000`, and the spec's tests expect that. | Same as `PartyParcel`. Cast with `::text`, and assert the exact string. |
| Comparing summed money values as floats gives spurious failures. | 2.3 compares decimals. |
| Condition and quality lookups resolve at now, while the improvement is read at `legal_as_of`. | The spec intends this (Known limits). A test voids condition 3 and checks that improvement 4 at 2022 has `condition_id` unset, so the "readable now" rule is pinned. |
| A future relaxation of the one-parcel constraint breaks an assumption hidden in code. | The ground rule above. In review, grep the diff for `[0]` on `parcels` and for `LIMIT 1` near `parcel_improvements`. |

## Out of scope for this plan (per spec)

Geometry and extents; the modifier overrides; `legal_as_of` on the type,
condition and quality lookups; writing improvements; `system_as_of`; an
improvements-by-parcel RPC; changing `AddressImprovement.parcel_ids`;
relaxing the one-parcel constraint; the valuation legal-time rule
([`../valuation-legal-time/intent.md`](../valuation-legal-time/intent.md));
backward pagination and `totalCount`.
