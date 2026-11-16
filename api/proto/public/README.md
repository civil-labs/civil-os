# civil-os public API

The contract that civil-os clients use. The gateway serves these services over
[ConnectRPC](https://connectrpc.com), which also accepts gRPC and gRPC-Web, on
one port.

| Package | Service | What it serves |
|---|---|---|
| `civil.public.parcels.v1` | `ParcelsService` | Parcel details, attribute lookups by parcel or map feature, stats, comparables, `UpdateParcel` |
| `civil.public.improvements.v1` | `ImprovementsService` | Improvement types and conditions |
| `civil.public.landuses.v1` | `LandUsesService` | Land uses and land-use types |
| `civil.public.zoning.v1` | `ZoningService` | Zoning designations |
| `civil.public.neighborhoods.v1` | `NeighborhoodService` | Neighborhood definitions and neighborhoods |
| `civil.public.valuations.v1` | `ValuationService` | Valuations |
| `civil.public.transfers.v1` | `RealPropertyTransfersService` | Real property transfers by transfer, parcel or map feature, with grantors, grantees, shares, codes and recordings. Paginated. |
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

APIs that can return many results are paginated with cursors, following the
GraphQL [Cursor Connections Specification](https://relay.dev/graphql/connections.htm).
`RealPropertyTransfersService` was the first, and the address and party
lookups follow it. New paginated RPCs follow the same pattern:

- **Request:** `optional int32 first` (the page size) and
  `optional string after` (a cursor from a previous page).
- **Response:** `repeated <Thing>Edge edges`, each with a `cursor` and the
  `node`, and a `civil.public.pagination.v1.PageInfo page_info`.
- **Page size:** `first` is 1 to 100, and defaults to 25 when unset. A larger
  value is rejected with `InvalidArgument`, not reduced. The limits are part of
  the contract and the same on every instance.
- **Reading every page:** repeat the request with
  `after = page_info.end_cursor` while `page_info.has_next_page` is true. Use
  `has_next_page`, not the number of edges, to detect the end.
- **Cursors are opaque.** Don't parse or build them. A cursor is only valid
  with the same filter (IDs and time range) as the request it came from;
  otherwise the request fails with `InvalidArgument`. `first` may change
  between pages.
- **Consistency:** pages are keyset-based, so results added or removed between
  requests don't shift later pages, but there's no snapshot either.
- Only forward pagination (`first` / `after`) is supported for now.

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
