# Intent: replace the unserved Sales and Tile services with a real property transfers API

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from the docs PR (#11) findings at the owner's request, without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The public API defines two services that the gateway doesn't serve:

| Service | Why it exists | Why it's wrong |
|---|---|---|
| `civil.public.tiles.v1.TileService` | Only to put the tile endpoint into the generated OpenAPI spec, through a `google.api.http` annotation | Tiles are served by Martin through the gateway's `/tiles/` proxy, not by an RPC. A protobuf service that no server implements makes the SDK and OpenAPI spec advertise a call that doesn't exist. |
| `civil.public.sales.v1.SalesService` (plus mesh `SaleReadService`) | The original sales API | No reader or gateway code implements it. The database has since renamed sales to **real property transfers** (`core.real_property_transfers` and related tables). The `Sale` message models one seller and one buyer, but the schema allows many grantors and grantees per transfer, each with a share, across parcels and improvements. |

Both are generated into the Go packages, the npm SDK and `openapi.yaml`. Clients
can call them and get `404` / `Unimplemented`. Meanwhile there's no public way
to read transfer history, apart from the latest price that
`ParcelsService.GetSalesComparables` derives.

## Proposed outcome

1. **`TileService` is deleted.** The `civil.public.tiles.v1` package and its
   generated code are removed. The tile endpoint is documented in prose
   (`docs/architecture.md`, the public API README) instead of the OpenAPI spec.
2. **`SalesService` and mesh `SaleReadService` are deleted**, with their generated
   code.
3. **A new public service reads real property transfers**, served by the
   reader through a new mesh `RealPropertyTransferReadService`. It follows the
   existing patterns: protovalidate rules, UUID IDs, and voided parcels hidden.
   It models transfers as the schema does:
   - date and amount;
   - deed references;
   - grantor and grantee parties with their transferred shares;
   - the parcels and improvements involved;
   - transfer codes and their types.
4. **The synthetic fixtures include transfers**, so the new service has
   integration and e2e coverage like every other read.

## Constraints

- **Timing.** No `public-api-v*` release exists yet. Removing `TileService` and
  `SalesService` before the first public API release means they never ship, and
  the change isn't breaking for anyone. If a release happens first, the removal
  is a breaking change (`feat!:`), which pre-1.0 means a minor bump.
- **Leave `ParcelsService.GetSalesComparables` alone.** It already reads
  transfers, and its name and shape are out of scope here. Renaming it to match
  "transfers" would be a separate breaking change.
- **Read-only.** Writing transfers is a separate feature.
- **Only synthetic data.** No real deed books, pages, URIs, party names or
  amounts in fixtures, docs or tests.
- The Helm chart, the gitops repo and the jurisdiction repos aren't edited from
  here.

## Success criteria

- `TileService`, `SalesService` and `SaleReadService` no longer exist in
  `api/proto`, the generated Go code, the npm package or `openapi.yaml`.
- The new transfers service is served by the gateway. Every RPC has handler
  tests and reader integration tests against fixture transfers. The e2e smoke
  test reads at least one transfer.
- A transfer with several grantors and grantees, and one covering more than one
  parcel, round-trip correctly in tests.
- A transfer involving only a voided parcel doesn't appear in results.
- The public API README and `docs/architecture.md` no longer list unserved
  services.

## Open questions (carry into spec)

- **Queries.** Which lookups do clients need: by parcel ID, by map feature ID,
  by polygon, or by date and amount range? `GetSalesByPolygon` suggests a
  polygon search was planned.
- **Names.** What should the public service and messages be called:
  `RealPropertyTransferService` / `Transfer`, or something shorter?
- **Party details.** Should parties be returned inline (name and mailing
  address), as with `primaryOwnerName` today, or only as `party_id`s?
- **Amounts.** Keep decimal strings, as `Sale.sale_price` does
  (`numeric(19,4)`), or use a money type?
- **Codes.** Are transfer codes (for example, qualified or unqualified sale) in
  scope for filters, or only returned?
- **History.** Should `system_as_of` time travel, which the old request had,
  be supported now or left for the history APIs?
- **Tile documentation.** With `TileService` gone, should the gateway expose
  Martin's TileJSON (`/tiles/get_parcel_tiles`) as the documented contract? See
  [`docs/tile-source-allowlist/intent.md`](../tile-source-allowlist/intent.md).
