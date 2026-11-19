# civil-os public API

The contract that civil-os clients use. The gateway serves these services over
[ConnectRPC](https://connectrpc.com), which also accepts gRPC and gRPC-Web, on
one port.

| Package | Service | What it serves |
|---|---|---|
| `civil.public.parcels.v1` | `ParcelsService` | Parcel details, attribute lookups by parcel or map feature, parcel extents (bounding boxes), stats, comparables, address search (`SearchParcelsAddresses`, paginated), `UpdateParcel` |
| `civil.public.improvements.v1` | `ImprovementsService` | Improvement types and conditions |
| `civil.public.landuses.v1` | `LandUsesService` | Land uses and land-use types |
| `civil.public.zoning.v1` | `ZoningService` | Zoning designations |
| `civil.public.neighborhoods.v1` | `NeighborhoodService` | Neighborhood definitions (with `is_legal`), and neighborhoods with the definitions that use them, optionally filtered by definition. Both RPCs accept Connect GET. |
| `civil.public.valuations.v1` | `ValuationService` | Valuations |
| `civil.public.transfers.v1` | `RealPropertyTransfersService` | Real property transfers by transfer, parcel, map feature or party, with grantors, grantees, shares, codes and recordings. Paginated. |
| `civil.public.addresses.v1` | `AddressesService` | Addresses, and the parcels, improvements and parties at an address. The lookups by address are paginated. |
| `civil.public.parties.v1` | `PartiesService` | Parties (owners, grantors, grantees) and the parcels a party holds. Paginated. |
| `civil.public.instance.v1` | `InstanceService` | Instance metadata and auth issuer (no token needed) |
| `civil.public.dex.v1` | `DexService` | Dex OIDC client administration, when the instance runs Dex |

Two packages only hold messages that other services share:
`civil.public.pagination.v1` (`PageInfo`) and `civil.public.recordings.v1`
(`Recording`, a document recorded with the county clerk).

Sales comparables are served by `ParcelsService.GetSalesComparables`.

Requests are validated with [protovalidate](https://protovalidate.com) rules
declared in the protos. For example, every parcel ID must be a UUID.

## Using it

- **TypeScript / JavaScript:** [`@civil-labs/civil-os-api`](https://www.npmjs.com/package/@civil-labs/civil-os-api)
  on npm. See [`sdk/js`](../../../sdk/js/README.md).
- **Go:** import `github.com/civil-labs/civil-os/api/gen/go/civil/public/<domain>/v1`
  and its `<domain>v1connect` client package.
- **Anything else:** the OpenAPI spec (`openapi.yaml`) is attached to every
  `public-api-v*` GitHub Release. You can also generate your own client from
  these protos with [buf](https://buf.build).

## Pagination

APIs that can return many results are paginated with cursors.
`RealPropertyTransfersService` was the first, followed by the address and party
lookups and `ParcelsService.SearchParcelsAddresses`. They share these rules:

- **Request:** `optional int32 first` (the page size) and
  `optional string after` (a cursor from a previous page).
- **Response:** the results and a `civil.public.pagination.v1.PageInfo
  page_info`. New paginated RPCs, starting with `SearchParcelsAddresses`,
  return `repeated <Thing> <things>` with no edges. The older RPCs still
  return `repeated <Thing>Edge edges`, each with a `cursor` and the `node`,
  until [`pagination-without-edges`](../../../docs/pagination-without-edges/intent.md)
  removes them. Either way, `page_info.end_cursor` is all you need to page.
- **Page size:** `first` is 1 to 100, and defaults to 25 when unset. A larger
  value is rejected with `InvalidArgument`, not reduced. The limits are part of
  the contract and the same on every instance.
- **Reading every page:** repeat the request with
  `after = page_info.end_cursor` while `page_info.has_next_page` is true. Use
  `has_next_page`, not the number of results, to detect the end.
- **Cursors are opaque.** Don't parse or build them. A cursor is only valid
  with the same filter (IDs, time range, or search query) as the request it
  came from; otherwise the request fails with `InvalidArgument`. `first` may
  change between pages.
- **Consistency:** pages are keyset-based, so results added or removed between
  requests don't shift later pages. The lookups have no snapshot: each page
  reads "now" again. **Search pages share a snapshot:** when `legal_as_of` is
  unset, the first page fixes "now", and later pages resolve at that time, so
  ranks don't move between pages.
- **Search has a cap.** `SearchParcelsAddresses` returns at most 1,000 hits
  across all pages. After that, `has_next_page` is false: refine the query.
- Only forward pagination (`first` / `after`) is supported for now.

## Searching parcels by address

`ParcelsService.SearchParcelsAddresses` takes free text (`query`, 1 to 200
characters, with at least 3 letters or digits) and returns visible parcels
whose address at `legal_as_of` matches, best first. Case and punctuation are
ignored, so `12, FIXTURE-way` finds the same parcels as `12 Fixture Way`.
Hits are ranked: addresses starting with the query, then those with a word
starting with it, then those containing it, then similar ones (typos). Each
hit has the `parcel_id`, its `feature_id`, the `address_id` that matched and
its `formatted_address`. Load details with
`GetParcelsWithImprovementSummaryByParcelId`. Owner names, parcel numbers,
abbreviations (`St` and `Street`) and accents aren't matched yet.

## Neighborhoods

Parcels are grouped into neighborhoods under one or more *neighborhood
definitions*. An instance can have several definitions, and none, one or
several of them can be legal. A parcel has at most one neighborhood per
definition at a time.

1. **Pick a definition.** `NeighborhoodService.GetNeighborhoodDefinitions`
   returns every definition, keyed by ID, with its `name` and `is_legal`.
   A legal definition's assignments are dated, so a parcel's neighborhood
   under it depends on the legal time. A non-legal one's usually aren't.
2. **List its neighborhoods.** `GetNeighborhoods` with
   `neighborhood_definition_id` returns the neighborhoods that definition
   uses. Without it, you get every neighborhood. Either way, each neighborhood
   lists all the definitions that use it in `neighborhood_definition_ids`. A
   definition "uses" a neighborhood when it has ever assigned a parcel to it,
   so a neighborhood with no parcels yet belongs to no definition.
3. **Read a parcel's neighborhood.** Pass the same `neighborhood_definition_id`
   on `ParcelsService.GetParcelsWithImprovementSummaryByParcelId` (or
   `...ByFeatureId`). For a legal definition, also set `legal_as_of`, which
   defaults to now. `ParcelDetails.neighborhood_id` is unset when no
   definition is requested, or the parcel has no neighborhood under it then.
   There is no default definition.

Both `NeighborhoodService` RPCs have no side effects, so they can be called
with Connect GET and cached.

## Vector tiles

Map tiles aren't part of this protobuf API. The gateway serves them over plain
HTTP, with the same bearer token as the RPCs:

- `GET /tiles/get_parcel_tiles/{z}/{x}/{y}` returns a Mapbox Vector Tile of
  parcel geometries.
- `GET /tiles/get_parcel_tiles` returns its [TileJSON](https://github.com/mapbox/tilejson-spec),
  which map libraries such as MapLibre can use as a source URL.

The tiles are served by [Martin](https://maplibre.org/martin/) behind the
gateway. Which tile sources are published, and whether the TileJSON is a
guaranteed contract, is decided by
[`docs/tile-source-allowlist`](../../../docs/tile-source-allowlist/intent.md).
Until then, treat `get_parcel_tiles` as the only supported source.

## Versioning and compatibility

This directory is released on its own, separately from the app:

- **Tags:** `public-api-vX.Y.Z`. The npm package has the same version.
- **Version bumps:** release-please reads conventional commits that touch this
  directory. Before 1.0, a breaking change bumps the minor version, and a
  `feat` bumps the patch.
- **Breaking-change check:** CI runs `buf breaking` against the last
  `public-api-v*` tag on every PR. A breaking change fails the check unless the
  PR title marks it as breaking (`feat!:`).

The mesh protos (`api/proto/mesh`) are internal to civil-os, have no
compatibility promise, and are not published.
