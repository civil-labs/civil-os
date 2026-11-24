# Changelog

## [0.6.0](https://github.com/civil-labs/civil-os/compare/public-api-v0.5.0...public-api-v0.6.0) (2026-11-24)


### ⚠ BREAKING CHANGES

* full and half bathrooms ([#88](https://github.com/civil-labs/civil-os/issues/88))

### Features

* full and half bathrooms ([#88](https://github.com/civil-labs/civil-os/issues/88)) ([13ca068](https://github.com/civil-labs/civil-os/commit/13ca0681bab681e5fd8a039c2fba91894682c799))

## [0.5.0](https://github.com/civil-labs/civil-os/compare/public-api-v0.4.1...public-api-v0.5.0) (2026-11-22)


### ⚠ BREAKING CHANGES

* `NeighborhoodService` is now `NeighborhoodsService`, and `ValuationService` is now `ValuationsService`, in the same packages with the same RPCs and messages. Generated clients change name. The mesh services between the gateway and its backends are renamed too: deploy the gateway, database reader and database writer of this release together. A mixed rollout fails every mesh call with `Unimplemented` until all three are updated.
* `GetParcelsByAddressId` and `GetParcelsByPartyId` moved to `ParcelsService`, `GetImprovementsByAddressId` to `ImprovementsService`, and `GetPartiesByAddressId` to `PartiesService`, with their request, response and result messages. Names and fields are unchanged. Call them on the new service, and import the messages from its package.
* Paginated responses no longer wrap results in edges. Read `response.transfers` (or `parcels`, `improvements`, `parties`) instead of `response.edges[].node`, and keep paging with `page_info.end_cursor`. Per-result cursors are gone.

### Features

* move reverse lookups to the service of what they return ([646b777](https://github.com/civil-labs/civil-os/commit/646b7770edb51011889e92927d1f7c14e4809b36))
* name services by their plural domain ([8f16227](https://github.com/civil-labs/civil-os/commit/8f1622791c80d19cd7750b18917e004bcfefada6))
* return paginated results without edges ([2fbb32a](https://github.com/civil-labs/civil-os/commit/2fbb32aa900014be3a88105cd960ea99b05b9b57))

## [0.4.1](https://github.com/civil-labs/civil-os/compare/public-api-v0.4.0...public-api-v0.4.1) (2026-11-20)


### Features

* search parcels by address ([#69](https://github.com/civil-labs/civil-os/issues/69)) ([2e4a27d](https://github.com/civil-labs/civil-os/commit/2e4a27d57aa0bd48a6df7fd6604a8665347d0c31))

## [0.4.0](https://github.com/civil-labs/civil-os/compare/public-api-v0.3.0...public-api-v0.4.0) (2026-11-18)


### ⚠ BREAKING CHANGES

* rename the neighborhood definitions map to neighborhood_definitions ([#53](https://github.com/civil-labs/civil-os/issues/53))

### Bug Fixes

* rename the neighborhood definitions map to neighborhood_definitions ([#53](https://github.com/civil-labs/civil-os/issues/53)) ([ad51c3b](https://github.com/civil-labs/civil-os/commit/ad51c3b2adbbb6fdb742e11ff16cffd5a54c64cb))

## [0.3.0](https://github.com/civil-labs/civil-os/compare/public-api-v0.2.0...public-api-v0.3.0) (2026-11-18)


### ⚠ BREAKING CHANGES

* return neighborhood definitions and their neighborhoods ([#52](https://github.com/civil-labs/civil-os/issues/52))

### Features

* parcel extents and transfers by party ([#49](https://github.com/civil-labs/civil-os/issues/49)) ([716979f](https://github.com/civil-labs/civil-os/commit/716979fef39aee9ea3f0e3eacff9f686c4edffca))
* public addresses and parties read API ([#48](https://github.com/civil-labs/civil-os/issues/48)) ([45b030c](https://github.com/civil-labs/civil-os/commit/45b030c7d581d557ef90dddbf19da51ff1f1d0cf))


### Bug Fixes

* return neighborhood definitions and their neighborhoods ([#52](https://github.com/civil-labs/civil-os/issues/52)) ([373807e](https://github.com/civil-labs/civil-os/commit/373807e76a593137d0ee96c9839c69c84369b988))

## [0.2.0](https://github.com/civil-labs/civil-os/compare/public-api-v0.1.0...public-api-v0.2.0) (2026-11-15)


### ⚠ BREAKING CHANGES

* remove the unserved SalesService and TileService ([#21](https://github.com/civil-labs/civil-os/issues/21))

### Features

* public real property transfers API with cursor pagination ([#24](https://github.com/civil-labs/civil-os/issues/24)) ([75e2a69](https://github.com/civil-labs/civil-os/commit/75e2a6949870f3213f6cfabcc58974dbe990dbea))
* remove the unserved SalesService and TileService ([#21](https://github.com/civil-labs/civil-os/issues/21)) ([5a23dfd](https://github.com/civil-labs/civil-os/commit/5a23dfd7ae81678e2700a371c1375d82b9c132a6))

## 0.1.0 (2026-11-14)


### Features

* **public-api:** publish the public API as @civil-labs/civil-os-api ([#10](https://github.com/civil-labs/civil-os/issues/10)) ([5e773c1](https://github.com/civil-labs/civil-os/commit/5e773c1e2bfdde87fc4788c42570a6333511a68e))
