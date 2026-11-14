# Architecture

civil-os is four services in front of one PostGIS database. Each deployment
serves one jurisdiction.

```
                 clients (web apps, scripts, SDKs)
                              │  HTTPS: Connect / gRPC / gRPC-Web, /tiles/
                              ▼
                     ┌─────────────────┐        ┌───────────────────┐
                     │     gateway     │───────▶│ OIDC issuer (Dex  │
                     │  auth, CORS,    │  JWKS  │ or the instance's │
                     │  public API     │        │ own IdP)          │
                     └─┬──────┬──────┬─┘        └───────────────────┘
          mesh API     │      │      │  HTTP proxy
     (Connect, HTTP/1) │      │      │
             ┌─────────┘      │      └──────────┐
             ▼                ▼                 ▼
     ┌───────────────┐ ┌───────────────┐ ┌─────────────┐
     │database-reader│ │database-writer│ │ tile-server │
     │  *ReadService │ │ParcelWrite-   │ │  (Martin)   │
     │               │ │  Service      │ │             │
     └───────┬───────┘ └───────┬───────┘ └──────┬──────┘
             └─────────────────┼────────────────┘
                               ▼
                     PostgreSQL + PostGIS
                   (bitemporal `core` schema)
```

| Service | Code | Default port | Role |
|---|---|---|---|
| gateway | `services/gateway` | 8080 | The only public entry point. Validates OIDC ID tokens, applies CORS, serves the public API, maps public requests to mesh calls, and proxies `/tiles/` to the tile server. |
| database-reader | `services/database-reader` | 50051 | Serves every mesh `*ReadService` with read-only SQL. |
| database-writer | `services/database-writer` | 50052 | Serves `ParcelWriteService` (`UpdateParcel`). Optional: without it, the gateway answers writes with `Unavailable`. |
| tile-server | `services/tile-server` | 3000 | [Martin](https://maplibre.org/martin/), serving vector tiles from the database's `core.get_parcel_tiles` function. |

## Public and mesh APIs

There are two sets of protobuf APIs, both under `api/proto`:

- **Public (`civil.public.*`)** is the contract for clients. It's versioned and
  released separately, and checked with `buf breaking`. See
  [`api/proto/public/README.md`](../api/proto/public/README.md). Its Go code in
  `api/gen/go` is importable by anyone.
- **Mesh (`civil.mesh.*`)** is internal traffic between the gateway and the
  database services. It carries no compatibility promise. Its Go code lives in
  `internal/gen/go`, so other modules can't import it.

The gateway owns the mapping between the two, which lets the mesh change
without breaking clients. Generated Go code for both is committed.
`mise run generate` rebuilds it, and CI fails if it's stale.

### Read/write split

Mesh services follow one naming pattern: `<Entity>ReadService` is served by the
reader, and `<Entity>WriteService` is served by the writer. Today the only write
service is `ParcelWriteService`. The public API keeps one `ParcelsService` for
both. The gateway sends `UpdateParcel` to the writer and every other parcel RPC
to the reader.

Each service gets its own database credentials
(`CIVIL_OS_READER_DB_*` and `CIVIL_OS_WRITER_DB_*`). Deployments should give the
reader a read-only role. The roles themselves are created outside this repo.

## Data model in brief

The schema is authored in `database/schema/schema.pg.hcl`. Versioned SQL
migrations in `database/migrations/` are generated from it.

- **Stable identity.** Entities (`core.parcels`, `core.improvements`, …) carry an
  internal key and a `public_id` UUID. Only the UUID appears in the API.
- **Legal time.** Facts that change in the real world, such as geometry,
  attributes, ownership and zoning, live in tables with a `legal_valid_range`.
  Exclusion constraints stop two facts about the same entity from overlapping
  in legal time. Reads return the row valid now.
- **System time.** Every such table has a `*_history` twin with a
  `system_valid_range`. Triggers copy the previous row there on every update or
  delete, so the database records what it believed and when. Every write
  carries a `trace_id`. The writer takes it from the request's W3C
  `traceparent` header, or generates one.
- **Voiding.** A parcel recorded in error is marked `is_voided`. Every current
  read hides voided parcels, and `UpdateParcel` treats them as not found.
- **Corrections, not new facts.** `UpdateParcel` corrects the current legal row
  in place, and the history trigger keeps the previous value. Recording a new
  legal fact with its own effective date would be a separate RPC.

Migrations are plain SQL. CI applies them in order to an empty PostGIS
container. [Atlas](https://atlasgo.io) is used only on a developer machine to
generate them (`mise run db:diff`) and to check `atlas.sum`
(`mise run db:validate`).

> **Known gap:** Martin publishes every geometry table it finds, not just
> `get_parcel_tiles`, including voided parcels and history. See
> [`docs/tile-source-allowlist/intent.md`](tile-source-allowlist/intent.md).

## Configuration

Every variable is named `CIVIL_OS_<SERVICE>_<SETTING>`, so all four services can
share one environment without collisions. `CIVIL_OS_LOG_LEVEL` is the only
variable shared on purpose.

The rules:

- **Fail fast.** A service exits at startup if a required variable is missing,
  a value doesn't parse, or any pre-rename `CIVIL_*` variable (one without the
  `OS_`) is set. The error names the variable.
- **Log levels** use `slog` syntax: `DEBUG`, `INFO`, `WARN` or `ERROR` in any
  case, or an offset such as `INFO+2`. A service's own
  `CIVIL_OS_<SERVICE>_LOG_LEVEL` overrides the shared one. The tile server uses
  Martin's `RUST_LOG` instead.

The Old name column maps each variable to the pre-monorepo name it replaced.
The rename isn't backwards compatible, so deployments must switch image names
and variables together.

| Service | Variable | Old name | Required | Default |
|---|---|---|---|---|
| all Go services | `CIVIL_OS_LOG_LEVEL`, overridden by `CIVIL_OS_<SERVICE>_LOG_LEVEL` | `CIVIL_VERBOSE` (`true` = `DEBUG`) | no | `INFO` |
| gateway | `CIVIL_OS_GATEWAY_PORT` | `CIVIL_PORT` | no | `8080` |
| gateway | `CIVIL_OS_GATEWAY_AUTH_SERVER`: the OIDC issuer. A bare host means `https://`, or give a full `http://` or `https://` URL. | `CIVIL_AUTH_SERVER` | yes | |
| gateway | `CIVIL_OS_GATEWAY_IDP_HOST`: where the gateway fetches the issuer's keys, if that differs from the issuer | `CIVIL_IDP_HOST` | yes | |
| gateway | `CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS`: a JSON array of accepted token audiences | `CIVIL_ALLOWED_CLIENT_IDS` | yes | |
| gateway | `CIVIL_OS_GATEWAY_INSTANCE_METADATA_URL`: `file://`, `s3://`, `gs://` or `azblob://` | `CIVIL_INSTANCE_METADATA_URL` | yes | |
| gateway | `CIVIL_OS_GATEWAY_READER_HOST` | `CIVIL_DB_READER_HOST` | yes | |
| gateway | `CIVIL_OS_GATEWAY_WRITER_HOST` | (new) | no; unset disables writes | |
| gateway | `CIVIL_OS_GATEWAY_TILE_SERVER_HOST` | `CIVIL_TILE_SERVER_HOST` | yes | |
| gateway | `CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS`: unset disables `DexService` | `CIVIL_DEX_GRPC_ADDRESS` | no | |
| reader | `CIVIL_OS_READER_PORT` | `CIVIL_PORT` | no | `50051` |
| reader | `CIVIL_OS_READER_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | `CIVIL_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | yes (port defaults to `5432`) | |
| writer | `CIVIL_OS_WRITER_PORT` | `CIVIL_PORT` | no | `50052` |
| writer | `CIVIL_OS_WRITER_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | `CIVIL_DB_HOST` / `_PORT` / `_USER` / `_PASS` / `_NAME` | yes (port defaults to `5432`) | |
| tile-server | `CIVIL_OS_TILE_SERVER_DATABASE_URL` | `DATABASE_URL` | yes | |

`compose.yaml` is a complete working example of this configuration.

## Load balancing

The gateway calls the reader and writer with Go's default HTTP client, using
the Connect protocol over plain HTTP/1.1 to the configured host name. It reuses
keep-alive connections: it opens more under concurrent load and keeps up to two
idle connections per host.

Kubernetes `ClusterIP` Services, and Istio ambient's ztunnel, balance
**connections, not requests**. Each gateway pod's traffic therefore goes to
whichever backend replicas its pooled connections happen to reach. Load spreads
across replicas only roughly, and new replicas get little traffic until
connections are reopened.

If that matters, run more gateway replicas, or add a request-level (L7)
balancer between the gateway and the backends, such as an Istio waypoint proxy.

## Releases

Two independent streams, both cut by
[release-please](https://github.com/googleapis/release-please) from
conventional commits on `main`:

| Stream | Path | Tag | Publishes |
|---|---|---|---|
| App | everything | `vX.Y.Z` | the four images at `ghcr.io/civil-labs/civil-os/<service>:vX.Y.Z`, and `migrations-configmap.yaml` (ConfigMap `civil-os-db-migrations-vX-Y-Z`) on the GitHub Release |
| Public API | `api/proto/public` | `public-api-vX.Y.Z` | `@civil-labs/civil-os-api` on npm with provenance, and `openapi.yaml` on the GitHub Release |

How versions move:

- **One change, two streams.** A change to a public proto touches both paths, so
  it releases both streams.
- **Before 1.0,** a breaking change bumps the minor version and a `feat` bumps
  the patch.
- **Every merge to `main`** also pushes `:sha-<short>` images. No `latest` tag
  is published.

Cluster rollout (Helm values, applying migrations) happens outside this
repository.
