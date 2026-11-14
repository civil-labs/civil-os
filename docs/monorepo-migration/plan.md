# Plan: civil-os open-source Go monorepo

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `a0b6838`), which implements [`intent.md`](intent.md) (`475ff11`) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-13 |
| **Status** | Reviewed by the owner on 2026-11-14 (generated JS index required; `internal/envconfig` deviation approved). Accepted when committed. |

## Ground rules

- **Branches and PRs.** Work happens on short-lived branches and lands in eight PRs (PR 1–PR 8 below). The agent never pushes to `main`; the owner merges each PR.
- **Each step is one commit** with a conventional-commit message, and every commit from PR 2 onward builds (`go build ./...`).
- **Plan and code stay in sync.** If implementation departs from this plan, `plan.md` is updated in the same commit. If the departure changes the spec, the agent stops and asks the owner first.
- **Verification before "done".** Each PR's checks are run and their output pasted into the PR description before the PR is reported as done.
- **Source repos are read-only.** The agent works from fresh clones in the session scratchpad and never touches the working copies in `~/development/civil-*`.
- **Protected paths.** No edits to helm, gitops or jurisdiction repos. No jurisdiction names in any file, commit message or PR text.

## Environment prerequisites (owner, before PR 1)

| # | Need | Why | Status on this machine |
|---|---|---|---|
| E1 | Network access for the session to `github.com`, `pypi.org` / `files.pythonhosted.org`, `proxy.golang.org`, `sum.golang.org`, `buf.build`, `registry.npmjs.org`, Docker Hub and `ghcr.io` | Clones, git-filter-repo, Go modules, buf remote plugins, image pulls | Sandbox prompts per host |
| E2 | Docker daemon reachable from the session | testcontainers integration tests, compose, e2e and image builds | **Not reachable from inside the sandbox.** Either approve sandbox bypass for docker commands, or the owner runs the docker-based checks and pastes the output. |
| E3 | `git-filter-repo` | History import | Installed by the owner on 2026-11-14. |
| E4 | PR 1 merged with **"Create a merge commit"** (not squash), *before* squash-only branch protection is enabled | A squash merge would flatten the imported histories (spec §3) | Owner action at merge time |

## PR 1: import history (`chore/import-history`)

Every commit in this PR is a pure import with no content edits. The repo doesn't build in this PR, because there is still no root `go.mod`.

**1.1** Confirm `git filter-repo --version` runs. It is already installed (E3).

**1.2** For each repo, in this order:

| Repo (branch) | filter-repo arguments |
|---|---|
| civil-blueprints (`master`) | `--path proto/ --path buf.yaml --path buf.gen.yaml --path buf.lock --path-rename proto/:api/proto/ --path-rename buf.yaml:api/buf.yaml --path-rename buf.gen.yaml:api/buf.gen.yaml --path-rename buf.lock:api/buf.lock` |
| civil-database-schema (`main`) | `--path manifests/ --path Dockerfile --invert-paths`, then `--to-subdirectory-filter database` |
| civil-gateway (`master`) | `--to-subdirectory-filter services/gateway` |
| civil-database-reader (`main`) | `--to-subdirectory-filter services/database-reader` |
| civil-database-editor (`main`) | `--to-subdirectory-filter services/database-writer` |
| civil-tile-server (`main`) | `--to-subdirectory-filter services/tile-server` |

For each repo:
1. Run `git clone https://github.com/civil-labs/<repo>` into the scratchpad.
2. Apply the filter.
3. In civil-os, run `git fetch <clone> <branch>`, then `git merge --allow-unrelated-histories -m "chore: import <repo> history into <path>"`.

The imported `.github/`, `mise.toml`, `go.mod` and `.gitignore` files stay as-is in this PR and are removed in PR 2.

**Checks**
- For each prefix, `diff -r` of the prefix against the filtered clone's HEAD is empty.
- `git log --oneline -- services/gateway/auth.go | wc -l` is greater than 1, and the same for `services/database-reader/parcels.go`, `api/proto/mesh/civil/mesh/parcels/v1/parcels.proto` and `database/atlas.hcl`.
- `git ls-files | grep -E '(^|/)manifests/|database/Dockerfile'` returns nothing.

## PR 2: single module and build (`refactor/single-module`)

**2.0 Remove customer hostnames.** This step was added during PR 1, when a scan found two customer hostnames in the OpenAPI `servers` block of `api/proto/public/civil/public/parcels/v1/parcels.proto`. They are replaced with `http://localhost:8080`, the compose gateway. The imported history still contains them; the owner accepted that, since civil-blueprints is already public.

> **Commit order (revised during implementation).** Step 2.1 on its own would not build, because the editor's stubs don't compile against any API version. To keep every commit building, PR 2 lands as:
> - **A** = 2.1 plus 2.4's stub. The root module still depends on `civil-api-go`, unified to its newest pin.
> - **B** = 2.2.
> - **C** = 2.3, which drops `civil-api-go`.
> - **D** = 2.5 plus 2.4's binary and user rename in the writer dockerfile.
>
> Dependency unification only moved the writer: `civil-api-go` from April to August, `x/sync` 0.19 → 0.20 and `x/text` 0.33 → 0.37. `go mod tidy` changed no other pin.
>
> Also, `database/.gitattributes` moved to the root `.gitattributes`, with paths prefixed by `database/`.

**2.1 Root scaffolding**
- **Add** `go.mod` (`module github.com/civil-labs/civil-os`, `go 1.26.1`) and `mise.toml` with these pins: go 1.26.1, buf 1.66.1, node 22, atlas 1.3.0.
- **`mise.toml` tasks:** `generate`, `build`, `test`, `test:integration`, `test:e2e`, `lint`, `check`, `up`, `db:diff`, `db:validate`. The last two are ported from the schema repo's task.
- **Add to `.gitignore`:** `api/gen/openapi/`, `sdk/js/gen/`, `node_modules/`, and service binaries (`/services/*/gateway`, `/reader`, `/writer`).
- **Delete:** `services/*/go.mod`, `go.sum`, `mise.toml`, `.gitignore` and `.github/`; `database/mise.toml` and `database/.github/`; `api/.github/` and `api/mise.toml` if present.

**2.2 Code generation**
- `api/buf.yaml`: module paths `proto/public` and `proto/mesh`, with deps unchanged.
- Replace `api/buf.gen.yaml` with two templates:
  - `buf.gen.public.yaml`: Go and Connect → `api/gen/go`, prefix `github.com/civil-labs/civil-os/api/gen/go`; ES → `sdk/js/gen`; OpenAPI → `api/gen/openapi`.
  - `buf.gen.mesh.yaml`: Go and Connect → `internal/gen/go`, prefix `github.com/civil-labs/civil-os/internal/gen/go`.
- `mise run generate` runs `buf generate --template … --path proto/<x>` for each template. Commit `api/gen/go/**` and `internal/gen/go/**`.

**2.3 Rewrite imports**
- Rewrite `github.com/civil-labs/civil-api-go/civil/public/` to `github.com/civil-labs/civil-os/api/gen/go/civil/public/`, and `…/civil-api-go/civil/mesh/` to `github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/`, across `services/**/*.go`.
- Run `go mod tidy`. This is the dependency unification accepted as C5.

**2.4 Writer rename and temporary stub**
- In `services/database-writer/`, rename the binary to `writer`, the user to `writeruser`, and "reader" strings in logs to "writer". Delete the two obsolete stub RPCs.
- Temporarily embed `parcelsv1connect.UnimplementedParcelsServiceHandler` so the package builds. PR 3 replaces this.

**2.5 Dockerfiles**
- Rewrite `services/{gateway,database-reader,database-writer}/dockerfile`:
  - build context is the repo root: `COPY go.mod go.sum ./`, `go mod download`, `COPY . .`;
  - `go build -o /app/<bin> ./services/<svc>`;
  - runtime `alpine` pinned to the current stable 3.x tag;
  - user, port and `grpc_health_probe` unchanged.
- `services/tile-server/dockerfile`: only its `COPY` path changes, to `services/tile-server/config.yaml`. That way all four images share the repo root as their build context, so PR 7's image matrix needs no per-service context. *(Revised during implementation.)*
- Add a root `.dockerignore`: `.git`, `.github`, `.claude`, `docs`, `sdk`, `test`, `dev`, `database`, `api/proto`, `api/gen/openapi`, `**/node_modules`, `*.md`.
- Images are pinned to `golang:1.26.1-alpine3.23` (builder) and `alpine:3.24` (runtime). The runtime pin is to the minor version, so security patch releases are still picked up.

**Checks**
- `go build ./... && go vet ./...` passes.
- `grep -r civil-api-go --include=*.go .` finds nothing.
- `docker build -f services/<svc>/dockerfile .` succeeds for all 4 (needs E2).

## PR 3: mesh read/write split, writer, gateway changes (`feat!: split mesh parcels into read/write services`)

**3.1 Protos** (`api/proto/mesh/civil/mesh/*/v1/*.proto`)
- Rename the services: `ParcelReadService`, `ImprovementReadService`, `LandUseReadService`, `ZoningReadService`, `ValuationReadService`, `NeighborhoodReadService`, `SaleReadService`.
- Move `UpdateParcel` into a new `ParcelWriteService` in the same `parcels.proto`. Messages are unchanged.
- Run `buf lint`, then `mise run generate`.

**3.2 Reader** (`services/database-reader/main.go`): register the `*ReadService` handlers and put the new names in the health checker. No SQL changes.

**3.3 Writer** (`services/database-writer/parcels.go`, `main.go`)
- Remove the stub and serve `ParcelWriteService` plus health.
- `UpdateParcel` follows spec §5.3, in one `pgx` transaction:
  1. `SELECT parcel_id FROM core.parcels WHERE public_id=$1 AND NOT is_voided FOR UPDATE`; if nothing comes back, return `NotFound`.
  2. Resolve `address_id` and `land_use_id` via their `public_id`s; a missing one returns `InvalidArgument` naming the field.
  3. Convert ft to m and sq ft to sq m.
  4. Check `properties` parses as a JSON object.
  5. Run a dynamic `UPDATE core.parcel_attributes SET … , trace_id=$n WHERE parcel_id=$1 AND legal_valid_range @> now()`, setting only the fields present. Zero rows returns `FailedPrecondition`.
- Map Postgres check violation `23514` on `properties_size_limit` to `InvalidArgument`.
- *(Clarified during implementation.)* `land_area_sq_ft`, `frontage_ft` and `depth_ft` must be finite and non-negative, else `InvalidArgument`. The mesh request has no protovalidate rules, so the writer checks for itself.
- `trace_id` comes from parsing the `traceparent` header, else 16 random bytes as hex.

**3.4 Gateway**
- Rename the clients to the read services.
- Add the optional writer client and route `ParcelServer.UpdateParcel` through it. With no writer host, return `Unavailable`.
- Forward `traceparent` by copying the header onto the mesh request in `UpdateParcel`. *(Revised during implementation: it is the only forwarded call, so an interceptor isn't needed.)* The writer host is read from `CIVIL_DB_WRITER_HOST` until PR 4 renames it to `CIVIL_OS_GATEWAY_WRITER_HOST`.
- Dex (spec §5.1): with no address, don't register the handler and log one `INFO` line; if `grpc.NewClient` fails, exit non-zero.
- Have servers depend on the generated Connect client interfaces.

**3.5 AWS removal:** delete `services/gateway/backend.go` and run `go mod tidy`.

**Checks**
- `go build ./... && go vet ./...` passes.
- `git grep aws-sdk-go-v2 -- services` finds nothing, and `servicediscovery` is gone from `go.mod`. The AWS SDK remains only through gocloud `s3blob` (spec F6, amended 2026-11-14).
- `buf lint` passes.

## PR 4: env var contract and log level (`feat!: CIVIL_OS_ env var contract`)

**4.1** Add a new package, `internal/envconfig`, with the helpers used by all three services:
- `LogLevel(service) (slog.Level, error)`: per-service variable, then shared, then `INFO`; parsed with `UnmarshalText`;
- `RejectLegacy() error`: finds any set `CIVIL_*` that isn't `CIVIL_OS_*`;
- `Port(key, default)`;
- `Required(keys…)`.

> **Deviation from spec §2:** the spec allows only generated code and test helpers under `internal/`. A shared package avoids three copies of the legacy-variable and log-level logic. **The owner approved this deviation on 2026-11-14.** The same commit amends spec §2 to list `internal/envconfig`.

**4.2** Rewrite `services/*/config.go` to the names in spec §5.5. Reader and writer default ports become `50051` and `50052`. Delete `getVerboseEnv`, and use a `slog.LevelVar` in every service.

**4.3** In `services/tile-server/config.yaml`, change `connection_string` to `${CIVIL_OS_TILE_SERVER_DATABASE_URL}`.

**4.4** *(Added by owner request after PR 3.)* Gateway auth fails fast. `main` handles `RequireAuth`'s error and exits, and `RequireAuth` returns an error for an empty auth server, IdP host or client ID list. As in spec §5.5, an invalid port or `ALLOWED_CLIENT_IDS` also stops startup.

> Note for PR 6, **resolved 2026-11-14 (owner chose option 1) and implemented as PR 5 step 5.0**: `CIVIL_OS_GATEWAY_AUTH_SERVER` may carry its scheme (`http://dex:5556`); a bare host keeps the `https://` default. Doing it in PR 5 lets the auth tests use a plain-http fake issuer.

**Checks:** `go build ./... && go vet ./...` passes, and `grep -rn '"CIVIL_[A-Z]' services | grep -v CIVIL_OS_` finds nothing.

## PR 5: tests (`test: unit, handler, integration and migration suites`)

**5.1 Testability refactor:** each service's `main()` becomes `run(ctx, cfg, logger) error` plus a thin `main`, with no behavior change.

**5.2 Synthetic fixtures** (`database/fixtures/synthetic.sql`)
- **Contents:** a fictional country/administrative area, land use types and land uses, zoning with attributes, one neighborhood definition and neighborhoods, 20 parcels, improvements, addresses and one valuation.
- **Parcels:** each has a fixed UUID, geometry, attributes and history-compatible ranges. Parcel geometries sit in an empty ocean area so they can't resemble a real place.
- **Format:** every row uses fixed UUIDs that tests import as constants from `internal/testutil/fixtures.go`.

**5.3 Shared helpers** (`internal/testutil/`)
- `postgres.go`: testcontainers PostGIS, pinned to the same image as compose. Applies `database/migrations/*.sql` in filename order and loads the fixtures.
- `oidc.go`: a fake issuer, meaning an RSA key, a JWKS endpoint, discovery and a token minting helper.

Integration tests use build tag `integration` and run through `mise run test:integration`.

**5.4 Tests**

| File | Covers (spec §8) |
|---|---|
| `internal/envconfig/envconfig_test.go` | Levels in any case and with offsets, invalid level, per-service override, legacy rejection, required and default handling, all three services' configs from one shared environment |
| `services/gateway/mappers_test.go` | Every `map*` function: nil, empty and partial input |
| `services/gateway/cors_test.go`, `auth_test.go` | CORS preflight and headers; auth valid / wrong client / expired / bad signature / missing |
| `services/gateway/handlers_test.go` | `httptest` fake mesh servers for every public RPC: mapping, error passthrough, protovalidate rejection; `UpdateParcel` with and without a writer; Dex unset → `Unimplemented`, unreachable → `Unavailable`, fake `DexClient` RPCs |
| `services/database-writer/parcels_unit_test.go` | Unit conversion, `properties` validation, `traceparent` parsing |
| `services/database-reader/integration_test.go` | Every read RPC by parcel and by feature, plus not-found cases |
| `services/database-writer/integration_test.go` | Full and partial update, unit round trip through the reader's SQL, `properties` replace, unknown parcel / address / land use, voided parcel, oversized `properties`, history row with `trace_id` |
| `database/migrations_test.go` | All migrations apply to an empty DB; `atlas.sum` file list matches the directory |

**Checks:**
- `go test ./...` passes.
- `go test -tags integration ./...` passes (needs E2).
- Coverage report attached to the PR.

**Clarified during implementation (2026-11-14):**
- **search_path.** The reader uses unqualified table names and relies on the database-level `search_path = core, geo, extensions, public` that cluster infrastructure sets. The fixture file and `internal/testutil` set the same path, so tests match production. Compose (PR 6) must set it too.
- **Reader bug fixed.** `GetValuations`, `GetNeighborhoodDefinitions` and `GetNeighborhoods` read from `public.*`, but the tables live in `core`, so they failed in every deployment. They are now qualified as `core.*`.
- **Gateway error codes.** Fourteen handlers wrapped every mesh error as `Internal`. To meet spec §8 (mesh error-code propagation), they now return the mesh error unchanged, as the per-attribute RPCs and `UpdateParcel` already did.
- **`atlas.sum` check is stronger than planned.** `database/migrations_test.go` recomputes Atlas's per-file and total hashes, so an edited migration fails CI without an Atlas licence.
- **Test database per test.** One PostGIS container per test binary. Migrations and fixtures load once into a template database, and each test gets its own `CREATE DATABASE … TEMPLATE` clone.
- **Voided parcels (owner decision, 2026-11-14):** every current read hides voided parcels. Separate APIs for them will come later. The attribute RPCs and summaries were changed to match the comparables, tiles and writer. Every by-ID and by-feature reader test asks for the voided parcel and requires it to be absent.
- **Comparables IDs (owner decision, 2026-11-14):** each item of `selected_parcel_ids` on `GetEquityComparables` and `GetSalesComparables` must be a UUID.
- **Deferred to their own features:** the timeout on the startup JWKS fetch (`docs/idp-startup-resilience`), Dex address validation (`docs/dex-address-validation`) and `traceparent` in the CORS allow-list (`docs/browser-trace-propagation`).

## PR 6: local stack and e2e (`feat: docker compose quickstart`)

**6.1 Files**
- `compose.yaml` with services `db`, `migrate`, `seed`, `dex`, `database-reader`, `database-writer`, `tile-server` and `gateway`:
  - health checks on each service;
  - `depends_on` with `service_completed_successfully` or `service_healthy`;
  - the gateway on `localhost:8080`;
  - all env vars use the `CIVIL_OS_*` names.
- `dev/dex.yaml`: static client `civil-os-dev`, static user `dev@example.com` with a documented throwaway password, gRPC API on `:5557`.
- `dev/instance-metadata.json`: synthetic metadata.

**6.2 E2E tests** (`test/e2e/smoke_test.go`, tag `e2e`)
1. Wait for health checks.
2. Get a token from Dex with the password grant.
3. Read a fixture parcel, update its `frontage_ft`, then read it back to confirm the change.
4. Fetch a tile covering the fixtures and expect a non-empty MVT.

**Checks:** from a fresh `git clone` into the scratchpad, `docker compose up --wait && go test -tags e2e ./test/e2e/...` passes (needs E2).

**Clarified during implementation (2026-11-14):**
- **Issuer split.** Dex's issuer is the host-facing `http://localhost:5556/dex`, so tokens work from the host and a browser. The gateway fetches keys over the compose network (`CIVIL_OS_GATEWAY_IDP_HOST=dex:5556/dex`).
- **`migrate` reruns safely.** It creates `civil` from `template0` (the PostGIS image pre-installs PostGIS in `template1`) and sets the production `search_path`. It records applied files in `civil_os_dev.applied_migrations` (local only) and runs each file in one transaction. `seed` skips when parcels exist. A second `docker compose up` is a no-op.
- **E2E covers more than planned:** an unauthenticated call is rejected, an unknown parcel update gives `NotFound`, plus instance metadata and the Dex admin API (`ListClients`). Each run writes a new frontage value, so reruns pass.
- **Finding, not changed:** `services/tile-server/config.yaml` auto-publishes every geometry table, so Martin also serves raw `parcel_geometry`, the `*_history` tables and address coordinates. That exposes voided parcels and history through `/tiles/`. It is the production config too, so it is deferred to `docs/tile-source-allowlist` (owner decision, 2026-11-14).

## PR 7: CI, release and publishing (`ci: workflows, release-please, npm SDK`)

**7.1 SDK package** (`sdk/js/`)
- `package.json`: `@civil-labs/civil-os-api`, version `0.1.0`, the same exports and deps as civil-api-js, plus `publishConfig.access: public` and `repository` pointing at civil-os.
- **Generated index (owner requirement).** `index.js` and `index.d.ts` are **never hand-written or committed**.
  - `sdk/js/scripts/gen-index.mjs` walks `sdk/js/gen/**` and writes one `export *` line per generated `*_pb.js` / `*_pb.d.ts`, sorted for stable output.
  - It runs as part of `mise run generate` and as the npm `prepack` script, so `npm pack` and `npm publish` always rebuild it.
  - `sdk/js/index.js` and `sdk/js/index.d.ts` are added to `.gitignore`.
  - A new public proto is exported automatically, with no manual step.
- `README.md`.

**7.2 Release-please files**
- `release-please-config.json`:
  - package `.`, type `go`, with `include-component-in-tag: false`;
  - package `api/proto/public`, type `simple`, with `component: public-api` and an `extra-files` json updater on `sdk/js/package.json` `$.version`;
  - `bump-minor-pre-major` and `bump-patch-for-minor-pre-major` set to true.
- `.release-please-manifest.json`: `{".": "0.2.0", "api/proto/public": "0.1.0"}`.

**7.3 Workflows** (spec §6.3)
- `ci.yml`:
  - `mise install go buf node` (no Atlas);
  - `go build`, `vet` and `test`;
  - `test -tags integration` (Docker is available on GitHub-hosted runners);
  - `buf lint`, `buf format --diff --exit-code`;
  - generated-code freshness: `mise run generate && git diff --exit-code api/gen/go internal/gen/go`;
  - `buf breaking api/proto/public --against '.git#tag=<last public-api tag>'`. This is skipped while no tag exists, and allowed to fail when the PR title has `!`;
  - an e2e job running `docker compose up --wait` and the e2e tests;
  - the no-customer-data check: `scripts/check-denylist.sh` reads newline-separated patterns from the `CUSTOMER_DENYLIST` secret and greps the tree. It skips with a warning when the secret is absent, as on forks.
- `pr-title.yml`: `amannn/action-semantic-pull-request` pinned to a version.
- `release-please.yml`: `googleapis/release-please-action`, pinned, with the App token from `RELEASE_PLEASE_APP_ID` and `RELEASE_PLEASE_PRIVATE_KEY`.
- `images.yml`:
  - a matrix over the four services, built from the repo root context;
  - `:sha-<short>` on `main`, `:vX.Y.Z` on `v*` tags;
  - attestations kept from the old workflows.
- `migrations-release.yml`: on `v*`, runs `kubectl create configmap civil-os-db-migrations-vX-Y-Z --from-file=database/migrations --dry-run=client -o yaml` and uploads the result to the release.
- `public-api-release.yml`: on `public-api-v*`, runs `mise run generate`, `node sdk/js/scripts/gen-index.mjs` and `npm publish --provenance` (`id-token: write`), and uploads `openapi.yaml` to the release.

**Checks**
- `actionlint` passes on all workflows.
- Run release-please in dry-run mode locally: `npx release-please release-pr --dry-run …`. It must propose both streams (spec acceptance 6).
- `npm pack --dry-run` in `sdk/js` lists the generated files and index.
- The generated `index.js` has one export per public proto file, 10 today, compared against `find api/proto/public -name '*.proto' | wc -l`.
- A Node smoke import (`node -e "import('@civil-labs/civil-os-api')"` against the packed tarball) succeeds.

**Clarified during implementation (2026-11-14):**
- **SDK imports need `.js` extensions.** `api/buf.gen.public.yaml` sets `import_extension=js` on the `es` plugin; without it the generated files import each other extensionlessly and Node ESM can't load the package. The SDK license is MIT, matching the repo `LICENSE`.
- **The index exports only `gen/civil/public/**`.** Vendored dependency code (protovalidate, googleapis, gnostic) is still packed, because the public files import it, but it isn't re-exported.
- **Manifest starts at `0.0.0`, with `initial-version`.** release-please treats the manifest value as the *last released* version, so `{".": "0.2.0"}` would make the first release `v0.3.0` (or `v1.0.0` without the pre-major flags) given the `feat!` commits. Instead the manifest is `0.0.0` for both packages, and the config sets `initial-version` to `0.2.0` (app) and `0.1.0` (public API), with `bootstrap-sha` at the PR 1 merge so old history is ignored. This was read from the release-please v17 source, not run: the `--dry-run` check needs a GitHub token, which wasn't used outside the sandbox. The first real release PR is the check.
- **`extra-files` path starts with `/`.** Without the leading slash release-please resolves it under the package path (`api/proto/public/sdk/js/package.json`), and `../` is rejected. `/sdk/js/package.json` is repo-root-relative.
- **`buf breaking` takes the public module as its input**, not `--path`: `buf breaking proto/public --against "../.git#tag=<tag>,subdir=api/proto/public"`, run from `api/`. Checked against a temporary local tag: a clean tree passes, and renaming a public message fails with exit 100.
- **Workflow actions are pinned to commit SHAs**, with the version in a comment. The images use the GitHub Actions build cache and build `linux/amd64` only, as the old workflows did.
- **`public-api-release.yml` installs npm 11**, which npm trusted publishing needs. It also passes `NPM_TOKEN` if set, so either publishing method works. A step checks that `sdk/js/package.json` matches the tag before publishing.
- **Release assets wait for the release.** The tag can arrive just before release-please creates the GitHub Release, so the upload steps retry `gh release view` for up to a minute.
- **The denylist check prints file names only**, never the matching line, so CI logs don't repeat customer terms.

## PR 8: docs and agent guardrails (`docs: README, architecture, CLAUDE.md, setup`)

**8.1 Documents**
- `README.md`: what civil-os is, the quickstart, the curl walkthrough (token, read, update, tile), releases and the license.
- `docs/architecture.md`:
  - the service map and public vs mesh APIs;
  - the read/write split;
  - the bitemporal model in brief;
  - the §5.5 env var table with old-to-new mapping;
  - the load-balancing note and the release streams.
- `docs/github-setup.md`: the intent's manual steps as an exact checklist. It lists the secret names (`RELEASE_PLEASE_APP_ID`, `RELEASE_PLEASE_PRIVATE_KEY`, `CUSTOMER_DENYLIST`), npm org and trusted publishing, branch protection, GHCR visibility, and the E4 merge-commit note.

**8.2 `CLAUDE.md`** (one page or less): the commands, the layout, these rules, and known mistakes.
- **Commands:** `mise run …`.
- **Rules:**
  - never edit `*/gen/`;
  - conventional commits;
  - Read/Write mesh naming;
  - no customer data;
  - run `mise run db:validate` locally after any migration change.

**8.3 Hook:** `.claude/settings.json` gets a `PreToolUse` hook (`.claude/hooks/block-gen-edits.sh`) that blocks Edit and Write under `api/gen/` and `internal/gen/`.

**Checks:**
- Follow every README command verbatim from a fresh clone.
- The hook blocks a test edit to `api/gen/go/...`.

## Final verification (before reporting the effort done)

1. Run on the tip of the last PR:
   - `mise run check` (build, vet, unit, lint, format, generated-code freshness, breaking);
   - `go test -tags integration ./...`;
   - the e2e suite;
   - `docker build` for all four images.

   Paste the output.
2. A fresh-context verifier subagent reviews the cumulative diff against `spec.md` §1 and §12 and reports any unmet requirement.
3. Owner checks after merge: CI is green on `main`, the `:sha-` images appear under `ghcr.io/civil-labs/civil-os/*`, and the first release-please PR shows `v0.2.0` and `public-api-v0.1.0`.

## Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | A squash merge of PR 1 erases the imported history. | E4: the owner merges PR 1 with a merge commit before enabling squash-only. |
| R2 | Docker is unavailable inside the agent sandbox, so integration, e2e and image checks can't run. | E2: bypass approval per command, or the owner runs and pastes. The PR isn't reported done without that output. |
| R3 | Synthetic fixtures must satisfy a large bitemporal schema (≈70 tables, exclusion constraints, history triggers) and Martin's `get_parcel_tiles`. | Build fixtures against the migrated DB in PR 5 first and iterate until migrations plus seed load cleanly before writing any assertions. Only populate the tables the reader's queries touch. |
| R4 | release-please `extra-files` may not accept a path outside the package directory (`api/proto/public` → `sdk/js/package.json`). | Verify in the PR 7 dry run. Fallback: make `sdk/js` the Public API package path and add `api/proto/public/**` changes to it via `additional-paths` (supported in recent release-please). Record whichever is used in this plan. |
| R5 | `buf generate` uses remote plugins from buf.build, which has rate limits and needs network access. | E1. If rate limited, retry; pin plugin versions in the templates. |
| R6 | The dependency bump changes reader or writer behavior. | Integration tests in PR 5 cover every reader RPC. The bump lands in PR 2 but is only proven by PR 5, so PRs 2–4 are not deployed in between. |
| R7 | Mesh renames break any caller outside this repo. | None exist: the only mesh callers are the gateway and the old repos, which keep their old images until the helm follow-up. |
| R8 | The `atlas.sum` hash isn't checked in CI (C6). | Local `mise run db:validate`, written into `CLAUDE.md`. CI still checks the file list and that migrations apply. |

## Out of scope for this plan (per intent)

Helm, gitops and jurisdiction repo changes; archiving the source repos; switching civil-lvt-shift to npm; GitHub, npm and GHCR settings (owner, `docs/github-setup.md`); OPA and audit logging; non-JS SDKs; Renovate or Dependabot.
