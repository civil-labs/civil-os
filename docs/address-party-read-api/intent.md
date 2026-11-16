# Intent: public read API for addresses and parties, and parcel extents

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The public API returns references to addresses and parties, but offers no way to
read them:

| Reference | Where it appears | What a client can't do |
|---|---|---|
| `address_id`, `formatted_address` | `ParcelDetails` | Read the address's parts (lines, locality, postal code, coordinates), or find the other parcels, improvements and parties that use it |
| `party_ids` (max 10), `primary_owner_name`, `primary_owner_address` | `ParcelDetails` | Read a party's name and mailing address by ID, see each party's ownership share, or list the parcels a party holds |
| `party_id`, `name`, `mailing_address` | `RealPropertyTransfer.parties` | Go from a grantor or grantee to the party's current record and holdings |

The database already models all of this:

- `core.addresses` + `address_attributes`, which hold the address parts, `formatted_address`,
  and `coordinates` as a 4326 point;
- `core.parties` + `party_attributes`, which hold `name` and the mailing `address_id`;
- `parcel_parties`, which holds `ownership_share` over a `legal_valid_range`;
- `address_id` on `parcel_attributes` and `improvement_attributes`.

There's a related gap for parcels themselves: **the public API returns no location for
a parcel.** There's no geometry, extent or centroid. A client can learn where a parcel
is only from the vector tiles (`get_parcel_tiles`) it has already loaded. A map client
given a `parcel_id` from a link, a transfer or a reload therefore can't find or center
that parcel unless its tile happens to be loaded. The geometry is in
`geo.parcel_geometry` (`geom_web`, EPSG:4326, versioned by `legal_valid_range`), and
`GetEstimatedParcelsExtentWGS84` already returns an extent for the whole jurisdiction.

The first client that needs these reads is a planned frontend app for browsing records.
In that app, a user moves from a parcel to its owner, to the owner's other parcels, and
to their mailing address. The parcel is shown on a map beside its details. With the
current API, the address and the party are dead ends, and a parcel opened by link
can't be found on the map.

## Proposed outcome

1. **A new public `AddressesService`**, served by the reader through a new mesh
   `AddressReadService`:
   - a lookup by address ID, which returns the `address_attributes` fields and the
     coordinates as longitude/latitude;
   - a reverse lookup, which returns the parcels, improvements and parties that
     reference an address.
2. **A new public `PartiesService`**, served by the reader through a new mesh
   `PartyReadService`:
   - a lookup by party ID, which returns the name and the mailing address (ID plus
     formatted address);
   - a lookup of the parcels a party holds through `parcel_parties`, each with its
     `ownership_share`, paginated the same way as transfers
     (`civil.public.pagination.v1`).
3. **A parcel extent lookup on `ParcelsService`:** `GetParcelExtentsWGS84ByParcelId`.
   It takes a list of parcel IDs and returns, per parcel, the WGS84 bounding box of its
   current legal geometry (`min_x, min_y, max_x, max_y`, as in
   `GetEstimatedParcelsExtentWGS84Response`), computed from `geom_web`. Voided parcels,
   and parcels with no geometry at the legal time, are left out of the response map. It
   reads `parcel_geometry`, so it's served by the reader through the existing mesh
   `ParcelReadService`.
4. **All of these follow the existing patterns:**
   - UUID public IDs;
   - protovalidate rules;
   - voided addresses, parties and parcels hidden;
   - attributes resolved at the current legal time by default, with an optional
     `legal_as_of`, as `GetParcelsWithImprovementSummaryByParcelId` already has.
5. **The synthetic fixtures cover:**
   - a party that holds several parcels with different shares;
   - a parcel with several parties;
   - an address shared by a parcel and a party's mailing address;
   - a voided party;
   - a parcel whose geometry was redrawn, so it has an earlier and a current
     geometry version;
   - a parcel with no geometry.

   Every RPC then gets handler tests, reader integration tests and an e2e smoke read.

## Constraints

- **Read-only.** Writing addresses, parties and geometry is a separate feature.
- **Adds to the API without breaking anything.** It adds new packages and services, and
  one new RPC on `ParcelsService`. No existing message changes. `buf breaking` must pass
  without `!`.
- **No new kind of data.** Party names and mailing addresses are already public through
  `primary_owner_*` and transfer parties. Parcel geometry is already public through the
  tiles, and an extent is coarser than the tile geometry. This makes these records
  addressable by ID; it doesn't widen what's exposed.
- **Extent only, not geometry.** Returning the full geometry (GeoJSON or WKT) is left
  out. The tiles already carry it for display, and a bounding box is all a client needs
  to find a parcel.
- **Only synthetic data**, in fixtures, docs and tests.
- The Helm chart, the gitops repo and the jurisdiction repos aren't edited from here.

## Success criteria

- A client holding any `address_id` or `party_id` that the public API returns can read that
  record, and can reach the parcels linked to it, with only public RPCs.
- A party that holds several parcels returns all of them, each with its share. A voided
  parcel or a voided party doesn't appear.
- A `legal_as_of` before an attribute change returns the earlier name or address.
- `GetParcelExtentsWGS84ByParcelId` returns a box that contains the parcel's
  current geometry. For the redrawn fixture parcel, it returns the current version's box,
  not the earlier one's. A voided parcel, or a parcel with no geometry, is absent from
  the response.
- The new services are generated into the npm SDK and `openapi.yaml`, and are listed in
  the public API README and `docs/architecture.md`.

## Open questions (carry into spec)

- **Names.** Should the packages be `civil.public.addresses.v1` and
  `civil.public.parties.v1`, with services `AddressesService` and `PartiesService`, to
  follow `landuses` and `RealPropertyTransfersService`?
- **Reverse lookup for addresses.** Should it be one RPC that returns parcels,
  improvements and parties together, or three separate ones? Does it need pagination? A
  shared mailing address, such as a management company, can be referenced by many
  parties.
- **Country and administrative area.** Should they be returned as codes (`ZZ`), names, or
  both? `country_id` and `administrative_area_id` point at lookup tables.
- **Transfers by party.** Should `RealPropertyTransfersService` add
  `GetTransfersByPartyId` in this feature, or in a later one? It is the natural next hop
  from a party.
- **Improvements.** There's no get-improvement-by-ID RPC either, so an improvement
  returned by the address reverse lookup is also a dead end. Should that RPC be added
  here, or get its own intent?
- **Extent message.** Should `GetParcelExtentsWGS84ByParcelId` reuse the shape of
  `GetEstimatedParcelsExtentWGS84Response` through a new shared `Extent` message, or
  define its own? Does it also need a `...ByFeatureId` variant, as the other parcel
  reads have, or `legal_as_of`?
- **Parcel search.** There's no parcel list or search (by address text or owner name).
  That's left out here and needs its own intent, but the spec should say whether the
  address lookup is meant to support it later.
