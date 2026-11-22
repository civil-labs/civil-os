# Intent: put reverse lookups on the service of what they return

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-21 |
| **Status** | Draft, spawned by [`../pagination-without-edges/spec.md`](../pagination-without-edges/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The public API mostly files an RPC under the service of the records it
returns. `RealPropertyTransfersService` holds `GetTransfersByParcelId`,
`GetTransfersByFeatureId` and `GetTransfersByPartyId`, and the parcel search
spec put `SearchParcelsAddresses` on `ParcelsService` "because it returns
parcels". Four reverse lookups don't follow that rule:

| RPC today | Returns |
|---|---|
| `AddressesService.GetParcelsByAddressId` | parcels |
| `AddressesService.GetImprovementsByAddressId` | improvements |
| `AddressesService.GetPartiesByAddressId` | parties |
| `PartiesService.GetParcelsByPartyId` | parcels |

The address and party intent
([`../address-party-read-api/intent.md`](../address-party-read-api/intent.md))
proposed one reverse lookup on `AddressesService` that returned all three
kinds. Its spec split that into three RPCs, because one cursor over mixed
kinds is awkward, but never revisited where the three should live. So they
stayed where the combined one would have been, and `GetParcelsByPartyId`
followed them.

The cost is mostly in finding things. A client looking for "parcels that…"
has to know to check `AddressesService` and `PartiesService` as well as
`ParcelsService`. Every new reverse lookup also has to choose between the
two patterns.

These four RPCs already break in
[`../pagination-without-edges/spec.md`](../pagination-without-edges/spec.md),
and the records app, their only known client, already changes the same call
sites for it. Moving them in the same release means one migration instead
of two.

## Proposed outcome

1. **Each lookup moves to the service of what it returns:**

   | Today | Moves to |
   |---|---|
   | `AddressesService.GetParcelsByAddressId` | `ParcelsService.GetParcelsByAddressId` |
   | `AddressesService.GetImprovementsByAddressId` | `ImprovementsService.GetImprovementsByAddressId` |
   | `AddressesService.GetPartiesByAddressId` | `PartiesService.GetPartiesByAddressId` |
   | `PartiesService.GetParcelsByPartyId` | `ParcelsService.GetParcelsByPartyId` |

   On the mesh they move to `ParcelReadService`, `ImprovementReadService`
   and `PartiesReadService`, the existing services for those records.
2. **Their request and response messages move with them** into the
   returned type's package. Behaviour doesn't change: the same filters,
   visibility rules, ordering, cursors, page sizes and errors as after
   `pagination-without-edges`.
3. **The rule is written down** in the public README and in `CLAUDE.md`'s
   rules: an RPC lives on the service of the records it returns, whatever it
   looks them up by.
4. **It ships in the same public API release as `pagination-without-edges`**,
   so the records app migrates once.

## Constraints

- **This breaks the public API:** RPCs and messages are removed from one
  service and package and added to another. The PR title takes a `!`. At 0.x
  this is a minor version, the same one as `pagination-without-edges`.
- **No old RPCs left behind.** The old names are removed in the same
  release, not kept as aliases, so the API never offers both. The records app
  is the only client and moves in step.
- **No behaviour change** and no schema change.
- **No deployment-repo changes** from here. The records app is updated in
  its own repo.

## Success criteria

- Every public RPC that returns records of one type is on that type's
  service.
- `AddressesService` holds only `GetAddressesById`, and `PartiesService`
  holds `GetPartiesById` and `GetPartiesByAddressId`.
- The moved RPCs' integration and e2e tests pass unchanged apart from the
  service they call.

## Open questions (carry into spec)

- **The link messages.** `AddressParcel` (parcel ID and feature ID) and
  `PartyParcel` (parcel ID, feature ID and the party's `ownership_share`)
  describe a link, not a parcel. On `ParcelsService` they'd need new names,
  such as `ParcelAtAddress` and `ParcelHolding`. Do they keep one message
  each, or share one, with `ownership_share` set only where there's a party?
  `AddressImprovement` raises the same question on `ImprovementsService`.
- **`ImprovementsService` gains its first record RPC.** Today it only serves
  lookup tables (types and conditions). Does `GetImprovementsByAddressId`
  wait for [`../improvement-read-api/intent.md`](../improvement-read-api/intent.md)
  to define how an improvement reads, or move now with its current
  reference message?
- **One PR or two.** These moves and `pagination-without-edges` touch the
  same protos, handlers and tests. Should they be one implementation PR, or
  two merged back to back before the release is cut?
- **Is "the returned type" always clear?** If a later RPC returns a link
  record that belongs equally to both sides, which service holds it? The
  rule in the README should cover that case or name a tiebreak.
