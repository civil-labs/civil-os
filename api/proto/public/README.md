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
| `civil.public.instance.v1` | `InstanceService` | Instance metadata and auth issuer (no token needed) |
| `civil.public.dex.v1` | `DexService` | Dex OIDC client administration, when the instance runs Dex |

`civil.public.sales.v1` (`SalesService`) and `civil.public.tiles.v1`
(`TileService`) are defined but **not served yet**. The gateway serves sales
comparables through `ParcelsService.GetSalesComparables`. It serves vector tiles
over plain HTTP at `/tiles/get_parcel_tiles/{z}/{x}/{y}`, with the same bearer
token, rather than through `TileService`.

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
