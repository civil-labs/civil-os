# Plan: real property transfers API

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `09afc93`, #17), which implements [`intent.md`](intent.md) (`3122113`) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-15 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The work lands in four PRs (PR 1–PR 4 below), each on
  its own short-lived branch. The agent never pushes to `main`; the owner
  merges each PR.
- **Each step is one commit** with a conventional-commit message, and every
  commit builds (`go build ./...`).
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Verification before "done".** Each PR's checks are run locally and their
  output is pasted into the PR description before the PR is reported as done.
  The customer-data denylist needs a CI secret, so CI is its first run.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated `api/gen/go` and `internal/gen/go`
  are committed in the same commit as the `.proto` change.
- **Synthetic data only.** Every fixture value is invented. Recording URIs use
  the reserved `.invalid` TLD.
- **No Atlas, no migration.** The spec needs no schema change.

## Environment

Docker is reachable from the session (server 29.4.1 on 2026-11-15), so
`mise run test:integration` and `mise run test:e2e` run locally. Nothing else
is needed beyond the repo's `mise` tools.

## PR overview

| PR | Branch | Title | Depends on | Release effect |
|---|---|---|---|---|
| 1 | `test/transfer-fixtures` | `test: synthetic real property transfer fixtures` | — | none |
| 2 | `feat/remove-unserved-services` | `feat!: remove the unserved SalesService and TileService` | — | public-api minor bump (breaking, pre-1.0) |
| 3 | `feat/transfers-reader` | `feat: serve real property transfers from the database reader` | PR 1 | Go module release |
| 4 | `feat/transfers-public-api` | `feat: public real property transfers API with cursor pagination` | PR 2, PR 3 | public-api and Go module |

PR 1 and PR 2 are independent and can be reviewed in parallel. PR 3's mesh
API is internal, so it can merge before the public service exists without
advertising anything unserved. PR 4 adds the public surface.

**Release timing.** PR 2 and PR 4 both change the public API. The owner
should merge the release-please PR only after both, so `public-api-v0.2.0`
removes the old services and adds the new one in the same release. Otherwise
there's a published SDK version with no sales API at all, which is harmless
(nothing served it) but noisier.

## Spec corrections found while planning

Two statements in the accepted spec were wrong. Neither changes the contract,
and both are fixed in `spec.md` in the same PR as this plan:

1. **`GetSalesComparables` and parcel 5.** The spec says the fixtures give
   parcels 2, 5, 6, 7, 8 and 19 a latest sale. The sales subquery
   (`services/database-reader/parcels.go:703`) takes the latest transfer per
   parcel with `DISTINCT ON`, and the handler skips candidates with no price
   (`parcels.go:1093`). Parcel 5's latest transfer is 6, which has no amount,
   so parcel 5 is **not** a sales comparable. The set is 2, 6, 7, 8 and 19.
2. **The voided-party test.** The spec says the test commits a voided party
   and then restores it. `testutil.FixtureDatabase` gives every test its own
   database copy, so the test can commit freely and nothing needs restoring.

## PR 1: fixtures (`test/transfer-fixtures`)

**1.1 Fixture rows** in `database/fixtures/synthetic.sql`, after the parcel,
party and improvement inserts, using the existing `pg_temp.fid(prefix, n)`
helper for public IDs and `trace_id = 'fixture'`:

- `core.real_property_transfer_code_types`: 1 `Fixture Qualification`.
- `core.real_property_transfer_codes`: 1 `Fixture Qualified` and
  2 `Fixture Unqualified` (type 1), 3 `Fixture Untyped` (no type).
- `core.real_property_transfers`: transfers 1–7 with the times and amounts in
  the spec's fixture table. Transfers 1–5 and 7 get `deed_book = 'FX-B<n>'`,
  `deed_page = '<n>'` and `deed_uri = 'https://recordings.example.invalid/FX-<n>'`.
  Transfer 6 has all three null and a null amount.
- `core.real_property_transfer_party_parcels`:

  | Transfer | Parcel | Grantor (share) | Grantee (share) |
  |---|---|---|---|
  | 1 | 5 | party 9 (1.0) | party 4 (1.0) |
  | 2 | 2 | party 1 (1.0) | parties 2 and 3 (0.5 each) |
  | 3 | 6 | party 10 (1.0) | party 6 (1.0) |
  | 3 | 7 | party 10 (1.0) | party 7 (1.0) |
  | 4 | 20 (voided) | party 1 (1.0) | party 10 (1.0) |
  | 5 | 19 | party 1 (1.0) | party 9 (1.0) |
  | 5 | 20 (voided) | party 1 (1.0) | party 10 (1.0) |
  | 6 | 5 | party 4 (1.0) | party 5 (1.0) |
  | 7 | 8 | party 3 (1.0) | — |

- `core.real_property_transfer_party_improvements`: transfer 3, improvement 6
  (party 10 → party 6) and improvement 7 (party 10 → party 7); transfer 4,
  improvement 17 (party 1 → party 10). Improvement *n* is on parcel *n*.
- `core.real_property_transfer_code_assignments`: transfer 1 → codes 1 and 3;
  transfer 3 → code 2.
- Extend the header comment of `synthetic.sql` with the transfer prefixes and
  the note about deliberately mismatched ownership dates.

**1.2 Test helpers** in `internal/testutil/fixtures.go`:
`TransferPrefix = "90000000"`, `TransferCodeTypePrefix = "91000000"`,
`TransferCodePrefix = "92000000"`, `TransferID(n)`, and named constants
`TransferCount = 7`, `VoidedOnlyTransfer = 4`, `PartlyVoidedTransfer = 5`,
`NoRecordingTransfer = 6` and `MissingGranteeTransfer = 7`.

**1.3 Pin `GetSalesComparables`** in
`services/database-reader/integration_test.go`: a new test asserts that
parcels 2, 6, 7, 8 and 19 come back with the expected sale time and price, and
parcel 5 doesn't (see the spec corrections above).

**Checks:** `mise run check`, `mise run test:integration`, `mise run up`
loads the fixtures without error, `mise run test:e2e`.

## PR 2: removals (`feat/remove-unserved-services`)

**2.1 Delete the protos and regenerate.** Remove
`api/proto/public/civil/public/sales/`, `api/proto/public/civil/public/tiles/`
and `api/proto/mesh/civil/mesh/sales/`, then run `mise run generate` and commit
the removed `api/gen/go` and `internal/gen/go` packages. No hand-written Go
code imports them (confirmed with `grep`).

**2.2 Document the tile endpoint in prose.**
- `api/proto/public/README.md`: replace the "defined but not served"
  paragraph with a short section on `/tiles/get_parcel_tiles/{z}/{x}/{y}` and
  its TileJSON at `/tiles/get_parcel_tiles`, behind the same bearer token. Say
  that whether the TileJSON is a guaranteed contract is decided by the
  tile-source-allowlist feature.
- `docs/architecture.md`: the same, next to the gateway's `/tiles/` proxy.

**Checks:**
- `mise run check`; `buf format --diff --exit-code` in `api/`;
  `git diff --exit-code api/gen/go internal/gen/go` after `generate`.
- `buf breaking proto/public --against '../.git#tag=public-api-v0.1.0,subdir=api/proto/public'`
  in `api/` fails only on the removed files. CI allows it because of the `!`
  in the PR title.
- `grep -rE 'sales\.v1|tiles\.v1' api/gen sdk/js/gen sdk/js/index.js` is empty
  after `generate`, and `api/gen/openapi` has no sales or tiles paths.

## PR 3: mesh API and reader (`feat/transfers-reader`)

**3.1 Mesh protos**, with no validation or gnostic annotations:
- `api/proto/mesh/civil/mesh/pagination/v1/pagination.proto`: `PageInfo`.
- `api/proto/mesh/civil/mesh/recordings/v1/recording.proto`: `Recording`.
- `api/proto/mesh/civil/mesh/transfers/v1/transfers.proto`:
  `RealPropertyTransferReadService` with the three RPCs, every message from
  the spec (`RealPropertyTransfer`, `TransferredParcel`,
  `TransferredImprovement`, `TransferShare`, `TransferParty`, `TransferCode`,
  `TransferCodeType`, the edge and the requests and responses), and
  `TransferCursor`.
- Run `mise run generate` and commit the generated code.

**3.2 Cursor** in `services/database-reader/transfers_cursor.go`:
- `encodeCursor(time, publicID, filterHash) string` and
  `decodeCursor(s, filterHash) (time, publicID, error)`: base64url without
  padding over the marshalled `TransferCursor`, version 1.
- `transferFilterHash(rpc string, ids []string, timeRange)`: SHA-256 over the
  RPC name, the sorted, de-duplicated IDs (feature IDs as decimal strings) and
  the range bounds as RFC 3339 with nanoseconds, or empty when unset, joined
  with a separator that can't appear in any of them.
- A decode failure, an unknown version or a hash mismatch returns
  `connect.CodeInvalidArgument`.
- Unit tests in `transfers_cursor_test.go` (no build tag): round trip,
  malformed base64, malformed protobuf, version 2, mismatched hash, and that
  the ID order doesn't change the hash.

**3.3 Reader handlers** in `services/database-reader/transfers.go`, on the
existing `APIServer`:
- **Page size:** unset `first` means 25; anything else is clamped to 1..100.
- **Step 1, the transfer IDs.** One query per RPC returns
  `(real_property_transfer_id, public_id, transfer_timestamp)`:
  - The filter is `public_id = ANY($ids)`, or `EXISTS` a
    `real_property_transfer_party_parcels` row on a non-voided requested
    parcel. For feature IDs, the parcels come from the current geometry's
    `feature_id`, as the existing `...ByFeatureId` reads do.
  - The time range is `transfer_timestamp >= start AND < end`, each bound
    only when set.
  - Visibility rule 4 is `EXISTS` a party-parcel row on a non-voided parcel
    `OR EXISTS` a party-improvement row on a visible improvement. Rule 2 is
    the non-voided improvement with a current `parcel_improvements` link to a
    non-voided parcel, as in the improvement-summary query
    (`parcels.go:642`).
  - The paginated RPCs add the keyset condition, then
    `ORDER BY transfer_timestamp DESC, public_id DESC LIMIT first + 1`.
    `GetTransfersById` has no limit beyond its 100 IDs.
- **Step 2, the details.** Three queries keyed by `= ANY($1)` on the step-1
  IDs:
  1. Party-parcel rows on non-voided parcels, joined to `parties` (selecting
     `is_voided`), the parcel's public ID and current `feature_id`, and
     `party_attributes` / `address_attributes` `LEFT JOIN`ed on
     `legal_valid_range @> transfer_timestamp`.
  2. The same for party-improvement rows on visible improvements.
  3. Code assignments joined to codes and `LEFT JOIN`ed to code types.
- **Step 3, assembly in Go.** Rows of voided parties are dropped from the
  share lists, but their parcel or improvement entry is kept (empty lists are
  valid). `parties` is built from the remaining shares, one per `party_id`,
  ordered by `party_id`. Entries are ordered by parcel or improvement public
  ID, and codes by code public ID, so responses are deterministic.
  `recordings` has one entry unless book, page and URI are all null.
- **Errors:** database failures return `Internal` with a generic message, as
  the other reader files do.
- Register `transfersv1connect.NewRealPropertyTransferReadServiceHandler(srv)`
  in `newHandler` in `services/database-reader/main.go`, and add the service
  name to the health checker.

**3.4 Reader integration tests** in `integration_test.go`: add a `transfers`
client to `readerClients`, then cover every row of the spec's "Reader
integration" and "Pagination (reader integration)" test rows. Two notes:
- The voided-party test runs `UPDATE core.parties SET is_voided = true ...`
  on its own fixture database; no restore is needed.
- `first = 500` is sent straight to the mesh client, which has no validation,
  to prove the reader clamps it to 100. The fixtures have only two transfers
  on parcel 5, so the test checks that the request succeeds and returns both,
  and a unit test on the clamp function checks the value itself.

**Checks:** `mise run check`, `mise run test:integration`.

## PR 4: public API and gateway (`feat/transfers-public-api`)

**4.1 Public protos**, with protovalidate rules and the same
`idempotency_level = NO_SIDE_EFFECTS` option as the other read RPCs:
- `api/proto/public/civil/public/pagination/v1/pagination.proto`: `PageInfo`.
- `api/proto/public/civil/public/recordings/v1/recording.proto`: `Recording`,
  with `string.uri` on `uri` when set.
- `api/proto/public/civil/public/transfers/v1/transfers.proto`:
  `RealPropertyTransfersService` and its messages. The requests carry the
  rules from the spec: unique, 1..100 IDs (`string.uuid` or `int64.gt: 0`),
  `time_range` with the same CEL rule as `GetSalesComparablesRequest`, and
  `first` with `int32: {gte: 1, lte: 100}`. The comments on `grantors` and
  `grantees` explain empty lists, and the comment on `transfer_amount`
  explains negative amounts, so both reach the OpenAPI spec and the SDK.
- Run `mise run generate`. `gen-index.mjs` exports the three new modules
  automatically, and the SDK job's "one export per public proto" check still
  holds.

**4.2 Gateway** in `services/gateway/transfers.go`:
- `TransferServer` with `dbReaderClient` and `logger`, like the other
  servers. Each method maps the public request to the mesh request
  field by field, sets `first = 25` when unset, passes `after` unchanged, and
  returns mesh errors with their status code, as the others do.
- Mapping functions for every message, one-to-one.
- In `main.go`, create the mesh client against the reader host and register
  the handler with the `validate` interceptor, CORS and `auth`, next to the
  other services.

**4.3 Gateway tests:**
- `handlers_test.go`: add the service to `publicServices`, which gives it the
  generic routing, round-trip and malformed-UUID tests. Add a test for
  `first = 0` and `first = 101` (rejected, no mesh call), an unset `first`
  (reaches the mesh as 25), an inverted `time_range`, duplicate IDs and 101
  IDs.
- `mappers_test.go`: a test per top-level mapper that maps a `full(...)` mesh
  `RealPropertyTransfer` (and `PageInfo`) and compares `asJSON` of the result
  with the input, as `TestMapParcelWithImprovementSummary` does. A field added
  to a mesh message and missed by the mapper then fails the test.
  (`checkMapMapper` is for map-valued fields and doesn't fit here.)

**4.4 e2e** in `test/e2e/smoke_test.go`: `GetTransfersByParcelId` for parcel 2
through the gateway with a bearer token returns transfer 2 with two grantee
shares and three parties. The existing tile check stays.

**4.5 Docs:**
- `api/proto/public/README.md`: add the transfers service, and a short
  section on the pagination pattern (Relay connections, `first` / `after`,
  the fixed 1..100 limit, opaque cursors) for later APIs to follow.
- `docs/architecture.md`: list `RealPropertyTransferReadService` among the
  reader's mesh services.
- `README.md`: the feature list says "sales"; change it to "real property
  transfers".

**Checks:** `mise run check`; `buf format --diff --exit-code`; generated-code
freshness; `buf breaking` against `public-api-v0.1.0` (only additions, plus
PR 2's removals if it's already merged); `mise run test:integration`;
`mise run test:e2e`.

## Final verification (before reporting the feature done)

- Every success criterion in the intent, checked one by one against `main`
  after PR 4 merges, with the evidence listed in PR 4's description.
- `openapi.yaml` (from `api/gen/openapi`) has the transfers paths and no sales
  or tiles paths.
- `npm pack ./sdk/js` exports `RealPropertyTransfersService`, `PageInfo` and
  `Recording`, and no `SalesService` or `TileService`.

## Risks

| Risk | Mitigation |
|---|---|
| The generic `TestPublicRPCs` checks that the mesh request equals the public request. If `fillValid` leaves the optional `first` unset, the gateway's default of 25 makes them differ. | Check what `fillValid` does with optional scalars first. If it leaves them unset, set `first` in the test request or compare with the default applied. Don't weaken the generic check. |
| `fillValid` may not produce a valid cursor, and the fake mesh doesn't decode it anyway. | Fine for gateway tests, since the gateway passes cursors through. Cursor decoding is tested in the reader. |
| Party-attribute lookups `as of transfer_timestamp` could return more than one row if attribute ranges overlapped. | Resolved in PR 3: `party_attributes` and `address_attributes` have exclusion constraints (`no_overlapping_party_attributes`, `no_overlapping_address_attributes`), so a plain `LEFT JOIN` can't duplicate a party. |
| Large multi-parcel transfers. | Accepted in the spec's "Known limits". |
| A release is cut between PR 2 and PR 4. | The owner holds the release-please PR until PR 4 merges. |

## Out of scope for this plan (per spec)

Writing transfers, polygon and amount searches, code filters, `system_as_of`,
backward pagination, `totalCount`, pagination for existing APIs, a recordings
table or API, the transfer-amount check constraint, renaming
`GetSalesComparables`, and the tile allowlist.
