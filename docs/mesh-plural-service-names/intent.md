# Intent: plural domain names for the mesh services

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Draft, written at the owner's request while specifying [`../address-party-read-api/spec.md`](../address-party-read-api/spec.md). Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The mesh services name their domain in the singular, `<Entity>ReadService` and
`<Entity>WriteService`, while most public services use the plural:

| Public service | Mesh service(s) |
|---|---|
| `ParcelsService` | `ParcelReadService`, `ParcelWriteService` |
| `ImprovementsService` | `ImprovementReadService` |
| `LandUsesService` | `LandUseReadService` |
| `RealPropertyTransfersService` | `RealPropertyTransferReadService` |
| `NeighborhoodService` | `NeighborhoodReadService` |
| `ValuationService` | `ValuationReadService` |
| `ZoningService` | `ZoningReadService` |

So the same domain has two names, depending on which side of the gateway
you're on, and moving between `services/gateway/*.go`, the reader and the
protos means translating `Parcels` to `Parcel` and back. Every new feature has
to make the choice again: the address and party read API adds
`AddressesService` and `PartiesService` on the public side, and would add
`AddressReadService` and `PartyReadService` on the mesh side under today's rule.

The public names aren't consistent either. Three are singular:
`NeighborhoodService`, `ValuationService` and `ZoningService`. "Zoning" is a
mass noun with no plural.

## Proposed outcome

1. **Every mesh service is named `<Domain>ReadService` or
   `<Domain>WriteService`, where `<Domain>` is the public service's domain
   name:** `ParcelsReadService`, `ParcelsWriteService`,
   `ImprovementsReadService`, `LandUsesReadService`,
   `RealPropertyTransfersReadService`, and so on. A reader can then go from a
   public service to its mesh service by swapping only the suffix.
2. **The rule is written down once**, in `CLAUDE.md` ("Mesh naming") and in
   `docs/architecture.md` ("Read/write split"), replacing `<Entity>`.
3. **Mesh packages don't change.** Only service names change. Packages already
   use the domain (`civil.mesh.parcels.v1`, `civil.mesh.landuses.v1`), so
   generated import paths stay the same.
4. **Everything that names a mesh service is updated in the same change:**
   - the gateway's clients;
   - the reader's and writer's handler registrations;
   - the gRPC health checker service names;
   - the tests, including the `ParcelWriteService` assertion in
     `services/gateway/handlers_test.go`;
   - the docs.

## Constraints

- **The mesh is internal, with no compatibility promise** (`api/proto/mesh`).
  `buf breaking` doesn't run on it, and all callers are in this repo, so the
  rename happens in one commit, as the monorepo migration's mesh renames did.
- **The gateway, reader and writer must be deployed together.** A Connect
  procedure path contains the service name. A gateway with the new names
  calling a reader with the old ones gets `Unimplemented` on every RPC until
  both are rolled out. The release notes must say so.
- **Health checks.** The reader and writer register their service names with
  the gRPC health checker. If a deployment probes a named service rather than
  the overall status, that probe changes too. The Helm chart isn't edited from
  here, so the owner checks the probes before rolling out.
- **No public API change in this feature by default.** Renaming a public
  service breaks generated clients. See the open question on public names.

## Success criteria

- Every mesh service name is its public service's domain name plus `Read` or
  `Write`. A grep for `ParcelReadService`, `ImprovementReadService` and the
  other old names outside `docs/` finds nothing.
- `mise run check`, the integration tests and the e2e tests pass. The e2e
  stack is the proof that the gateway and backends agree on the new paths.
- `CLAUDE.md` and `docs/architecture.md` state the new rule, and new
  features follow it.

## Open questions (carry into spec)

- **The three singular public services.** Should `NeighborhoodService` and
  `ValuationService` become `NeighborhoodsService` and `ValuationsService`?
  That's a breaking change (`!`, a minor bump before 1.0). The neighborhoods
  read API fixes are already breaking, so that rename could ride along there.
  If they stay singular, the mesh services follow them
  (`NeighborhoodReadService`, `ValuationReadService`) and keep their current
  names. `ZoningService` stays as it is either way, since "zoning" has no
  plural.
- **Order against the address and party read API.** *Decided 2026-11-16 by
  the owner:* that feature names its new mesh services
  `AddressesReadService` and `PartiesReadService` from the start, so the
  mesh is mixed until this rename lands. This rename doesn't need to come
  first.
- **Release.** The mesh isn't part of the public API, so a `refactor:` title
  doesn't trigger a release. But the deployment ordering above matters to
  operators. Should it be a `fix:` or `chore:` with a release note, or wait
  for the next release that has other changes?
