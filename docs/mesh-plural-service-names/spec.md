# Spec: plural domain names for the mesh services

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-22 |
| **Status** | Draft. Renaming the two singular public services and shipping in 0.5.0 are proposals awaiting sign-off in review. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the names, what refers to them, the health checks, the
docs, the release and the tests. The order of the work goes in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| The singular public services | **Proposed: rename `NeighborhoodService` to `NeighborhoodsService` and `ValuationService` to `ValuationsService`**, in the 0.5.0 release the owner is holding. That release already breaks the public API twice ([`../pagination-without-edges/spec.md`](../pagination-without-edges/spec.md), [`../reverse-lookups-by-returned-type/spec.md`](../reverse-lookups-by-returned-type/spec.md)), and the records app migrates to it in one PR. Its change for this rename is one import in `src/api/lookups.ts` (it uses `NeighborhoodService` and not `ValuationService`). Renaming later would cost a third migration. Their packages, `civil.public.neighborhoods.v1` and `civil.public.valuations.v1`, are already plural and don't change, so only the service name in the procedure path and in generated code changes. `ZoningService` stays: "zoning" has no plural. |
| Order against the address and party read API | Decided in the intent: `AddressesReadService` and `PartiesReadService` were named plural from the start. They don't change. |
| Release | **Proposed: in the held 0.5.0**, as a `feat!:` PR, because it renames two public services. Its migration note says to deploy the gateway, reader and writer together. 0.5.0 already requires that, because #74 changed mesh response fields, so the note says it once for the whole release. If the owner keeps the public names, the PR becomes `refactor:` and changes only the mesh. A `refactor:` commit's body doesn't reach the changelog, so the deploy-together note would then have to go in release PR #75's description by hand. |

## Further decisions

| Question | Decision |
|---|---|
| The rule | **A mesh service is named `<Domain>ReadService` or `<Domain>WriteService`, where `<Domain>` is the public service's name without `Service`.** `ParcelsService` → `ParcelsReadService` and `ParcelsWriteService`. `ZoningService` → `ZoningReadService`. A public service with no mesh counterpart (`InstanceService`, `DexService`) has none. A new public service gets a plural domain name unless the noun has no plural. |
| Packages and messages | **Unchanged.** Mesh packages are already named by domain (`civil.mesh.parcels.v1`), so generated import paths don't change. Only the service declarations and the code that names them change. |
| Health checks | **The reader and writer register the new names**, through the generated `…ServiceName` constants, so no strings change by hand. Nothing probes a named service today: `compose.yaml` checks overall status (`{}`), and the `civil-os` Helm chart's reader and writer define no probes (checked read-only on 2026-11-22). The gitops repo doesn't name a mesh service either. The owner still confirms this before the rollout, since the deployment repos aren't changed from here. The gateway's checker uses the public `ParcelsServiceName`, which doesn't change. |
| Reader and gateway structs | **Unchanged.** The reader's `APIServer` and the gateway's `ParcelServer`, `LandUseServer` and so on aren't named after mesh services. Renaming the gateway's singular struct names is a separate tidy-up, not part of this. |

## Names

| Public service | Mesh service today | Mesh service after |
|---|---|---|
| `ParcelsService` | `ParcelReadService` | `ParcelsReadService` |
| `ParcelsService` | `ParcelWriteService` | `ParcelsWriteService` |
| `ImprovementsService` | `ImprovementReadService` | `ImprovementsReadService` |
| `LandUsesService` | `LandUseReadService` | `LandUsesReadService` |
| `RealPropertyTransfersService` | `RealPropertyTransferReadService` | `RealPropertyTransfersReadService` |
| `NeighborhoodService` → **`NeighborhoodsService`** | `NeighborhoodReadService` | `NeighborhoodsReadService` |
| `ValuationService` → **`ValuationsService`** | `ValuationReadService` | `ValuationsReadService` |
| `ZoningService` | `ZoningReadService` | unchanged |
| `AddressesService` | `AddressesReadService` | unchanged |
| `PartiesService` | `PartiesReadService` | unchanged |

If the owner keeps the two singular public names, their mesh services keep
their names too (`NeighborhoodReadService`, `ValuationReadService`), which
matches the rule.

## What changes

- **Protos:** the seven mesh `service` declarations, and the two public ones.
  Comments that name a renamed service are updated, including the two
  OpenAPI field descriptions in `parcels.proto` that point at
  `NeighborhoodService.GetNeighborhoodDefinitions`. Then
  `mise run generate`.
- **Gateway** (`services/gateway/`): `main.go`'s mesh clients and public
  handlers, and the client types on each server struct (`parcels.go`,
  `improvements.go`, `land_uses.go`, `transfers.go`, `neighborhoods.go`,
  `valuations.go`).
- **Reader** (`services/database-reader/main.go`): the handler registrations
  and the health checker.
- **Writer** (`services/database-writer/main.go`, `parcels.go`): the handler
  registration, the health checker, and the doc comment on `ParcelServer`.
- **Tests:**
  - the reader's `readerClients` (`integration_test.go`, `search_perf_test.go`);
  - the writer's `integration_test.go`;
  - in the gateway: `handlers_test.go`'s `ParcelWriteService` assertion and
    `publicServices` list, and `address_party_test.go`'s mesh procedure
    names in `TestReverseLookupPlacement`;
  - `test/e2e/smoke_test.go`'s neighborhoods client.
- **Docs:** see below.

`buf breaking` runs only on the public module. It reports the two renamed
public services, as removed and not as renamed, and nothing else.

## Documentation

- **`CLAUDE.md`, Mesh naming:** replaced with the rule above, without the
  "older singular names" caveat:

  > **Mesh naming:** a mesh service is the public service's domain plus
  > `ReadService` (served by the reader) or `WriteService` (served by the
  > writer): `ParcelsService` → `ParcelsReadService`, `ParcelsWriteService`.
  > New public services take a plural domain name unless the noun has none
  > (`ZoningService`).

- **`docs/architecture.md`, Read/write split:** the same rule, the
  `ParcelsWriteService` name, and no pointer to the intent as pending work.
- **`api/proto/public/README.md`:** the services table's `NeighborhoodsService`
  and `ValuationsService` rows.

## Release

- **Title:** `feat!: name services by their plural domain`.
- **Migration note**, as a release-please commit override in the PR
  description:

  > `NeighborhoodService` is now `NeighborhoodsService`, and
  > `ValuationService` is now `ValuationsService`, in the same packages with
  > the same RPCs and messages. Generated clients change name. The mesh
  > services between the gateway and its backends are renamed too: deploy
  > the gateway, database reader and database writer of this release
  > together. A mixed rollout fails every mesh call with `Unimplemented`
  > until all three are updated.

- **Version:** with 0.5.0 still held, release PR #75 folds this in. The
  records app's 0.5.0 PR also renames its `NeighborhoodService` import.

## Tests

The rename changes no behaviour, so no test expectation changes.

- `mise run check`, `mise run test:integration` and `mise run test:e2e`
  pass. The e2e stack runs the gateway, reader and writer from this commit,
  which shows they agree on the new paths.
- **The success criterion** is checked by a grep in the PR's verification:
  none of the old mesh names (`ParcelReadService`, `ParcelWriteService`,
  `ImprovementReadService`, `LandUseReadService`,
  `RealPropertyTransferReadService`, `NeighborhoodReadService`,
  `ValuationReadService`), nor `NeighborhoodService` or `ValuationService` as
  whole words, appear outside `docs/` and the regenerated code's history.
  Design docs keep the names they were written with.
- **A naming test** in the gateway: for every public service that has mesh
  services, the mesh service names are the public name with `Service`
  replaced by `ReadService` or `WriteService`. It walks the registered mesh
  descriptors and, for each, checks that a public service with the matching
  domain exists. That keeps the rule enforced for new services without a
  hand-kept list.
