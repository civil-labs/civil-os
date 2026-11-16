# Plan: public read API for addresses and parties, and parcel extents

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `1684a69`, #44), which implements [`intent.md`](intent.md) (#35) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-16 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The work lands in three PRs (PR 1–PR 3 below), each on
  its own short-lived branch. The agent never pushes to `main`. The owner
  merges each PR.
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
- **No Atlas, no migration** (see the spec correction below).
- **Every new query filters voided rows** (`NOT x.is_voided`), per
  `CLAUDE.md`'s known mistakes, and resolves every attributes, link and
  geometry row with `legal_valid_range @> t`.

## Environment

Docker is reachable from the session (server 29.4.1 on 2026-11-16), so
`mise run test:integration` and `mise run test:e2e` run locally.

## PR overview

| PR | Branch | Title | Depends on | Release effect |
|---|---|---|---|---|
| 1 | `fix/legal-time-and-voided-reads` | `fix: resolve addresses and owners at the legal time and hide voided records` | — | Go module patch |
| 2 | `feat/address-party-read-api` | `feat: public addresses and parties read API` | PR 1 | public-api minor and Go module |
| 3 | `feat/parcel-extents-transfers-by-party` | `feat: parcel extents and transfers by party` | PR 2 | public-api minor and Go module |

PR 1 is the fixes and every new fixture, as the spec requires: the fixes merge
before the new RPCs. PR 2 and PR 3 are vertical slices, each covering mesh
proto, reader, public proto, gateway, tests and docs. Each is a complete,
additive public change, so a release cut between them is harmless and the
release-please PR needn't be held. PR 3 depends on PR 2 only for the shared
`IdCursor` and fixtures helpers. Its two RPCs don't use the address or party
packages.

## Spec correction found while planning

**The transfer party tables are already indexed by party.** The spec says they
"are indexed by transfer and by parcel, but not by party" and asks for an
`EXPLAIN` check before planning. The schema has `idx_rpt_party_parcels_party_id`
and `idx_rpt_party_improvements_party_id` (`database/schema/schema.pg.hcl`,
`real_property_transfer_party_parcels` and `..._party_improvements`). So
`GetTransfersByPartyId` needs no migration, and the `EXPLAIN` check is moot.
`spec.md`'s "Reader query shape" is corrected in the same PR as this plan. The
contract doesn't change.

## PR 1: fixes and fixtures (`fix/legal-time-and-voided-reads`)

**1.1 Fixtures** (`test: fixtures for history, a voided party and a parcel without geometry`)

In `database/fixtures/synthetic.sql`, using `pg_temp.fid(prefix, n)` and
`trace_id = 'fixture'`:

- **Voided party 11.** Extend the parties insert to `generate_series(1, 11)`.
  Then:
  - void it with `voided_at = '2025-06-01'`;
  - attributes `Fixture Owner 11`, mailing address 11, from 2020-01-01 (the
    existing party-attributes insert covers it once the series is 1..11);
  - add a `parcel_parties` row: parcel 4, party 11, `0.5000`, from 2020-01-01.
- **Party 8's history.** The existing party-attributes insert gets a `CASE`
  so party 8's current row starts 2023-01-01. Add a row for party 8:
  `Fixture Owner 8 Formerly`, mailing address 18, 2020-01-01..2023-01-01.
- **Address 5's history.** The existing address-attributes insert gets a
  `CASE` so address 5's current row starts 2023-01-01. Add a row for address 5:
  - line `5 Old Fixture Way`;
  - formatted `5 Old Fixture Way, Testville ZZ000`;
  - locality, postal code, country and area as the current row;
  - null coordinates;
  - valid 2020-01-01..2023-01-01.
- **Parcel 9 redrawn.** The existing geometry insert gets a `CASE` so parcel
  9's current row starts 2024-01-01. Add an earlier geometry row:
  - feature 1109;
  - the west half of parcel 9's square (`ST_MakeEnvelope` with
    `xmax = xmin + 0.0002`);
  - valid 2020-01-01..2024-01-01.
- **Parcel 21 without geometry.**
  - The parcels insert goes to `n <= 21`.
  - The geometry insert keeps `n <= 20`.
  - Parcel 21 gets its own attributes insert, with address 2 and land use 1,
    as the spec says, from 2020-01-01. The shared insert's formulas would give
    address 21 and land use 3, so it keeps `n <= 20`.
  - The ownership insert takes party 2 at `1.0000` for parcel 21. Today's
    formula, `((21-1) % 10) + 1 = 1`, would make party 1 the owner, so the
    insert needs an explicit row.
  - No improvement, no affordance, no neighborhood. The affordance and
    neighborhood inserts are checked to make sure they don't pick parcel 21 up
    through `fx`.
- **The header comment** gets each new shape, the extended parcel range
  (1..21, where 21 has no geometry), and party 11.

**1.2 Test helpers** in `internal/testutil/fixtures.go`:
- `ParcelCount = 21` and `ActiveParcelCount = 20`. Parcel 21 isn't voided.
  Update the comments.
- New constants:
  - `NoGeometryParcel = 21`
  - `RedrawnParcel = 9` and `RedrawnParcelEarlierFeature = 1109`
  - `VoidedParty = 11` and `VoidedPartyParcel = 4`
  - `HistoricalParty = 8` and `HistoricalPartyEarlierAddress = 18`
  - `HistoricalAddress = 5`
  - `HistoryChange = 2023-01-01` (a `time.Time`) and
    `GeometryChange = 2024-01-01`
- Helpers `PartyID(n)` and `AddressID(n)`, matching `ParcelID`.
- `database/migrations_integration_test.go` uses the counts and picks up the
  new values. Its primary-improvement count stays 15.
- `TestSalesComparables` loops over `ParcelCount`, so it now requests parcel
  21. Parcel 21 has no transfer, so the expected set doesn't change. The test
  run proves it.

**1.3 F1 and F2: parcel details** (`fix: resolve parcel addresses and owners at the legal time`)

In `services/database-reader/parcels.go`, `getParcelsWithImprovementSummary`:
- **Address (F1).** Change the joins to
  `LEFT JOIN core.addresses a ON pa.address_id = a.address_id AND NOT a.is_voided`
  and
  `LEFT JOIN core.address_attributes aa ON aa.address_id = a.address_id AND aa.legal_valid_range @> $1`.
  Select `a.public_id` only when `aa` matched, with `CASE WHEN aa.address_id
  IS NOT NULL`, so an address with no attributes at `t` sets neither field
  (rule 1).
- **Owners (F2).** In the `parties_agg` subquery:
  - `JOIN core.parties pty ON ... AND NOT pty.is_voided`;
  - `JOIN core.party_attributes pa_attr ON ... AND pa_attr.legal_valid_range @> $1`.
    This is an inner join: a party with no attributes at `t` isn't visible.
  - `LEFT JOIN core.addresses p_a ON p_a.address_id = pa_attr.address_id AND NOT p_a.is_voided`
    and `LEFT JOIN core.address_attributes p_aa ON p_aa.address_id = p_a.address_id AND p_aa.legal_valid_range @> $1`.

  The aggregate's order is unchanged. The exclusion constraints guarantee one
  attributes row per party at `t`, so the array has one entry per party.

**1.4 F3: comparables** (`fix: resolve comparables at the current legal time`)

In `getCompParcels` (`parcels.go`, the joins list at about line 553), add
`legal_valid_range @> NOW()` to the `parcel_attributes`, `address_attributes`
and `parcel_geometry` joins. Apply rule 1 to the address as in 1.3. The
polygon filter uses `pg.geom_web`, so it now sees only the current geometry.
Check the other joins in the sales and equity branches, such as land use
through `pa`, and fix any that read history the same way. Record what was
found in the PR description.

**1.5 F4: transfer parties** (`fix: hide voided mailing addresses on transfer parties`)

In `services/database-reader/transfers.go` (`partyColumnsSQL`'s joins), join
`core.addresses` with `NOT is_voided` before `address_attributes`, so a voided
address leaves `mailing_address` unset. The time condition is unchanged.

**1.6 Tests** in `services/database-reader/integration_test.go` and
`transfers_integration_test.go`. These cover the spec's "Reader integration:
fixes" row and pin the fixtures:
- F1: parcel 5 returns `5 Fixture Way, ...` now, and `5 Old Fixture Way, ...`
  at 2022. An address voided in the test leaves parcel 3's `address_id` and
  `formatted_address` unset.
- F2: parcel 4's `party_ids` is party 4 only. Parcels 8 and 18 have one
  `party_ids` entry and `primary_owner_name` `Fixture Owner 8`, and at 2022
  `Fixture Owner 8 Formerly` with `primary_owner_address` from address 18.
- F3: equity comparables for parcels 5 and 9 return parcel 5's current
  address and parcel 9's feature 1009 (not 1109), each exactly once, whatever
  the row order. The earlier geometry lies inside the current square, so no
  polygon can tell the two apart. The feature ID is the check.
- F4: voiding party 9's mailing address in the test leaves transfer 1's party
  9 `mailing_address` unset, with its name still set.
- `TestLegalAsOf` and the existing transfer tests still pass unchanged.

**Checks:** `mise run check`, `mise run test:integration`, `mise run up`
loads the fixtures, `mise run test:e2e`.

## PR 2: addresses and parties (`feat/address-party-read-api`)

**2.1 Mesh protos** (`feat: mesh addresses and parties read services`), with no
validation or gnostic annotations:
- `civil.mesh.pagination.v1`: add `IdCursor` (`version`, `last_id`,
  `filter_hash`).
- `api/proto/mesh/civil/mesh/parties/v1/parties.proto`:
  - `PartiesReadService` with `GetPartiesById` and `GetParcelsByPartyId`;
  - messages `Party`, `PartyParcel` and `PartyParcelEdge`, with the requests
    and responses.
- `api/proto/mesh/civil/mesh/addresses/v1/addresses.proto`:
  - `AddressesReadService` with its four RPCs;
  - messages `Address`, `Country`, `AdministrativeArea`, `AddressParcel`,
    `AddressImprovement`, the edges (`PartyEdge` imports mesh `Party`), and
    the requests and responses;
  - `google.type.LatLng` for coordinates.
- `mise run generate`, and commit the generated code.

**2.2 Shared cursor helper** in `services/database-reader/id_cursor.go`:
- `encodeIDCursor(lastID, hash) string` and
  `decodeIDCursor(s, hash) (lastID, error)`: base64url without padding over the
  marshalled `IdCursor`, version 1.
- `idFilterHash(rpc, id string, legalAsOf *timestamppb.Timestamp) []byte`:
  SHA-256 over the RPC name, the ID and `legal_as_of` as RFC 3339 with
  nanoseconds, or an "unset" marker. The fields are joined with the separator
  `transfers_cursor.go` uses.
- `pageSize(first *int32) int` is shared with transfers if it isn't already.
  Unset gives 25, and anything else is clamped to 1..100.
- Decode failure, an unknown version or a hash mismatch returns
  `InvalidArgument`.
- Unit tests in `id_cursor_test.go`: round trip, malformed base64, malformed
  protobuf, version 2, hash mismatch, a different RPC name, a different
  `legal_as_of`, and set versus unset `legal_as_of`.

**2.3 Reader: parties** (`feat: serve parties from the database reader`) in
`services/database-reader/parties.go`:
- **`GetPartiesById`:** one query on `public_id = ANY($ids)`:
  - `NOT is_voided`;
  - `JOIN party_attributes` at `t` (rule 2);
  - `LEFT JOIN addresses` with `NOT is_voided` and `address_attributes` at
    `t` for the mailing fields, which are set only when the attributes
    matched.

  The result is a map keyed by `public_id`.
- **`GetParcelsByPartyId`:** resolve the party's internal ID. The party must
  be visible at `t` (rule 2), otherwise the result is no edges. Then:
  - read `parcel_parties` at `t`, joined to visible parcels (`NOT is_voided`,
    plus geometry and attributes at `t`, which gives `feature_id`);
  - keyset `p.public_id > $last`, `ORDER BY p.public_id LIMIT first + 1`;
  - `ownership_share::text`, which is already 4 places from `numeric(5,4)`.
- Register the handler in `newHandler`, and add the service name to the health
  checker.

**2.4 Reader: addresses** (`feat: serve addresses from the database reader`)
in `services/database-reader/addresses.go`:
- **`GetAddressesById`:**
  - `NOT is_voided`, and `JOIN address_attributes` at `t`;
  - `LEFT JOIN` countries and administrative areas;
  - `ST_X(coordinates)` and `ST_Y(coordinates)` into `LatLng`
    (`latitude = ST_Y`, `longitude = ST_X`), unset when null.
- **The three reverse lookups** each first check that the address is visible
  at `t` (rule 1), returning no edges if not. Then:
  - **parcels:** `parcel_attributes.address_id` at `t` on visible parcels;
  - **improvements:**
    - `improvement_attributes.address_id` at `t`;
    - `NOT i.is_voided`;
    - `parcel_ids` aggregated from `parcel_improvements` at `t` on visible
      parcels, ordered by parcel `public_id`;
    - `HAVING count(...) > 0` (rule 4);
  - **parties:** `party_attributes.address_id` at `t` on non-voided parties,
    returning full `Party` rows. Reuse the select from 2.3.

  Each uses the keyset on the returned record's `public_id`.
- Register the handler and the health name.

**2.5 Reader integration tests** (`test: reader tests for addresses and parties`):
- Add `addresses` and `parties` clients to `readerClients`.
- Cover every case in the spec's "Reader integration: addresses",
  "...: parties" and "...: pagination" rows.
- Tests that void a record do it on their own fixture database.

**2.6 Public protos** (`feat: public addresses and parties services`):
- `api/proto/public/civil/public/parties/v1/parties.proto` and
  `.../addresses/v1/addresses.proto`.
- Requests carry the spec's rules:
  - unique, 1..100 UUID lists;
  - a single required UUID (`string.uuid`; an empty string fails it);
  - `legal_as_of` with the existing rule and description;
  - `first` with `int32: {gte: 1, lte: 100}`.
- Every RPC is `NO_SIDE_EFFECTS`.
- Response fields carry no protovalidate rules, as the transfers PR decided.
  Limits go in comments.
- `mise run generate`. `gen-index.mjs` exports the new modules, and the SDK
  job's one-export-per-proto check still holds.

**2.7 Gateway** (`feat: serve addresses and parties from the gateway`):
- **Servers:** `services/gateway/parties.go` and `addresses.go`, with servers,
  one-to-one mappers, `first = 25` when unset, and cursors passed through.
- **Clients and handlers:** in `main.go`, create the mesh clients against the
  reader host, and register both handlers with `validate`, CORS and `auth`.
- **Tests:**
  - add both services to `publicServices` in `handlers_test.go`, plus the
    fake mesh if it lists services;
  - add specific tests for the spec's gateway rows (`first` bounds, a missing
    ID, too many IDs, duplicates);
  - add `mappers_test.go` round-trips with `full(...)` for `Address`, `Party`
    and each reference message.

**2.8 e2e and docs** (`docs: addresses and parties APIs`):
- `test/e2e/smoke_test.go`: one read per new RPC, as the spec's e2e row lists
  them.
- `api/proto/public/README.md`: both services.
- `docs/architecture.md`: list `AddressesReadService` and `PartiesReadService`
  among the reader's mesh services. In "Read/write split", say that new mesh
  services use the public domain name, pending
  [`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md).
- `CLAUDE.md`, "Mesh naming": the same note.

**Checks:**
- `mise run check`, and `buf format --diff --exit-code` in `api/`.
- Generated-code freshness.
- `buf breaking` against the base passes without `!`, since everything is
  additive.
- `mise run test:integration` and `mise run test:e2e`.

## PR 3: extents and transfers by party (`feat/parcel-extents-transfers-by-party`)

**3.1 Mesh protos** (one commit with 3.2, `feat: serve parcel extents and transfers by party`,
because a new RPC on an existing service doesn't build until the reader
implements it):
- On `ParcelReadService`: `GetParcelExtentsWGS84ByParcelId`, `ExtentWGS84` and
  the request and response.
- On `RealPropertyTransferReadService`: `GetTransfersByPartyId` and its
  request and response.
- These existing services keep their singular names, as the spec says.
  `mise run generate`.

**3.2 Reader** (`feat: serve parcel extents and transfers by party`):
- **`GetParcelExtentsWGS84ByParcelId`** in `parcels.go`:
  - `public_id = ANY($ids)`, `NOT is_voided`, with geometry and attributes at
    `t` (rule 3);
  - `ST_XMin`, `ST_YMin`, `ST_XMax` and `ST_YMax` over `geom_web`;
  - a map keyed by `public_id`.
- **`GetTransfersByPartyId`** in `transfers.go`:
  - a new step-1 filter: `EXISTS` a party-parcel row for a requested,
    non-voided party on a non-voided parcel, `OR EXISTS` a party-improvement
    row for one on a visible improvement (rule 6);
  - otherwise the step-1 query, keyset, step 2 and assembly are reused
    unchanged;
  - `transferFilterHash` gets the RPC name `GetTransfersByPartyId`.
- **Tests:** the spec's "Reader integration: extents" and "...: transfers by
  party" rows. The extent containment check runs
  `ST_Contains(ST_MakeEnvelope(...), geom_web)` in SQL against the fixture
  database.

**3.3 Public protos, gateway, e2e and docs**, as in 2.6–2.8:
- the RPCs and `ExtentWGS84`, with gnostic descriptions copied from
  `GetEstimatedParcelsExtentWGS84Response`;
- the gateway mappers in `parcels.go` and `transfers.go`;
- handler and mapper tests;
- e2e reads of parcel 1's extent and party 1's transfers;
- the README entries for the two RPCs.

**Checks:** as PR 2.

## Final verification (before reporting the feature done)

- Check each intent success criterion against `main` after PR 3 merges, and
  list the evidence in PR 3's description.
- `openapi.yaml` has the new paths.
- `npm pack ./sdk/js` exports `AddressesService`, `PartiesService` and the new
  messages.

## Risks

| Risk | Mitigation |
|---|---|
| The F2 inner join on `party_attributes` drops a party with no attributes at `t` from `party_ids`, where today it's listed with no name. | That's rule 2, signed off in the spec. Check that no fixture party lacks current attributes before relying on the existing tests. |
| Fixture changes shift existing expectations, such as parcel 21 in the "all parcels" loops, or party 8's current row now starting in 2023. | PR 1 runs the full integration and e2e suites before any new RPC exists, so a shift shows up against unchanged code. |
| `TestPublicRPCs` compares mesh and public requests field by field. An unset optional `first` gets the default of 25 at the gateway. | Already solved for transfers. Follow what `transfers_test.go` does, and don't weaken the generic check. |
| `fillValid` fills a single `address_id` or `party_id` field. | It already rewrites string ID fields to UUIDs. Confirm it treats a singular `string` the same as a list item. |
| `google.type.LatLng` in the mesh and public protos. | Both already depend on googleapis for `Interval`. Check that the SDK and OpenAPI output render it, in the PR 2 generate step. |
| Comparables (F3) have joins beyond the four the spec names. | 1.4 asks for each join to be checked, and the findings to be recorded. |

## Out of scope for this plan (per spec)

Writing addresses, parties or geometry; full geometry; search
([`../parcel-search/intent.md`](../parcel-search/intent.md)); improvement
reads ([`../improvement-read-api/intent.md`](../improvement-read-api/intent.md));
by-feature extents; `system_as_of`; country and area lookups; backward
pagination and `totalCount`; the tile allowlist; and the mesh rename
([`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md)).
