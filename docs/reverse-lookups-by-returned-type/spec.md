# Spec: put reverse lookups on the service of what they return

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-21 |
| **Status** | Draft. Message names, moving `GetImprovementsByAddressId` now, the PR split and the placement rule are proposals awaiting sign-off in review. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: where each RPC and message goes, on the
public API and the mesh, what the reader and gateway do, the placement rule,
the release and the tests. The order of the work goes in `plan.md`.

It builds on [`../pagination-without-edges/spec.md`](../pagination-without-edges/spec.md),
merged in #74. Its response shapes (results at field 1, `page_info` at 2)
are the ones moved here.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| The link messages | **Proposed: they keep their names and fields, and move to the returned type's package**: `civil.public.parcels.v1.AddressParcel`, `civil.public.parcels.v1.PartyParcel` and `civil.public.improvements.v1.AddressImprovement`. Each name says which lookup it comes from ("a parcel at an address", "a parcel a party holds"), and the package now says what it is. That's enough. New names (`ParcelAtAddress`, `ParcelHolding`) would be churn for no gain. Sharing one message, with `ownership_share` set only for party lookups, would make a field's presence depend on which RPC returned it, which the type can't show. The records app calls these RPCs through `Service.method` and never names the message types, so keeping the names costs it nothing either way. |
| `ImprovementsService`'s first record RPC | **Proposed: move it now**, with `AddressImprovement` as it is (an `improvement_id` and the `parcel_ids` it sits on). [`../improvement-read-api/intent.md`](../improvement-read-api/intent.md) already puts `GetImprovementsById` on `ImprovementsService`, so this is where the improvement reads are going anyway. `AddressImprovement` is a reference, not a full improvement, and doesn't constrain how that intent defines one. Waiting would leave one by-address lookup on `AddressesService` after the others have moved, or hold the 0.5.0 release for a feature that hasn't been specified. |
| One PR or two | **Two, back to back, before the release is cut.** `pagination-without-edges` merged as #74. This is a second `feat!:` PR. Release-please folds every unreleased commit into one version, so both land in 0.5.0 as long as the owner holds the release PR (#75) until this merges. |
| When "the returned type" isn't clear | **The rule: an RPC lives on the service of the records it returns. A link record belongs to the side the request doesn't name.** `GetParcelsByPartyId` names a party and returns parcels (with the party's share), so it's a parcel lookup. A later RPC that returned ownership rows for a parcel would name the parcel and belong to `PartiesService`. Lookups that return links from both sides, or records of more than one type, should be split, as the address reverse lookup was. |

## Further decisions

| Question | Decision |
|---|---|
| RPC and request names | **Unchanged.** `GetParcelsByAddressId` keeps its name on `ParcelsService`, and so on. The request and response messages keep their names and fields and move with the RPC. A client changes the service, and its imports if it names the messages. |
| Cursors | **An `end_cursor` from before the move still works after it.** The ID cursor's filter hash includes the RPC's short name (`"GetParcelsByAddressId"`), not the service, and neither changes. That isn't promised, but it means nothing fails mid-walk during the rollout, as in `pagination-without-edges`. |
| Behaviour | **Unchanged:** the same queries, visibility rules (`NOT is_voided` included), ordering, page sizes, validation and errors. No schema change, no migration. |
| Mesh services | **The existing service of the returned type**, as the intent says: `ParcelReadService` (which keeps its singular name until [`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md) renames it), `ImprovementReadService` and `PartiesReadService`. |

## Public API

| RPC | From | To |
|---|---|---|
| `GetParcelsByAddressId` | `civil.public.addresses.v1.AddressesService` | `civil.public.parcels.v1.ParcelsService` |
| `GetParcelsByPartyId` | `civil.public.parties.v1.PartiesService` | `civil.public.parcels.v1.ParcelsService` |
| `GetImprovementsByAddressId` | `civil.public.addresses.v1.AddressesService` | `civil.public.improvements.v1.ImprovementsService` |
| `GetPartiesByAddressId` | `civil.public.addresses.v1.AddressesService` | `civil.public.parties.v1.PartiesService` |

Messages that move with them, unchanged apart from their package:

| Package | Messages |
|---|---|
| `parcels.v1` | `GetParcelsByAddressIdRequest`, `GetParcelsByAddressIdResponse`, `AddressParcel`, `GetParcelsByPartyIdRequest`, `GetParcelsByPartyIdResponse`, `PartyParcel` |
| `improvements.v1` | `GetImprovementsByAddressIdRequest`, `GetImprovementsByAddressIdResponse`, `AddressImprovement` |
| `parties.v1` | `GetPartiesByAddressIdRequest`, `GetPartiesByAddressIdResponse` |

Field numbers, comments, validation and `idempotency_level = NO_SIDE_EFFECTS`
move as they are. Comments that point at another service are reworded where
the move makes them wrong. For example, `GetParcelsByAddressId`'s comment
refers to `ParcelsService.GetParcelsWithImprovementSummaryByParcelId`, which
is now on the same service.

Afterwards:

- `AddressesService` holds only `GetAddressesById`, and `addresses.v1` no
  longer imports `parties.v1`. `GetPartiesByAddressIdResponse` now uses
  `Party` from its own package.
- `PartiesService` holds `GetPartiesById` and `GetPartiesByAddressId`.
- `improvements.v1` gains imports of `buf/validate/validate.proto`,
  `civil/public/pagination/v1/pagination.proto` and
  `google/protobuf/timestamp.proto`.
- The service comment on `AddressesService` that describes "the lookups by
  address" moves to each RPC, since the lookups now live on three services.

## Mesh API

The same moves in `civil.mesh.addresses.v1`, `civil.mesh.parties.v1`,
`civil.mesh.parcels.v1` and `civil.mesh.improvements.v1`, to
`ParcelReadService`, `ImprovementReadService` and `PartiesReadService`, with
the same message names. The mesh promises no compatibility. A gateway and a
reader on different versions during a rollout get `Unimplemented` for the
moved RPCs, not wrong data, because the service is part of the procedure path.

## Reader

One `APIServer` already implements every mesh read service, so the handlers
stay where they are in `addresses.go` and `parties.go`. Only their request
and response types change package. They can move into `parcels`- and
`improvements`-named files if the plan finds that clearer, but that's not
required. `main.go` already registers all four mesh services, so nothing
changes there.

## Gateway

Each public handler moves to the server of its new service: `ParcelServer`
(`parcels.go`), `ImprovementServer` (`improvements.go`) and `PartyServer`
(`parties.go`). Each calls its own mesh client, which already exists on that
server. The mappers (`mapAddressParcel`, `mapPartyParcel`,
`mapAddressImprovement`, `mapParty`) change only their types. `pageSize`
stays shared.

## Documentation

- **`api/proto/public/README.md`:** the services table moves the lookups to
  their new rows. A new short section states the rule above, after the
  table: "An RPC lives on the service of the records it returns, whatever it
  looks them up by. A link record, such as a parcel with a party's share,
  belongs to the side the request doesn't name."
- **`CLAUDE.md`, Rules:** a bullet with the same rule, next to the mesh
  naming rule, linking this spec.
- **The migration note** goes in the PR description as a release-please
  commit override, as in `pagination-without-edges`:

  > `GetParcelsByAddressId` and `GetParcelsByPartyId` moved to
  > `ParcelsService`, `GetImprovementsByAddressId` to
  > `ImprovementsService`, and `GetPartiesByAddressId` to `PartiesService`,
  > with their request, response and result messages. Names and fields are
  > unchanged. Call them on the new service, and import the messages from its
  > package.

## Release

- **The PR title takes a `!`:** `feat!: move reverse lookups to the service
  of what they return`. `buf breaking` against the PR's base fails on the
  removed RPCs and moved messages, as intended.
- **It ships in 0.5.0 with `pagination-without-edges`.** The owner holds
  release PR #75 until this merges, then checks that the changelog carries
  both migration notes.
- **The records app moves both at once**, in its PR 2 from
  [`../pagination-without-edges/plan.md`](../pagination-without-edges/plan.md):
  - `src/api/addresses.ts` and `src/api/parties.ts` call the moved lookups
    on `ParcelsService`, `ImprovementsService` and `PartiesService`;
  - its test fixtures (`src/test/fixtures.ts`) register those methods on
    the new services' routers.

  The same deployment-order rule applies: no deployment takes civil-os 0.5.x
  before that records app release.

## Tests

- **Integration tests** call the moved RPCs through the mesh client of
  their new service. `readerClients` already has `parcels`, `improvements`
  and `parties` clients. Their expectations don't change, which is the
  check that behaviour didn't change.
- **Gateway tests:** `publicServices` in `handlers_test.go` covers the
  moved methods through their new services without change. The page-size
  default test in `address_party_test.go` iterates over
  `AddressesServiceName` and `PartiesServiceName`. It now iterates over the
  services that hold the paginated lookups: `ParcelsService`,
  `ImprovementsService` and `PartiesService`. The `checkFullMapping` tests
  just change types.
- **e2e:** the address and party checks in `test/e2e/smoke_test.go` call
  the new services.
- **A check of the success criterion:** a gateway unit test reads the
  `AddressesService` descriptor and fails unless `GetAddressesById` is its
  only method. A generic check over every service, matching method names to
  result packages, would need exceptions such as
  `ParcelsService.GetParcelIdsByFeatureId`, which returns IDs, not records.
  The README and `CLAUDE.md` rule covers new RPCs instead.
