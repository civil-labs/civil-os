# Plan: plural domain names for the mesh services

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `6d3d399`, #79), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-22 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** One PR, on its own short-lived branch. The agent never
  pushes to `main`. The owner merges.
- **Each step is one commit** with a conventional-commit message, and every
  commit passes `mise run check`. The PR is squash-merged, so the commits
  only matter for review.
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same PR. If the departure changes the spec, the
  agent stops and asks the owner first.
- **Verification before "done".** The PR's checks are run locally, and their
  output goes in the PR description before the PR is reported as done.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated code is committed with them.
- **No behaviour change.** No test expectation changes, only names.

## Environment

Docker is reachable from the session, so `mise run test:integration` and
`mise run test:e2e` run locally. `go` and `buf` aren't on `PATH`. Run them as
`mise exec -- go …` and `mise exec -- buf …`.

## PR overview

| PR | Repo | Branch | Title | Release effect |
|---|---|---|---|---|
| 1 | civil-os | `feat/plural-service-names` | `feat!: name services by their plural domain` | public-api 0.5.0 (held, with #74 and #78); Go module minor |

The records app's 0.5.0 PR, planned in its repo, also renames its
`NeighborhoodService` import (`src/api/lookups.ts`). Release PR #75 stays
held.

## PR 1: civil-os (`feat/plural-service-names`)

### 1.1 Rename the services (`feat!: name services by their plural domain`)

One commit, because generated code and its callers don't compile apart.

**Protos:**

| File | From | To |
|---|---|---|
| `mesh/civil/mesh/parcels/v1/parcels.proto` | `ParcelReadService` | `ParcelsReadService` |
| | `ParcelWriteService` | `ParcelsWriteService` |
| `mesh/civil/mesh/improvements/v1/improvements.proto` | `ImprovementReadService` | `ImprovementsReadService` |
| `mesh/civil/mesh/landuses/v1/land_uses.proto` | `LandUseReadService` | `LandUsesReadService` |
| `mesh/civil/mesh/transfers/v1/transfers.proto` | `RealPropertyTransferReadService` | `RealPropertyTransfersReadService` |
| `mesh/civil/mesh/neighborhoods/v1/neighborhoods.proto` | `NeighborhoodReadService` | `NeighborhoodsReadService` |
| `mesh/civil/mesh/valuations/v1/valuations.proto` | `ValuationReadService` | `ValuationsReadService` |
| `public/civil/public/neighborhoods/v1/neighborhoods.proto` | `NeighborhoodService` | `NeighborhoodsService` |
| `public/civil/public/valuations/v1/valuations.proto` | `ValuationService` | `ValuationsService` |

The two OpenAPI field descriptions in `public/.../parcels/v1/parcels.proto`
that say `NeighborhoodService.GetNeighborhoodDefinitions` become
`NeighborhoodsService.GetNeighborhoodDefinitions`. Any other comment in
`api/proto` that names an old service is updated too.

Then `mise run generate`, `buf format --diff --exit-code` and `buf lint`.

**Code:** every identifier generated from those names changes with them:
the `…ServiceName` constants, `New…Client`, `New…Handler`, and the
`…Client` and `…Handler` types. The files, from the spec and a grep on
`main`:

- `services/gateway/`: `main.go`, `parcels.go`, `improvements.go`,
  `land_uses.go`, `transfers.go`, `neighborhoods.go`, `valuations.go`;
- `services/database-reader/main.go` (registrations and health checker);
- `services/database-writer/main.go` (registration and health checker), and
  `parcels.go`'s doc comment;
- tests:
  - `services/database-reader/integration_test.go` and `search_perf_test.go`;
  - `services/database-writer/integration_test.go`;
  - `services/gateway/handlers_test.go` (`publicServices` and the
    `ParcelWriteService` assertion);
  - `services/gateway/address_party_test.go` (the procedure names in
    `TestReverseLookupPlacement`);
  - `test/e2e/smoke_test.go` (the neighborhoods client).

The edit is a **substring** replacement of the nine old names across
`services/` and `test/`, followed by `gofmt` and a build. It has to be a
substring match, not a whole-word one: the generated identifiers embed the
names (`NewNeighborhoodServiceClient`, `ParcelReadServiceName`,
`UnimplementedValuationServiceHandler`), and a whole-word match would miss
them. A substring match is safe because no new name contains an old one:
each new name inserts an `s` inside the old name (`Parcel|s|ReadService`).

**Check:** `mise run check`, `mise run test:integration`.

### 1.2 The naming test (`test: check mesh service names follow the public ones`)

`TestMeshServiceNames` in `services/gateway/handlers_test.go`, next to
`publicServices`:

- range over `protoregistry.GlobalFiles`, as `newFakeMesh` does, and collect
  every service in a `civil.mesh.*` package and in a `civil.public.*`
  package;
- for each mesh service, it fails unless its name ends in `ReadService` or
  `WriteService`, and unless a public service named `<Domain>Service`
  exists, where `<Domain>` is the name without that suffix. For example,
  `ParcelsWriteService` needs `ParcelsService`;
- it also fails if no mesh services were found, so a broken registry walk
  can't pass silently.

The gateway imports every mesh and public package, so the registry holds
all of them.

**Check:** `mise exec -- go test -run TestMeshServiceNames -v ./services/gateway/`.
Before committing, rename one mesh service locally to confirm that the test
fails, then revert.

### 1.3 Docs (`docs: state the mesh naming rule`)

- `CLAUDE.md`: the **Mesh naming** bullet becomes the spec's text.
- `docs/architecture.md`, **Read/write split**: the same rule,
  `ParcelsWriteService`, and the sentence about singular names being due for
  renaming is removed.
- `api/proto/public/README.md`: the services table's `NeighborhoodsService`
  and `ValuationsService` rows. Any other mention of the old public names in
  it is updated.

### 1.4 Verify and open the PR

Run, and paste the output into the PR description:

```sh
mise run check
mise run test:integration
mise run test:e2e
(cd api && mise exec -- buf format --diff --exit-code)
mise run generate && git diff --exit-code api/gen/go internal/gen/go sdk/js/gen
(cd api && mise exec -- buf breaking proto/public --against "../.git#ref=origin/main,subdir=api/proto/public")
grep -rnE 'ParcelReadService|ParcelWriteService|ImprovementReadService|LandUseReadService|RealPropertyTransferReadService|NeighborhoodReadService|ValuationReadService|NeighborhoodService|ValuationService' \
  --include='*.go' --include='*.proto' --include='*.ts' --include='*.yaml' --include='*.md' . \
  | grep -vE '^\./docs/[a-z0-9-]+/(intent|spec|plan)\.md|node_modules'   # expect no output
```

The grep matches substrings, so it also catches generated identifiers
such as `NewNeighborhoodServiceClient`. It can't match a new name, because
no new name contains an old one. Generated code is included, so stale
generated names would show up.

`buf breaking` is expected to report only `NeighborhoodService` and
`ValuationService` as deleted from their files. It doesn't run on the mesh.

The PR description ends with the spec's migration note as a commit
override:

```
BEGIN_COMMIT_OVERRIDE
feat!: name services by their plural domain

BREAKING CHANGE: `NeighborhoodService` is now `NeighborhoodsService`, and
`ValuationService` is now `ValuationsService`, in the same packages with the
same RPCs and messages. Generated clients change name. The mesh services
between the gateway and its backends are renamed too: deploy the gateway,
database reader and database writer of this release together. A mixed
rollout fails every mesh call with `Unimplemented` until all three are
updated.
END_COMMIT_OVERRIDE
```

### After the merge

- **Release PR #75** regenerates with a third breaking change. Before
  merging, the owner checks that its 0.5.0 changelog has all three migration
  notes.
- **Before rolling out 0.5.x,** the owner confirms that no deployment probes
  a mesh service by name (the spec found none in the chart or gitops repo),
  and deploys the gateway, reader and writer together.
- **The records app's 0.5.0 PR** adds the `NeighborhoodsService` import
  rename to its other changes.
