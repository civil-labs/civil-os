# Spec: civil-os open-source Go monorepo

| | |
|---|---|
| **Implements** | [`intent.md`](intent.md) (accepted in `475ff11`) |
| **Author** | Claude (drafted for Jackson, product owner) |
| **Date** | 2026-11-13 |
| **Status** | Draft. C1–C6 decided by the owner on 2026-11-13; C7–C9 are owner actions after implementation. Awaiting final review. |

## 1. Requirements

### Functional

| ID | Requirement | Intent source |
|----|-------------|---------------|
| F1 | A single repo `github.com/civil-labs/civil-os` with one Go module containing gateway, database-reader, database-writer, tile-server, the proto definitions and the database schema/migrations. | Outcome 1 |
| F2 | Generated Go code is committed. Public-API Go packages are importable by external users; mesh Go packages are under `internal/`. | Decisions |
| F3 | The mesh `ParcelsService` is split into `ParcelReadService` (reader) and `ParcelWriteService` (writer). All other mesh services are renamed to the `<Entity>ReadService` pattern. The public API keeps its current service names, including a single `ParcelsService`. | Outcome 4, Decisions |
| F4 | database-editor is renamed to database-writer in every path, module, binary, image, env var, log string and doc. | Outcome 4 |
| F5 | The writer implements `ParcelWriteService.UpdateParcel` against the database. The gateway's public `UpdateParcel` validates the request, forwards it to the writer and returns the writer's result or error. | Outcome 4 |
| F6 | The gateway has no AWS coupling: it reaches backends through configured host names (Kubernetes Service DNS in-cluster) and calls no cloud discovery API. Optional S3 support for instance metadata (gocloud.dev `s3blob`) stays, alongside GCS, Azure and file, so the AWS SDK remains an indirect dependency. *(Amended 2026-11-14, owner decision.)* | Outcome 7 |
| F7 | An app release `vX.Y.Z` publishes `ghcr.io/civil-labs/civil-os/{gateway,database-reader,database-writer,tile-server}:vX.Y.Z` and a migration ConfigMap artifact at the same version. The tag is the Go module version. | Outcome 2 |
| F8 | A Public API release `public-api-vX.Y.Z` publishes `@civil-labs/civil-os-api@X.Y.Z` to npm and attaches the public OpenAPI spec. Its structure accepts further language SDKs without renaming. | Outcome 3 |
| F9 | Each merge to `main` publishes `:sha-<short>` images for all four services. No `latest` tag is ever published. | Decisions |
| F10 | `docker compose up` from a fresh clone runs the full stack on synthetic data, with a working login and a working parcel read, parcel update and tile request. | Outcome 5 |
| F11 | The repo contains a README (with a quickstart), `docs/architecture.md`, `CLAUDE.md`, and `docs/github-setup.md` listing the owner's manual configuration steps. | Outcome 5, Manual steps |
| F12 | Unit, gateway-handler, integration, end-to-end smoke and contract test layers exist and run in CI. | Outcome 6 |

### Non-functional

| ID | Requirement |
|----|-------------|
| N1 | **No customer or jurisdiction data, ever.** No config, fixture, name, hostname or ID from any real deployment. All fixtures are synthetic. CI enforces this with a denylist check (§9). |
| N2 | **Runtime behavior of existing reads is unchanged.** Refactors are limited to the renames, the read/write split, the writer, AWS removal and changes needed for testability. |
| N3 | **Env vars are namespaced per service** as `CIVIL_OS_<SERVICE>_<SETTING>` (§5.5), so all services can share one environment (single `.env`, single host, one shell) without collisions. Reader and writer DB credentials are separate by construction, and each service has a distinct default port. Old `CIVIL_*` names are not supported; a service that sees them refuses to start. |
| N4 | **Reproducible builds.** Base images, tool versions and actions are pinned to versions (no `alpine:latest`). |
| N5 | **No agent path to `main`.** All changes land through PRs approved by the owner. |
| N6 | **Repo is safe to make public.** No secrets in the tree. The imported history is already public. |

## 2. Repository layout

```
civil-os/
├── go.mod / go.sum              module github.com/civil-labs/civil-os
├── mise.toml                    go, buf, node pinned; atlas pinned for local authoring only; tasks: generate, build, test, lint, check, up, db:diff, db:validate
├── compose.yaml                 local stack (§7)
├── README.md  CLAUDE.md  LICENSE (MIT, unchanged)
├── release-please-config.json  .release-please-manifest.json
├── api/
│   ├── buf.yaml  buf.gen.yaml  buf.lock
│   ├── proto/public/civil/public/<domain>/v1/*.proto
│   ├── proto/mesh/civil/mesh/<domain>/v1/*.proto
│   └── gen/go/civil/public/...  generated, committed; the only importable generated code
├── internal/
│   ├── gen/go/civil/mesh/...    generated, committed; mesh code (not importable externally)
│   ├── envconfig/               shared CIVIL_OS_* env parsing (legacy rejection, log level, ports); approved deviation, 2026-11-14
│   └── testutil/                shared test helpers (Postgres container, fixtures, fake OIDC issuer)
├── services/
│   ├── gateway/                 package main + *_test.go, dockerfile
│   ├── database-reader/         package main + *_test.go, dockerfile
│   ├── database-writer/         package main + *_test.go, dockerfile
│   └── tile-server/             config.yaml, dockerfile (Martin)
├── database/
│   ├── atlas.hcl  schema/schema.pg.hcl  migrations/
│   └── fixtures/synthetic.sql   synthetic seed data, shared by compose and integration tests
├── sdk/js/                      package.json (@civil-labs/civil-os-api), README; generated code built at publish time
├── test/e2e/                    end-to-end smoke tests (build tag `e2e`)
├── dev/                         compose support: dex.yaml, instance-metadata.json
├── docs/
│   ├── architecture.md  github-setup.md
│   └── monorepo-migration/{intent,spec,plan}.md
└── .github/workflows/           ci.yml, pr-title.yml, images.yml, release-please.yml,
                                 public-api-release.yml, migrations-release.yml
```

Each service stays a `package main` directory. Moving code into shared `internal/` packages is not in scope, except for the generated mesh code and test helpers.

## 3. Migration of source repos and history

The intent leaves history import as an open question (Q4). **Decision: import history.** Using `git filter-repo --path-rename`, each repo's files go straight to their final paths, so blame works without `--follow` and the restructure needs no separate move commits.

| Source repo (branch) | Path mapping into civil-os |
|---|---|
| civil-gateway (`master`) | `/` → `services/gateway/` |
| civil-database-reader (`main`) | `/` → `services/database-reader/` |
| civil-database-editor (`main`) | `/` → `services/database-writer/` |
| civil-tile-server (`main`) | `/` → `services/tile-server/` |
| civil-blueprints (`master`) | `proto/` → `api/proto/`, `buf.*` → `api/` (`gen/` dropped) |
| civil-database-schema (`main`) | `/` → `database/` (`manifests/` and `Dockerfile` dropped) |

The rules:
- Each import is one merge commit (`--allow-unrelated-histories`) that contains no content changes.
- civil-api-go and civil-api-js are not imported, because they hold only bot-generated commits.
- Imported `.github/`, `mise.toml`, per-service `go.mod`/`go.sum` and the schema `Dockerfile` are removed in follow-up commits and replaced by the root equivalents.
- Historical ConfigMaps in `manifests/migrations/` are not carried over. They remain in the old repo, which stays readable after archiving.

## 4. API design

### 4.1 Code generation

`api/buf.gen.yaml` (managed mode):

| Output | Plugin | Destination | Go package prefix |
|---|---|---|---|
| Public Go messages + Connect | `protocolbuffers/go`, `connectrpc/go` | `api/gen/go` | `github.com/civil-labs/civil-os/api/gen/go` |
| Mesh Go messages + Connect | same | `internal/gen/go` | `github.com/civil-labs/civil-os/internal/gen/go` |
| Public JS/TS | `bufbuild/es` (`target=js+dts`) | `sdk/js/gen` (git-ignored, built at publish) | n/a |
| Public OpenAPI | `sudorandom-connect-openapi` | `api/gen/openapi` (git-ignored, release asset) | n/a |

Mesh and public use separate `buf generate` invocations (or `override` rules keyed by `module`), so each gets its own `go_package_prefix`. The mesh OpenAPI spec is no longer generated, because it describes an internal contract.

The workflow:
- `mise run generate` regenerates everything.
- CI fails if `api/gen/go` or `internal/gen/go` differs from a fresh generation.
- A Claude Code hook blocks direct edits under `api/gen/` and `internal/gen/`.

### 4.2 Mesh service renames

| Before | After | Served by |
|---|---|---|
| `civil.mesh.parcels.v1.ParcelsService` | `ParcelReadService` (35 read RPCs) + `ParcelWriteService` (`UpdateParcel`) | reader / writer |
| `civil.mesh.improvements.v1.ImprovementsService` | `ImprovementReadService` | reader |
| `civil.mesh.landuses.v1.LandUsesService` | `LandUseReadService` | reader |
| `civil.mesh.zoning.v1.ZoningService` | `ZoningReadService` | reader |
| `civil.mesh.valuations.v1.ValuationService` | `ValuationReadService` | reader |
| `civil.mesh.neighborhoods.v1.NeighborhoodService` | `NeighborhoodReadService` | reader |
| `civil.mesh.sales.v1.SalesService` | `SaleReadService` | unimplemented (unchanged state) |

The rules:
- Proto packages stay `v1`. Mesh renames are breaking, but the mesh is internal and every caller is in this repo, so all callers change in the same commit.
- The reader and writer gRPC health checkers report their new service names.
- The public protos are unchanged. That includes the `TileService` and `SalesService` definitions that have no implementation today.

### 4.3 Public API compatibility policy

This resolves intent open question Q2.
- **`buf breaking`** runs on every PR against the last `public-api-v*` tag and checks `api/proto/public` only.
  - A breaking change **fails** the check unless the PR title marks it breaking (`feat!:` / `BREAKING CHANGE`). Such a title makes release-please bump the Public API version.
  - Before 1.0, a breaking change bumps the minor version.
- **The mesh protos have no breaking check.**

## 5. Service design

### 5.1 Gateway

- **AWS removal.** Delete `backend.go`. `BackendManager` is never called today, because the gateway already dials the reader's configured host. Drop the Cloud Map (`servicediscovery`) module. `aws-sdk-go-v2` itself stays, as a dependency of the optional `s3blob` driver (F6).
- **New env var `CIVIL_OS_GATEWAY_WRITER_HOST`, optional.**
  - When set, the gateway creates a `ParcelWriteService` client and routes public `UpdateParcel` to it.
  - When unset, public `UpdateParcel` returns `Unavailable` ("writes are not enabled on this instance"). Existing deployments without a writer keep starting unchanged.
- **Public `UpdateParcel`.**
  - The request is validated by the existing protovalidate interceptor, which checks that IDs are UUIDs.
  - It is mapped to the mesh `UpdateParcelRequest` and forwarded.
  - Mesh error codes pass through unchanged.
- **Testability refactors (N2-compliant).**
  - `ParcelServer` and the other servers depend on the generated Connect client *interfaces* rather than concrete clients, so tests can substitute fakes.
  - `DexServer` depends on the `api.DexClient` interface, which it already does.
- **Dex admin API stays (C4)**, because some jurisdictions without their own OIDC server run Dex as their IdP. It is usable only when an endpoint is configured:
  - When `CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS` is unset or empty, the `DexService` handler is **not registered**. Requests to its path get Connect `Unimplemented` / HTTP 404, and startup logs one line at `INFO` saying the Dex admin API is disabled.
  - When it is set but the address is invalid (`grpc.NewClient` fails), the gateway exits at startup instead of logging and continuing. A configured-but-broken admin API is a misconfiguration.
  - When it is set but Dex is unreachable at call time, calls return `Unavailable`.
  - The handler stays behind `RequireAuth`, as today.
  - `main()` is split into `run(ctx, cfg) error` plus a thin `main`, so tests can boot the mux.
- **Load balancing.** One HTTP/2 connection per backend Service name, balanced per connection by Istio ambient. This limitation is documented in `docs/architecture.md`.

### 5.2 Database reader

- Rename only: implements the `*ReadService` handlers, with the same SQL and behavior.
- Add `run(ctx, cfg)` for test boot, the same pattern as the gateway.

### 5.3 Database writer (replaces database-editor)

- **Naming.** The module path no longer applies (single module). The binary is `writer`, the image is `civil-os/database-writer`, and the Linux user is `writeruser` (uid 1001).
- **Config.** Its own `CIVIL_OS_WRITER_*` variables (§5.5), default port 50052. The writer is expected to connect with a role that has write permissions, distinct from the reader's role; DB roles are owned by the infrastructure repos.
- **Services served.** `ParcelWriteService` plus gRPC health. Old stubs are deleted.
- **`UpdateParcel` behavior** (C1 and C2 decided):

  | Step | Behavior |
  |---|---|
  | Resolve parcel | `core.parcels.public_id = parcel_id`, `is_voided = false`. Missing or voided → `NotFound`. |
  | Resolve references | `address_id` → `addresses.public_id`; `land_use_id` → `land_uses.public_id`. A missing reference → `InvalidArgument` naming the field. |
  | Units | API feet / square feet are converted to stored meters / square meters (`÷ 3.28084`, `÷ 10.7639`), the inverse of the reader's conversion. |
  | `properties` | Must parse as a JSON object, else `InvalidArgument`. It **replaces** the stored value. The size limit (5000 bytes, DB check) surfaces as `InvalidArgument`. |
  | Partial update | Only fields present in the request change. An absent optional field leaves the stored value untouched. Clearing a field to NULL is not supported in this version. |
  | Target row | The `core.parcel_attributes` row whose `legal_valid_range` contains `now()`. Updated in place in one transaction, as a correction (C1 option a). `legal_valid_range` is not changed. The existing trigger writes the previous row to `parcel_attributes_history`. A separate effective-dated RPC for new legal facts may come later. |
  | `trace_id` | Taken from the incoming W3C `traceparent` header when present, else a random 32-hex value (C2). The gateway forwards `traceparent` to the writer. |
  | No current row | `FailedPrecondition`. |
  | Response | Empty `UpdateParcelResponse`, unchanged from the proto. |

### 5.4 Tile server

- Martin config is unchanged except that the connection string reads `${CIVIL_OS_TILE_SERVER_DATABASE_URL}` instead of `$DATABASE_URL`.
- The image pins `ghcr.io/maplibre/martin:1.15.0`, moves to `services/tile-server/` and is published as `civil-os/tile-server`.

### 5.5 Configuration (env var contract)

The scheme is `CIVIL_OS_<SERVICE>_<SETTING>`, with `<SERVICE>` one of `GATEWAY`, `READER`, `WRITER`, `TILE_SERVER`. `CIVIL_OS_LOG_LEVEL` is the only variable shared on purpose.

| Service | Old name | New name | Required | Default |
|---|---|---|---|---|
| all Go services | `CIVIL_VERBOSE` (bool) | `CIVIL_OS_LOG_LEVEL`, overridden per service by `CIVIL_OS_<SERVICE>_LOG_LEVEL` | no | `INFO` |
| gateway | `CIVIL_PORT` | `CIVIL_OS_GATEWAY_PORT` | no | `8080` |
| gateway | `CIVIL_AUTH_SERVER` | `CIVIL_OS_GATEWAY_AUTH_SERVER` | yes | |
| gateway | `CIVIL_IDP_HOST` | `CIVIL_OS_GATEWAY_IDP_HOST` | yes | |
| gateway | `CIVIL_ALLOWED_CLIENT_IDS` | `CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS` (JSON array) | yes | |
| gateway | `CIVIL_INSTANCE_METADATA_URL` | `CIVIL_OS_GATEWAY_INSTANCE_METADATA_URL` | yes | |
| gateway | `CIVIL_DB_READER_HOST` | `CIVIL_OS_GATEWAY_READER_HOST` | yes | |
| gateway | (new) | `CIVIL_OS_GATEWAY_WRITER_HOST` | no; unset disables writes | |
| gateway | `CIVIL_TILE_SERVER_HOST` | `CIVIL_OS_GATEWAY_TILE_SERVER_HOST` | yes | |
| gateway | `CIVIL_DEX_GRPC_ADDRESS` | `CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS` | no | |
| reader | `CIVIL_PORT` | `CIVIL_OS_READER_PORT` | no | `50051` |
| reader | `CIVIL_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | `CIVIL_OS_READER_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | yes (port defaults to `5432`) | |
| writer | `CIVIL_PORT` | `CIVIL_OS_WRITER_PORT` | no | `50052` |
| writer | `CIVIL_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | `CIVIL_OS_WRITER_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | yes (port defaults to `5432`) | |
| tile-server | `DATABASE_URL` | `CIVIL_OS_TILE_SERVER_DATABASE_URL` | yes | |

The rules:
- **Log level follows `slog` conventions.** The value is parsed with `slog.Level.UnmarshalText`. That accepts `DEBUG`, `INFO`, `WARN` and `ERROR` in any case, plus offsets like `INFO+2` or `DEBUG-4`.
  - The per-service variable wins over the shared one.
  - An unparseable value makes the service exit at startup with an error naming the variable. Today a bad `CIVIL_VERBOSE` silently falls back to the default.
  - The level is held in a `slog.LevelVar`, as the gateway already does, so it can be changed at runtime later without touching config.
  - The old `CIVIL_VERBOSE=true` corresponds to `DEBUG`.
  - The tile server (Martin, written in Rust) keeps its own `RUST_LOG` and is not covered by this variable.
- **Invalid values fail fast** *(added 2026-11-14 with the owner-approved gateway auth fix)*. An unparseable port or an invalid or empty `CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS` stops startup. Previously these logged a warning and fell back to a default or an empty list, which silently rejected every token. The gateway also exits if authentication setup fails, instead of starting with a nil auth middleware.
- **Old names fail fast.** At startup, each Go service checks for any set variable that starts with `CIVIL_` but not `CIVIL_OS_`. If it finds one, it exits with code 1 and lists the offending names along with a pointer to this table in `docs/architecture.md`. That catches a half-migrated deployment immediately.
- **No compatibility aliases.** The helm follow-up already has to change every image name, so it updates env vars in the same pass. The table in `docs/architecture.md` doubles as that follow-up's checklist.
- **Clashes are ruled out.** Defaults are unique per service, so all four services can run on one host without setting a single port.

## 6. Build, release and publishing

### 6.1 Images

- **Dockerfiles.** Each Go dockerfile builds from the repo root context: `go build -o /app/<bin> ./services/<svc>`.
- **Runtime stages.** Unchanged (non-root uid 1001, port 8080 or the existing default, `grpc_health_probe` for reader and writer), except that `alpine:latest` is pinned (N4).
- **`.dockerignore`.** A root `.dockerignore` excludes `.git`, `docs`, `sdk`, `test` and `api/gen/openapi`.

### 6.2 Release streams (release-please, manifest mode)

| Stream | Package path | Release type | Tag | Version also written to |
|---|---|---|---|---|
| App | `.` | `go` | `vX.Y.Z` (`include-component-in-tag: false`) | CHANGELOG.md |
| Public API | `api/proto/public` | `simple` | `public-api-vX.Y.Z` | `sdk/js/package.json` (`extra-files`) |

The rules:
- **Version bumps before 1.0:** `bump-minor-pre-major: true` and `bump-patch-for-minor-pre-major: true`, so breaking changes bump the minor version and `feat` bumps the patch.
- **Overlapping releases:** a public-proto change touches both package paths, so it releases both streams. That is intended, because the app serves the public API.
- **Starting versions (C3):** the app starts at **`v0.2.0`**, above the schema repo's last release `v0.1.8`, so ConfigMap names never go backwards or collide. The Public API starts at **`public-api-v0.1.0`**. Both are set in `.release-please-manifest.json`.
- **No Atlas in any workflow (C6).** The owner has a local Pro seat only, with no CI license. Workflows never install or run Atlas; see §6.4.

### 6.3 Workflows

| Workflow | Trigger | Does |
|---|---|---|
| `ci.yml` | PR, push to `main` | §8 test layers, lint, generated-code freshness, `buf breaking` (§4.3), no-customer-data check |
| `pr-title.yml` | PR | Conventional-commit title check |
| `release-please.yml` | push to `main` | Maintains release PRs; tags on merge. Uses the owner-provided App token so tags trigger the workflows below. |
| `images.yml` | push to `main`; tag `v*` | Matrix over the 4 services. On `main` it pushes `:sha-<short>`; on a tag it pushes `:vX.Y.Z`. Provenance attestations kept from the existing workflow. |
| `migrations-release.yml` | tag `v*` | Builds ConfigMap `civil-os-db-migrations-vX-Y-Z` from `database/migrations/` and attaches `migrations-configmap.yaml` to the GitHub Release. **Does not commit to the repo** (resolves intent Q1). |
| `public-api-release.yml` | tag `public-api-v*` | `buf generate` → builds `sdk/js` → `npm publish --provenance --access public`. Attaches the public `openapi.yaml` to the GitHub Release. |

### 6.4 Database migrations without Atlas in CI

Atlas is a **local authoring tool only**.

| Step | Where | How |
|---|---|---|
| Write the desired schema, generate a migration | Owner's machine | `mise run db:diff`: `atlas migrate diff --env local`, the existing task including line-ending normalization |
| Check `atlas.sum` integrity | Owner's machine | `mise run db:validate`: `atlas migrate validate`. `CLAUDE.md` tells agents to run it, or ask the owner to, whenever `database/migrations/` changes. |
| Check migrations apply | CI | Go integration tests apply every `database/migrations/*.sql` file in filename order to an empty PostGIS container with pgx. The files are plain SQL with no Atlas directives. |
| Check `atlas.sum` lists exactly the migration files | CI | A small Go test compares the file names in `atlas.sum` with the directory. It does not recompute hashes, which needs Atlas. |
| Package the ConfigMap | CI | `kubectl create configmap --dry-run=client` (no Atlas), as the old workflow already did |
| Apply migrations in clusters | Clusters | Unchanged and outside this repo (helm-owned). |

CI installs tools selectively (`mise install go buf node`), so the Atlas pin in `mise.toml` is never downloaded in workflows.

Later SDKs (Python, Rust, Swift) add jobs to `public-api-release.yml` and directories under `sdk/`, with no change to tags or names.

## 7. Local stack (`compose.yaml`)

| Service | Image / build | Role |
|---|---|---|
| `db` | `postgis/postgis:18-3.6` (pinned) | Postgres + PostGIS |
| `migrate` | `postgis` image, `psql` | Applies `database/migrations/*.sql` in filename order with `ON_ERROR_STOP`, then exits. No Atlas, so open-source users need no Atlas account. |
| `seed` | `postgis` image, `psql` | Loads `database/fixtures/synthetic.sql`, then exits |
| `dex` | `dexidp/dex` (pinned) | Local OIDC issuer: one static client and one static user, both clearly fake (`dev@example.com`). Its gRPC API is enabled and the gateway's `CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS` points at it, so the admin API works locally. |
| `database-reader`, `database-writer`, `tile-server`, `gateway` | built from `services/*` | the app; gateway exposed on `localhost:8080` |

Instance metadata is a synthetic `dev/instance-metadata.json`, mounted and read through gocloud `file://`. The README shows how to get a token from Dex and make an authenticated read, an update and a tile request with `curl`.

**Synthetic fixtures** (`database/fixtures/synthetic.sql`):
- About 20 parcels with geometries in a fictional area, with addresses, land uses, zoning, one neighborhood definition, improvements and one valuation.
- All IDs are fixed UUIDs, so tests can assert on them.
- The same file seeds compose and integration tests.

## 8. Test design

| Layer | Location | Infra | Covers |
|---|---|---|---|
| Unit | `services/*/*_test.go` | none | Gateway mesh→public mappers (all domains; nil/empty/partial). Config/env parsing for all three Go services: required vars, defaults, log level parsing (each named level, any case, `INFO+2`-style offsets, an invalid value rejected, the per-service level overriding the shared one), and refusal to start when any old `CIVIL_*` (non-`CIVIL_OS_`) name is set. A test that loads all three configs from one shared environment confirms there are no collisions. CORS middleware. Auth middleware against a fake OIDC issuer + JWKS (valid, wrong client ID, expired, bad signature, missing). Writer unit conversion and `properties` validation. |
| Gateway handler | `services/gateway/*_test.go` | in-memory fake mesh servers (`httptest`) | Every public RPC: routing, mapping, mesh error-code propagation, protovalidate rejection. `UpdateParcel` with and without `CIVIL_OS_GATEWAY_WRITER_HOST`. Dex RPCs against a fake `DexClient`. With no Dex address the `DexService` path returns `Unimplemented`; with an unreachable Dex calls return `Unavailable`. |
| Integration | `services/database-{reader,writer}/*_test.go` | testcontainers-go, `postgis` image, migrations applied in order, synthetic fixtures | **Reader:** every read RPC, both by-parcel and by-feature variants, plus not-found cases. **Writer:** full and partial updates, unit round-trip through the reader, `properties` replace, unknown parcel / address / land use, voided parcel, oversized `properties`, and a history row written. |
| Migrations | `database/*_test.go` | testcontainers PostGIS (no Atlas) | All migrations apply cleanly in order to an empty DB. The file list in `atlas.sum` matches the directory. Hash integrity is checked locally only (§6.4). |
| E2E smoke | `test/e2e` (`-tags e2e`) | `docker compose up --wait` | All health checks green. Token from Dex. Read a parcel → update it → read the updated value through the gateway. A tile request returns a non-empty MVT. |
| Contract | CI | buf | `buf lint`, `buf format --diff --exit-code`, generated-code freshness, `buf breaking` on public. |

The no-customer-data check is a CI step that greps the tree for a denylist of known jurisdiction identifiers, such as county names and deployment codes. The list itself is kept in a CI secret so it doesn't publish them.

## 9. Documentation

- **`README.md`:** what civil-os is, the architecture diagram link, the quickstart (`mise install && docker compose up`), the release model and the license.
- **`docs/architecture.md`:**
  - the service map (gateway → reader / writer / tile-server → Postgres) and public vs mesh APIs;
  - the bitemporal data model in brief;
  - the full env var table from §5.5, including the old → new mapping for the helm follow-up;
  - the load-balancing note (§5.1) and the release streams.
- **`CLAUDE.md` (≤ 1 page):** commands (`mise run generate|build|test|check|up`), layout, conventions (conventional commits, Read/Write mesh naming, never edit `*/gen/`), the no-customer-data rule, and known mistakes.
- **`docs/github-setup.md`:** the intent's manual steps, written as an exact checklist (secrets and their names, branch protection settings, App permissions, GHCR visibility, npm org and trusted publishing).

## 10. Areas of concern

| # | Concern | Recommendation | Needs |
|---|---|---|---|
| C1 | **Bitemporal semantics of `UpdateParcel`.** An edit can (a) correct the current legal row in place, or (b) close it and insert a new legal row. | **Decided: (a)** correction in place (§5.3). | Resolved |
| C2 | **`trace_id` is NOT NULL on every write**, but there is no tracing setup today. | **Decided:** take it from `traceparent`, else generate one (§5.3). | Resolved |
| C3 | **Version collision** with the schema repo's `v0.1.1`–`v0.1.8`. | **Decided:** app starts at `v0.2.0`, Public API at `public-api-v0.1.0` (§6.2). | Resolved |
| C4 | **Dex admin API.** Some jurisdictions without their own OIDC server still use Dex. | **Decided:** keep it, but make it unusable when no Dex endpoint is configured (§5.1). Compose runs Dex with the admin API enabled. | Resolved |
| C5 | **Dependency unification.** One module moves reader and writer up to the gateway's versions of shared deps. | **Accepted.** Covered by integration tests. | Resolved |
| C6 | **No Atlas CI license** (owner has a local Pro seat only). | **Decided:** no Atlas in any workflow or in compose; Atlas is a local authoring tool (§6.4). | Resolved |
| C7 | **The first image push needs GHCR permissions** and the release-please App token (manual steps). | Documented in `docs/github-setup.md`. The PR can merge before then; image workflows fail visibly until configured. | Owner action |
| C8 | **Live clusters still pull old image names.** | No change until the helm follow-up (intent constraint). Old repos keep publishing until then; avoid merging into them. | — |
| C9 | **The env var rename breaks the existing helm values.** New images refuse to start with old variable names. | Intended. The helm follow-up switches image names and env vars together, using the §5.5 table as its checklist. Live clusters are unaffected until then, because they still run the old images. | — |

## 11. Intent open questions

| Intent Q | Resolution |
|---|---|
| Q1 ConfigMap publishing | Resolved: GitHub Release asset, no commit-back (§6.3). |
| Q2 Public API breaking policy | Resolved: fail unless marked breaking (§4.3). |
| Q3 civil-api-js vs civil-api-ts repo | Resolved: the owner confirmed the GitHub repo is `civil-labs/civil-api-js`. It is archived after the helm follow-up. |
| Q4 History import | Resolved: import with path renames (§3). |

## 12. Acceptance criteria

Each criterion maps to the intent's success criterion with the same number.

1. A single PR in civil-os can change a proto, the generated code and every consumer, and CI passes on it.
2. `images.yml` builds all 4 images. On a release tag they appear as `ghcr.io/civil-labs/civil-os/<svc>:vX.Y.Z`.
3. `git log -- services/gateway/auth.go` and `git blame` show pre-migration commits.
4. On a fresh clone, `mise install && docker compose up --wait` succeeds and the README curl walkthrough works.
5. `ci.yml` runs unit, handler, integration, e2e, migration and contract layers, and all pass.
6. `release-please --dry-run` (or the first release PR) proposes both an app release and a `public-api` release.
7. `grep -ri editor` finds no service reference (excluding docs history). An e2e update persists and reads back through the gateway.
8. README, `docs/architecture.md`, `CLAUDE.md` and `docs/github-setup.md` exist, and every command in them works as written.
9. No gateway code calls AWS directly: `git grep aws-sdk-go-v2 -- services` returns nothing, and `go mod why -m github.com/aws/aws-sdk-go-v2/service/servicediscovery` reports the module is not needed. *(Amended 2026-11-14, owner decision.)*
