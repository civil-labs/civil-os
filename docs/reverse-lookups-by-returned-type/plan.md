# Plan: put reverse lookups on the service of what they return

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `59afa95`, #76), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-21 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The civil-os work lands in one PR, on its own
  short-lived branch. The agent never pushes to `main`. The owner merges.
- **Each step is one commit** with a conventional-commit message, and every
  commit passes `mise run check`. The PR is squash-merged, so the commits
  only matter for review.
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same PR. If the departure changes the spec, the
  agent stops and asks the owner first.
- **Verification before "done".** The PR's checks are run locally, and their
  output goes in the PR description before the PR is reported as done.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated `api/gen/go`, `internal/gen/go`
  and `sdk/js/gen` are committed with the `.proto` change.
- **No behaviour change, no fixtures, no schema.** Every test expectation
  stays as it is. Only the service a test calls changes.

## Environment

Docker is reachable from the session (server 29.5.3 on 2026-11-21), so
`mise run test:integration` and `mise run test:e2e` run locally. `go` and
`buf` aren't on `PATH`. Run them as `mise exec -- go …` and
`mise exec -- buf …`, as for #74.

## PR overview

| PR | Repo | Branch | Title | Depends on | Release effect |
|---|---|---|---|---|---|
| 1 | civil-os | `feat/reverse-lookups-by-returned-type` | `feat!: move reverse lookups to the service of what they return` | #74 (merged) | public-api 0.5.0 with #74; Go module minor |
| 2 | opencama-record-keeper | `feat/civil-os-api-0.5` | (planned there) | civil-os 0.5.0 on npm | records app minor |

PR 2 is the same records app PR as in
[`../pagination-without-edges/plan.md`](../pagination-without-edges/plan.md).
It takes both changes at once.

**Release PR #75 stays open** until PR 1 merges. Release-please then
regenerates it with both breaking changes in 0.5.0.

## PR 1: civil-os (`feat/reverse-lookups-by-returned-type`)

### 1.1 Move the RPCs (`feat!: move reverse lookups to the service of what they return`)

One commit, because the generated code, the reader, the gateway and the
tests don't compile apart.

**Public protos** (`api/proto/public/civil/public/`). Each RPC moves with
its comment, options and request and response messages. Each result
message keeps its name, fields and comments.

- From `addresses/v1/addresses.proto`:
  - `GetParcelsByAddressId`, its request and response, and `AddressParcel`
    → `parcels/v1/parcels.proto`, in `ParcelsService`;
  - `GetImprovementsByAddressId`, its request and response, and
    `AddressImprovement` → `improvements/v1/improvements.proto`, in
    `ImprovementsService`;
  - `GetPartiesByAddressId` and its request and response →
    `parties/v1/parties.proto`, in `PartiesService`. Its response's
    `civil.public.parties.v1.Party` becomes `Party`.
- From `parties/v1/parties.proto`: `GetParcelsByPartyId`, its request and
  response, and `PartyParcel` → `parcels/v1/parcels.proto`, in
  `ParcelsService`.
- New RPCs go at the end of each service, after the existing RPCs, and new
  messages go after that file's existing messages.
- `addresses.proto` drops its `parties/v1` and `pagination/v1` imports, and
  any others it no longer uses. `buf lint` reports unused imports.
  `improvements.proto` gains `buf/validate/validate.proto`,
  `civil/public/pagination/v1/pagination.proto` and
  `google/protobuf/timestamp.proto`.
- **Comments:**
  - `AddressesService`'s comment stops describing "the lookups by address".
    The order and "nothing for an unknown or voided address" go on each moved
    RPC's comment.
  - `GetParcelsByAddressId`'s pointer to
    `ParcelsService.GetParcelsWithImprovementSummaryByParcelId` becomes
    `GetParcelsWithImprovementSummaryByParcelId`.
  - Any `AddressesService.` or `PartiesService.` prefix that's now wrong is
    fixed. `grep -n 'AddressesService\.\|PartiesService\.' api/proto/public`
    lists them.

**Mesh protos** (`api/proto/mesh/civil/mesh/`): the same moves, from
`addresses/v1` and `parties/v1` to `parcels/v1` (`ParcelReadService`),
`improvements/v1` (`ImprovementReadService`) and `parties/v1`
(`PartiesReadService`), with the same names. Import changes follow the same
pattern.

Then `mise run generate`, `buf format --diff --exit-code` and `buf lint` in
`api/`.

**Reader** (`services/database-reader/`): the handlers stay in
`addresses.go` and `parties.go`, next to `visibleAddressCTE`,
`visibleParcelJoinsSQL` and `scanParties`, which they share. Only their
request and response types change package (`parcelsv1.`,
`improvementsv1.`, `partiesv1.`), and the `pageByID` calls keep their RPC
names, so the filter hashes don't change. `main.go` doesn't change: the same
`srv` already backs `ParcelReadService`, `ImprovementReadService` and
`PartiesReadService`.

**Gateway** (`services/gateway/`):

- `GetParcelsByAddressId` and `GetParcelsByPartyId` move to `ParcelServer`
  in `parcels.go`, with `mapAddressParcel` and `mapPartyParcel`, and call
  `s.dbReaderClient` (`ParcelReadServiceClient`).
- `GetImprovementsByAddressId` moves to `ImprovementServer` in
  `improvements.go`, with `mapAddressImprovement`.
- `GetPartiesByAddressId` moves to `PartyServer` in `parties.go`.
- `AddressServer` keeps only `GetAddressesById`, and `PartyServer` keeps
  `GetPartiesById` and gains `GetPartiesByAddressId`. `main.go` doesn't
  change: each server already has its mesh client.

**Tests:**

- `services/database-reader/address_party_integration_test.go`: the
  helpers (`parcelsAtAddress`, `improvementsAtAddress`, `partiesAtAddress`,
  `partyParcels`), `TestAddressPartyPagination` and
  `TestAddressLookupPagination` call `r.parcels`, `r.improvements` and
  `r.parties` with the moved request types. Expectations don't change.
- `services/database-reader/id_cursor_test.go` doesn't change. It passes RPC
  names as strings, and those names don't change.
- `services/gateway/address_party_test.go`:
  - `TestMapAddressPartyResults` takes the mesh types from their new
    packages;
  - `TestAddressPartyPageSizeDefault` iterates over `ParcelsServiceName`,
    `ImprovementsServiceName` and `PartiesServiceName`. On those services,
    `SearchParcelsAddresses` also has a `first` field, so the test would
    find five paginated RPCs, not four. It checks that the set it tested is
    exactly the four moved lookups plus `SearchParcelsAddresses`, by name,
    instead of counting.
- `test/e2e/smoke_test.go`: the `GetParcelsByAddressId` and
  `GetParcelsByPartyId` calls use the parcels client. If the smoke test
  doesn't have one, it builds one the way it builds the others.

**Check:** `mise run check`, `mise run test:integration`.

### 1.2 Check where the lookups live (`test: check where the reverse lookups are served`)

Two assertions in `services/gateway/address_party_test.go`, as the spec
asks:

- **`AddressesService` has one method:** read its descriptor, and fail unless
  its methods are exactly `[GetAddressesById]`.
- **Each moved RPC reaches the right mesh service:** for each of the four,
  call it through the gateway against the fake mesh, and check that the
  recorded procedure is
  `/civil.mesh.parcels.v1.ParcelReadService/GetParcelsByAddressId` and so
  on. The generic routing test in `handlers_test.go` only checks the method
  name suffix, so it wouldn't notice a lookup sent to the wrong mesh
  service.

**Check:** `mise exec -- go test ./services/gateway/`.

### 1.3 Write down the rule (`docs: say where reverse lookups live`)

- `api/proto/public/README.md`:
  - in the services table, `ParcelsService`'s row gains "parcels by
    address or party". `ImprovementsService`'s row gains "improvements at an
    address (paginated)". `AddressesService` becomes "Addresses by ID", and
    `PartiesService` "Parties by ID or mailing address (paginated)";
  - a short section after the table, **Where an RPC lives**, gives the
    spec's rule word for word;
  - the Pagination section's first paragraph names the paginated lookups by
    their new services.
- `CLAUDE.md`, Rules: a bullet after **Mesh naming**:

  > **RPC placement:** an RPC lives on the service of the records it
  > returns, whatever it looks them up by. A link record belongs to the side
  > the request doesn't name. See
  > `docs/reverse-lookups-by-returned-type/spec.md`.

### 1.4 Verify and open the PR

Run, and paste the output into the PR description:

```sh
mise run check
mise run test:integration
mise run test:e2e
(cd api && mise exec -- buf format --diff --exit-code)
mise run generate && git diff --exit-code api/gen/go internal/gen/go sdk/js/gen
(cd api && mise exec -- buf breaking proto/public --against "../.git#ref=origin/main,subdir=api/proto/public")
grep -rn 'AddressesService\.\(GetParcels\|GetImprovements\|GetParties\)\|PartiesService\.GetParcels' api/proto services test docs/architecture.md   # expect no output
```

`buf breaking` is expected to fail. It should list only the four RPCs
removed from `AddressesService` and `PartiesService`, and the eleven
messages deleted from their old packages: eight from `addresses.v1` (three
requests, three responses, `AddressParcel` and `AddressImprovement`) and
three from `parties.v1` (`GetParcelsByPartyId`'s request and response, and
`PartyParcel`). The additions to the new packages aren't breaking.

The PR description ends with the spec's migration note as a commit
override:

```
BEGIN_COMMIT_OVERRIDE
feat!: move reverse lookups to the service of what they return

BREAKING CHANGE: `GetParcelsByAddressId` and `GetParcelsByPartyId` moved to
`ParcelsService`, `GetImprovementsByAddressId` to `ImprovementsService`, and
`GetPartiesByAddressId` to `PartiesService`, with their request, response
and result messages. Names and fields are unchanged. Call them on the new
service, and import the messages from its package.
END_COMMIT_OVERRIDE
```

### After the merge

- **Release PR #75** regenerates. The owner checks that its
  `api/proto/public/CHANGELOG.md` entry for 0.5.0 carries both migration
  notes, then merges it, which publishes `@civil-labs/civil-os-api` 0.5.0.
- **The records app PR** follows in its own repo. Beyond the pagination
  change, it:
  - calls `getParcelsByAddressId` and `getParcelsByPartyId` on
    `ParcelsService`, `getImprovementsByAddressId` on `ImprovementsService`,
    and `getPartiesByAddressId` on `PartiesService` (`src/api/addresses.ts`,
    `src/api/parties.ts`);
  - registers those methods on the new services in `src/test/fixtures.ts`.

**Deployment order:** unchanged from `pagination-without-edges`. No
deployment takes civil-os 0.5.x before that records app release.
