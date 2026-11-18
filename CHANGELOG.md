# Changelog

## [0.5.0](https://github.com/civil-labs/civil-os/compare/v0.4.0...v0.5.0) (2026-11-18)


### ⚠ BREAKING CHANGES

* rename the neighborhood definitions map to neighborhood_definitions ([#53](https://github.com/civil-labs/civil-os/issues/53))

### Bug Fixes

* rename the neighborhood definitions map to neighborhood_definitions ([#53](https://github.com/civil-labs/civil-os/issues/53)) ([ad51c3b](https://github.com/civil-labs/civil-os/commit/ad51c3b2adbbb6fdb742e11ff16cffd5a54c64cb))

## [0.4.0](https://github.com/civil-labs/civil-os/compare/v0.3.0...v0.4.0) (2026-11-18)


### ⚠ BREAKING CHANGES

* return neighborhood definitions and their neighborhoods ([#52](https://github.com/civil-labs/civil-os/issues/52))

### Features

* parcel extents and transfers by party ([#49](https://github.com/civil-labs/civil-os/issues/49)) ([716979f](https://github.com/civil-labs/civil-os/commit/716979fef39aee9ea3f0e3eacff9f686c4edffca))
* public addresses and parties read API ([#48](https://github.com/civil-labs/civil-os/issues/48)) ([45b030c](https://github.com/civil-labs/civil-os/commit/45b030c7d581d557ef90dddbf19da51ff1f1d0cf))


### Bug Fixes

* resolve addresses and owners at the legal time and hide voided records ([#46](https://github.com/civil-labs/civil-os/issues/46)) ([042d9ff](https://github.com/civil-labs/civil-os/commit/042d9fff883cc971073b4d053b6903a929a31add))
* return neighborhood definitions and their neighborhoods ([#52](https://github.com/civil-labs/civil-os/issues/52)) ([373807e](https://github.com/civil-labs/civil-os/commit/373807e76a593137d0ee96c9839c69c84369b988))

## [0.3.0](https://github.com/civil-labs/civil-os/compare/v0.2.2...v0.3.0) (2026-11-15)


### ⚠ BREAKING CHANGES

* remove the unserved SalesService and TileService ([#21](https://github.com/civil-labs/civil-os/issues/21))

### Features

* public real property transfers API with cursor pagination ([#24](https://github.com/civil-labs/civil-os/issues/24)) ([75e2a69](https://github.com/civil-labs/civil-os/commit/75e2a6949870f3213f6cfabcc58974dbe990dbea))
* remove the unserved SalesService and TileService ([#21](https://github.com/civil-labs/civil-os/issues/21)) ([5a23dfd](https://github.com/civil-labs/civil-os/commit/5a23dfd7ae81678e2700a371c1375d82b9c132a6))
* serve real property transfers from the database reader ([#23](https://github.com/civil-labs/civil-os/issues/23)) ([dacc0bd](https://github.com/civil-labs/civil-os/commit/dacc0bd3b0b764d5663de87267dcaabd57820e82))

## [0.2.2](https://github.com/civil-labs/civil-os/compare/v0.2.1...v0.2.2) (2026-11-14)


### Bug Fixes

* **ci:** no :latest tag, exact Node/npm pins, stricter breaking-change bypass ([#14](https://github.com/civil-labs/civil-os/issues/14)) ([7b77248](https://github.com/civil-labs/civil-os/commit/7b77248dfda95cbbbeb78a3d78881505e1d64d54))

## [0.2.1](https://github.com/civil-labs/civil-os/compare/v0.2.0...v0.2.1) (2026-11-14)


### Features

* docs, agent guardrails and mise stack tasks ([#11](https://github.com/civil-labs/civil-os/issues/11)) ([1cd6430](https://github.com/civil-labs/civil-os/commit/1cd6430f3be4139aeb5dbe9f7dabe76ad3a91c90))
* **public-api:** publish the public API as @civil-labs/civil-os-api ([#10](https://github.com/civil-labs/civil-os/issues/10)) ([5e773c1](https://github.com/civil-labs/civil-os/commit/5e773c1e2bfdde87fc4788c42570a6333511a68e))

## 0.2.0 (2026-11-14)


### ⚠ BREAKING CHANGES

* CIVIL_OS_ env var contract, slog log levels, fail-fast auth ([#4](https://github.com/civil-labs/civil-os/issues/4))
* mesh read/write split and working database-writer ([#3](https://github.com/civil-labs/civil-os/issues/3))

### Features

* CIVIL_OS_ env var contract, slog log levels, fail-fast auth ([#4](https://github.com/civil-labs/civil-os/issues/4)) ([fda70d6](https://github.com/civil-labs/civil-os/commit/fda70d61949a8c10951596ec3d376ddfad3b7191))
* docker compose quickstart ([#7](https://github.com/civil-labs/civil-os/issues/7)) ([344e3a6](https://github.com/civil-labs/civil-os/commit/344e3a68febc463f4e01494f16eb7a3486cfcd92))
* mesh read/write split and working database-writer ([#3](https://github.com/civil-labs/civil-os/issues/3)) ([ea17247](https://github.com/civil-labs/civil-os/commit/ea1724757a4be25985249a66e9258cb18651b521))
