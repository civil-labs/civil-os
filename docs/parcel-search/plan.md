# Plan: search parcels by address

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `fe88158`, #61), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-19 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The work lands in two PRs (below), each on its own
  short-lived branch. The agent never pushes to `main`. The owner merges each
  PR.
- **Each step is one commit** with a conventional-commit message, and every
  commit builds (`go build ./...`).
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Verification before "done".** Each PR's checks are run locally, and their
  output goes in the PR description before the PR is reported as done.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated code is committed with the `.proto`
  change.
- **Synthetic data only**, including the performance dataset's word lists.
- **Every new query filters voided rows** (`NOT x.is_voided`) and resolves
  every attributes, link and geometry row with `legal_valid_range @> t`.

## Environment

- **Docker is reachable from the agent's session** (server 29.5.3 on
  2026-11-19, after the owner added the user to the `docker` group and
  started the daemon). So `mise run test:integration`, `mise run test:e2e`
  and the performance check run locally.
- **Atlas isn't logged in** in the agent's session (`atlas whoami` needs
  `atlas login`), and `mise run db:diff` needs Atlas Pro. **The owner
  generates the migration** (step 1.2) and runs `mise run db:validate`. The
  agent writes the HCL and everything around it.

## PR overview

| PR | Branch | Title | Depends on | Release effect |
|---|---|---|---|---|
| 1 | `feat/address-search-index` | `feat: trigram index for address search` | — | Go module patch (`bump-patch-for-minor-pre-major`) |
| 2 | `feat/search-parcels-addresses` | `feat: search parcels by address` | PR 1 | public-api patch and Go module patch |

PR 1 is the schema change alone. That keeps the owner's Atlas step in a small
PR, and lets the migration run in CI and compose before any code depends on
it. It changes nothing a client can see. If the owner doesn't want a release
for it on its own, the release-please PR is held until PR 2 merges. PR 2 is
the vertical slice: mesh proto, reader, public proto, gateway, tests, docs and
the performance check. It's additive, so `buf breaking` passes without `!`.

## Decisions the spec left to the plan

| Question | Decision |
|---|---|
| `CREATE INDEX CONCURRENTLY` | **No: a plain `CREATE INDEX`.** `CONCURRENTLY` can't run in a transaction. Compose's `dev/migrate.sh` applies each file with `psql --single-transaction`, and `testutil.ExecFiles` sends a file as one multi-statement simple query, which Postgres runs as one implicit transaction. Both would fail, and changing them for one index isn't worth it. A plain build takes a `SHARE` lock on `address_attributes`, which blocks writes to that table but not reads, for the build's duration. The performance step (2.9) times the build on 500,000 rows and records it in PR 2, so the owner can schedule the production migration. |
| Where the query is normalised | **In SQL**, by calling `core.search_normalize($1)` at the start of the reader's transaction. The Go side never reimplements it, so the length check, the filter hash and the indexes all see the same string. That costs one round trip, in the same transaction. |
| Setting the threshold | `SELECT set_config('pg_trgm.word_similarity_threshold', $1, true)` in the reader's read-only transaction (`pool.BeginTx` with `AccessMode: pgx.ReadOnly`), which is the `SET LOCAL` the spec asks for. The reader's pool uses `QueryExecModeExec`, which runs it as is. |
| The function's `search_path` | **Qualified names instead of `SET search_path`**, if Atlas's `function` block can't express a `SET` clause. The body calls `pg_catalog.lower`, `pg_catalog.btrim` and `pg_catalog.regexp_replace`, which gives the same protection. Step 1.1 checks this. |
| The snapshot time | When `legal_as_of` is unset, the reader takes `time.Now()`, truncates it to the microsecond (Postgres's precision), and uses that for the query and for `as_of`. Truncating keeps the cursor's `as_of` equal to what the database compared. |
| The test-only cap | The cap is a field on the reader's `APIServer` (`searchCap`), which `newHandler` sets to 1,000. The cap test builds the handler with an unexported option that sets 10. Package-level variables aren't used, because tests run the reader in-process and in parallel. |
| Where the performance check lives | **A Go test behind a `perf` build tag** in `services/database-reader` (`search_perf_test.go`), not a compose tool. The reader's port isn't published by compose, and the test can reuse `testutil`'s container and the in-process reader, as the integration tests do. Its dataset is a SQL file, `dev/perf/search-dataset.sql`. `mise run perf:search` runs it. |

## PR 1: schema (`feat/address-search-index`)

**1.1 Schema** (`feat: trigram index for address search`)

In `database/schema/schema.pg.hcl`:
- `extension "pg_trgm"` with `schema = schema.extensions`, next to the other
  three, with a comment pointing at the spec.
- A new section under "Functions":

  ```hcl
  function "search_normalize" {
    schema     = schema.core
    lang       = "sql"
    arg "input" { type = text }
    return     = text
    volatility = IMMUTABLE
    parallel   = SAFE
    strict     = true
    as         = "SELECT pg_catalog.lower(pg_catalog.btrim(pg_catalog.regexp_replace(input, '[^[:alnum:]]+', ' ', 'g')))"
  }
  ```

  The exact attribute spelling (`lang = "sql"` and the `as` form) follows what
  `get_parcel_tiles` uses and what `atlas schema inspect` emits. If Atlas
  can't express a function-level `SET search_path`, use the qualified names
  above (see the decisions table).
- On `table "address_attributes"`:

  ```hcl
  index "idx_address_attributes_formatted_address_trgm" {
    type = GIN
    on {
      expr = "core.search_normalize(formatted_address)"
      ops  = sql("extensions.gin_trgm_ops")
    }
  }
  ```

  No index in the schema uses an expression or an operator class yet, so the
  owner checks the syntax in step 1.2. If Atlas rejects `ops = sql(...)` for a
  schema-qualified operator class, the fallback is
  `ops = "extensions.gin_trgm_ops"`.

**1.2 Migration** (owner: `mise run db:diff`, then `mise run db:validate`)

The owner generates the migrations on the branch, and they are committed with
`atlas.sum`. There are **two migrations, generated in two passes**. Atlas
can't see the function call inside the index's `expr` string, and it plans
table changes before new functions. In one pass, it put `CREATE INDEX` before
`CREATE FUNCTION`, and `db:validate` failed with
`function core.search_normalize(text) does not exist`. Atlas documents
`depends_on` only for views and materialized views. After PR 1 merged, we
found that a table-level `depends_on = [function.search_normalize]` would
probably have fixed the order too (ariga/atlas#2572). So:
1. The HCL without the index gives `CREATE EXTENSION`, then `CREATE FUNCTION`.
2. The index added back gives `CREATE INDEX` alone, in a later migration.

`db:validate` then runs once over both. The agent reviews the SQL:
- The statements are in that order across the two files.
- There's no `CONCURRENTLY` (see the decisions table).
- There's nothing unrelated: Atlas can pick up drift.

For later changes like this, the `atlas-migrations` skill says to try
`depends_on` first.

`scripts/check-migrations-size.sh database/migrations` stays well under its
768 KiB warning.

**1.3 Tests** (`test: migration and normalisation tests for address search`)

In `database/migrations_integration_test.go`:
- `TestMigrationsApplyToEmptyDatabase` (or a new test on the same database)
  checks that `pg_trgm` is installed in `extensions`
  (`pg_extension` joined to `pg_namespace`). It also checks that
  `idx_address_attributes_formatted_address_trgm` exists, is GIN and uses
  `gin_trgm_ops` (`pg_indexes.indexdef`).
- New `TestSearchNormalize`, a table test of `core.search_normalize`:

  | Input | Output |
  |---|---|
  | `12 Fixture Way` | `12 fixture way` |
  | `12, FIXTURE-way` | `12 fixture way` |
  | `  --12   Fixture__Way!! ` | `12 fixture way` |
  | `#4B` | `4b` |
  | `--` | `` (empty) |
  | `NULL` | `NULL` (strict) |

  Plus one check that a query on `core.search_normalize(formatted_address)
  LIKE '%fixture%'` uses the index. Run `SET enable_seqscan = off` and look
  for the index name in `EXPLAIN`, because 21 rows would otherwise always
  seqscan.

**1.4 Docs** (in 1.1's commit):
- `docs/architecture.md`, next to the migrations paragraph: the database
  needs `postgis`, `btree_gist`, `pgcrypto` and `pg_trgm`, all in the
  `extensions` schema.

**Checks:**
- `mise run check`.
- `mise run db:validate` (owner).
- `mise run test:integration`.
- `mise run up` from an empty volume (`mise run down` first), which applies
  the migration through `dev/migrate.sh`.
- `mise run test:e2e`.

## PR 2: the RPC (`feat/search-parcels-addresses`)

**2.1 Mesh protos and reader stub** (one commit with 2.3, `feat: search parcels by address in the database reader`,
because a new RPC on an existing service doesn't build until the reader
implements it):
- `civil.mesh.pagination.v1`: add `SearchCursor` as the spec gives it, with
  `google.protobuf.Timestamp as_of`.
- `civil.mesh.parcels.v1`: on `ParcelReadService`, add `SearchParcelsAddresses`
  with `NO_SIDE_EFFECTS`, `SearchParcelsAddressesRequest`, `...Response`
  (`repeated ParcelSearchHit hits`, `PageInfo page_info`) and
  `ParcelSearchHit`. No validation or gnostic annotations.
- `mise run generate`.

**2.2 Search cursor** in `services/database-reader/search_cursor.go`, with
unit tests in `search_cursor_test.go`:
- `searchFilterHash(normalizedQuery string, legalAsOf *timestamppb.Timestamp) []byte`
  uses `newFilterHasher`, written as `"SearchParcelsAddresses"`, the query,
  then `"unset"`, or `"set"` plus RFC 3339 nanoseconds, as `idFilterHash` does.
- `searchKey{tier int; similarity float32; parcelID string; asOf time.Time}`.
- `encodeSearchCursor(key, hash) (string, error)`.
- `decodeSearchCursor(s string, hash []byte, legalAsOf *timestamppb.Timestamp) (searchKey, error)`
  applies every spec check, in order, each failure returning
  `connect.NewError(CodeInvalidArgument, errInvalidCursor)`:
  1. base64url and protobuf decode;
  2. `version == 1`;
  3. `filter_hash` equal (`bytes.Equal`);
  4. `tier <= 3` (`uint32`, so never negative);
  5. `similarity`: not `math.IsNaN`, not `math.IsInf`, and `0 <= s <= 1`;
  6. `last_parcel_id`: `uuid.Parse`;
  7. `as_of`: present, `CheckValid()`, and after `-62135596800` seconds, the
     bound the public `legal_as_of` uses (a shared constant);
  8. when `legalAsOf` is set, `as_of` is equal to it (`AsTime().Equal`).
- **Unit tests:**
  - round trip;
  - each check above failing on its own, with a correctly recomputed hash
    (the spec's forged-cursor row);
  - a forged cursor with valid values accepted;
  - a cursor for another normalised query rejected.

**2.3 Reader query** in `services/database-reader/parcel_search.go`, on
`APIServer`:

1. **Begin** a read-only transaction. Then:
   - `SELECT core.search_normalize($1)`;
   - if the result has fewer than 3 runes (`utf8.RuneCountInString`, after
     removing spaces), return
     `InvalidArgument("query must contain at least 3 letters or digits")`;
   - `set_config` the threshold (`searchSimilarityThreshold`, a constant,
     `"0.6"`, confirmed in step 2.5).
2. **Resolve `t`:** `legal_as_of` if set, otherwise the truncated now (see the
   decisions table). Decode `after` if present.
3. **Query** (sketch; `$q` is the normalised query, already free of `%`, `_`
   and `\`, which `escapeLike` still escapes as the spec requires):

   ```sql
   WITH cand AS (
     SELECT p.public_id AS parcel_id, pg.feature_id,
            a.public_id AS address_id, aa.formatted_address,
            core.search_normalize(aa.formatted_address) AS t
     FROM address_attributes aa
     JOIN addresses a ON a.address_id = aa.address_id AND NOT a.is_voided
     JOIN parcel_attributes pa ON pa.address_id = a.address_id AND pa.legal_valid_range @> $1
     JOIN parcels p ON p.parcel_id = pa.parcel_id AND NOT p.is_voided
     JOIN parcel_geometry pg ON pg.parcel_id = p.parcel_id AND pg.legal_valid_range @> $1
     WHERE aa.legal_valid_range @> $1
       AND ($2 <% core.search_normalize(aa.formatted_address)
            OR core.search_normalize(aa.formatted_address) LIKE '%' || $3 || '%')
   ), ranked AS (
     SELECT *,
       CASE WHEN t LIKE $3 || '%'             THEN 0
            WHEN ' ' || t LIKE '% ' || $3 || '%' THEN 1
            WHEN t LIKE '%' || $3 || '%'       THEN 2
            ELSE 3 END AS tier,
       word_similarity($2, t) AS sim
     FROM cand
     ORDER BY tier, sim DESC, parcel_id
     LIMIT $4                                  -- the cap
   )
   SELECT parcel_id, feature_id, address_id, formatted_address, tier, sim
   FROM ranked
   WHERE $5::int IS NULL                       -- no cursor
      OR (tier, -sim, parcel_id) > ($5, -$6::real, $7::uuid)
   ORDER BY tier, sim DESC, parcel_id
   LIMIT $8;                                   -- first + 1
   ```

   The parcel join uses the existing `visibleParcelJoinsSQL` from
   `parties.go` if it fits the aliases, so rule 1 is written once.
   `feature_id` is joined in the candidates, not after the page as the spec
   sketched. It's needed for rule 1 anyway, and joining it again would cost
   more.
4. **Page:** `hasNextPage = rows > size`. Encode cursors only for the first and
   last hit into `start_cursor` and `end_cursor`, and set
   `has_previous_page = after != nil`. Commit, so the transaction is released.
5. **Register** nothing new: `ParcelReadService` is already served. Log the
   request (the query's length, not its text) and the number of hits, like
   the other parcel reads.

**2.4 Reader integration tests** in
`services/database-reader/parcel_search_integration_test.go`, covering the
spec's rows with `testutil` helpers. A new `testutil.ParcelSearchHit`
assertion helper isn't needed. Compare parcel IDs in order.
- **Matching:**
  - `2 fixture` begins with parcels 2 and 12, and parcel 2's hit carries
    `AddressID(2)` and `2 Fixture Way, Testville ZZ000`;
  - `2, FIXTURE-way` returns the same IDs in the same order;
  - `fxture way` returns hits, all at tier 3 (step 2.5);
  - `fixture way` returns parcels 1–19 in ID order.
- **Visibility:**
  - parcels 20 and 21 never appear, whatever the query;
  - `5 Old Fixture` now returns no hit carrying `5 Old Fixture Way`, and
    parcel 5 at most at tier 3 with its current line (step 2.5). At
    `testutil.HistoryChange - 1 day`, parcel 5 is first with the old line;
  - an address voided in the test (`startReaderOn` with a committed update)
    drops its parcel.
- **Pagination:**
  - the `first = 5` walk over `fixture way` equals a single `first = 100`
    page;
  - `start_cursor` as `after` returns that page minus its first hit;
  - an empty page has neither cursor;
  - cursor reuse across queries fails, and across `12 fixture way` and
    `12, FIXTURE-way` works;
  - malformed and unknown-version cursors fail;
  - `first = 500` is clamped.
- **The snapshot:** with `legal_as_of` unset, read page 1 of `fixture way`
  (`first = 5`). Then, in the test, reword address 7 the way a new legal fact
  would be recorded: close its current row's range at a time after page 1's
  `as_of`, and insert a new row from then reading `7 Renamed Road, ...`. An
  in-place `UPDATE` wouldn't do: it changes the row the snapshot still covers,
  which the spec's known limits accept. Page 2 still contains parcel 7.
- **The cap:** with the handler's cap set to 10, `fixture way` with `first = 4`
  gives 4, 4 and 2 hits, and the last page has `has_next_page = false`.
- **Minimum query:** `--`, `a.b` and `ab` fail with `InvalidArgument`, and
  `abc` doesn't.

**2.5 Threshold check** (`test: tune the address search threshold`)

Before the public proto, a throwaway query over the fixtures lists
`word_similarity` for the spec's examples (`fixtrue way`, `2 fixture`).
If 0.6 doesn't give the spec's results, the constant changes, and this plan
and the spec's "Matching and ranking" are updated in this commit. The owner
is told, because the spec names 0.6.

**Done on 2026-11-19, on the compose fixtures.** Best `word_similarity`
against any current address, and the number of addresses (of 21) at or
above each threshold:

| Query | Best | ≥ 0.6 | ≥ 0.5 |
|---|---|---|---|
| `fixtrue way` | 0.500 | 0 | 21 |
| `fxture way` | 0.727 | 21 | 21 |
| `fixture wya` | 0.750 | 21 | 21 |
| `old way` | 0.500 | 0 | 21 |
| `maple way` | 0.400 | 0 | 0 |
| `5 Old Fixture` | 0.714 | 1 | 21 |
| `2 fixture` | 1.000 | 21 | 21 |

The owner kept 0.6 (2026-11-19). The spec's typo example became
`fxture way`, because the `fixtrue` swap breaks four of the word's eight
trigrams. Its history example was reworded: `5 Old Fixture` matches address
5's current line at tier 3, so it can't return nothing now, but it never
returns the old line. Both changes are in the spec's "Matching and ranking",
"Fixtures" and "Tests". `maple way` is the empty-page query in the tests.

**2.6 Public protos** (`feat: SearchParcelsAddresses in the public API`):
- In `api/proto/public/civil/public/parcels/v1/parcels.proto`, the RPC with the
  spec's comment and `NO_SIDE_EFFECTS`. The request carries:
  - `query` with `string = {min_len: 1, max_len: 200}`;
  - `legal_as_of` with the bound and description from
    `GetParcelExtentsWGS84ByParcelIdRequest`;
  - `first` with `int32 = {gte: 1, lte: 100}`.

  Each field gets a gnostic description, and synthetic examples (`12 Fixture
  Way`).
- `ParcelSearchHit` with gnostic descriptions. The response carries `hits`
  and `page_info`.
- In `api/proto/public/civil/public/pagination/v1/pagination.proto`, reword
  the `start_cursor` and `end_cursor` comments from "edge" to "result". Make
  the same change in the mesh copy if it has those comments.
- `mise run generate`, and commit `api/gen/go`. Check that the SDK index
  exports the new messages, and that `openapi.yaml` has the path.

**2.7 Gateway** (`feat: serve SearchParcelsAddresses from the gateway`):
- `ParcelServer.SearchParcelsAddresses` in `services/gateway/parcels.go`:
  - `first = 25` when unset, as the address/party RPCs do;
  - map the request and the hits one-to-one, with `PageInfo` copied;
  - pass cursors through unchanged.
- **Tests:**
  - `TestPublicRPCs` covers the RPC through `publicServices`, since
    `ParcelsService` is already listed. `validString` gives
    `query = "query-value"`, which is valid.
  - New tests in `services/gateway/parcel_search_test.go`, all rejected
    without calling the mesh: an empty query, a 201-character query, `first`
    of 0 and 101, and a `legal_as_of` before year 0. Also: unset `first`
    reaches the mesh as 25, and a cursor passes through.
  - Add a fully populated `ParcelSearchHit` to the mapper round-trips.

**2.8 e2e and docs** (`docs: SearchParcelsAddresses`):
- `test/e2e/smoke_test.go`: `SearchParcelsAddresses("3 Fixture Way")` through the
  gateway returns `ParcelID(3)` first.
- `api/proto/public/README.md`: the RPC under `ParcelsService`. Rewrite the
  "Pagination" section as the spec's "Docs to update" says:
  - new RPCs return `repeated <Thing> <things>` and no edges, while the
    existing RPCs still have edges until `pagination-without-edges`;
  - search pages share a snapshot;
  - search has a cap.

**2.9 Performance check** (`test: latency check for address search`):
- **`dev/perf/search-dataset.sql`** loads 500,000 extra parcels into a database
  that already has the fixtures:
  - one `INSERT ... SELECT` per table from `generate_series`, so it takes
    seconds;
  - each parcel gets a 30 m square on a grid (offset from the fixture grid),
    an attributes row with land use 1, and its own address;
  - addresses are `<house number> <street> <suffix>, <locality> ZZ<5 digits>`,
    drawn deterministically from fixed, invented word lists: about 400
    streets, 6 suffixes and 30 localities, with house numbers 1–9999 picked by
    `hashint4`. Every word is checked against the denylist rule: invented,
    with no real place names.
  - `ANALYZE` at the end.
- **`services/database-reader/search_perf_test.go`** (`//go:build perf`):
  - Start a fixture database and load the dataset. Before that, drop the
    trigram index, load, and time `CREATE INDEX` separately, so PR 2 can
    report the build time.
  - Start the reader in-process, warm up with 20 queries, then run each of
    the spec's five query kinds 200 times (fixed seeds), timing the mesh call.
  - Print p50, p95 and p99 per kind and overall, the machine (`runtime`, CPU
    model from `/proc/cpuinfo`) and the row counts.
  - Fail if overall p95 > 200 ms or p99 > 500 ms.
  - Also print `EXPLAIN (ANALYZE, BUFFERS)` for one query of each kind, to
    show that the trigram index is used.
- **`mise.toml`:** `[tasks."perf:search"]` runs
  `go test -tags perf -run TestSearchLatency -v -timeout 30m ./services/database-reader`.
- `mise run check` doesn't run it (build tag), and CI doesn't either. `go vet
  -tags perf ./...` goes in this step's checks, so the file can't rot
  unnoticed.
- If the budget isn't met, PR 2 isn't reported done. The agent reports the
  numbers and the plans to the owner, who decides between changing the query
  and amending the spec.

**Checks:**
- `mise run check`, and `buf format --diff --exit-code` in `api/`.
- Generated-code freshness.
- `buf breaking` passes without `!`.
- `mise run test:integration` and `mise run test:e2e`.
- `mise run perf:search`, with its output in the PR description.

## Final verification (before reporting the feature done)

Check each intent success criterion against `main` after PR 2 merges, with
the evidence in PR 2's description:
- **"Typing part of a fixture address returns the right parcels first":** the
  matching tests and the e2e test.
- **"A voided parcel or address never appears":** the visibility tests. The
  voided party no longer applies, since owner names aren't searched (spec,
  "Owner names").
- **The latency budget:** step 2.9's output.

## Risks

| Risk | Mitigation |
|---|---|
| Atlas can't express the expression index with a schema-qualified operator class, or the `SQL` function as written. | The owner tries it in step 1.2, and the fallbacks are listed in 1.1. If neither works, the index is hand-written in the generated migration and `atlas.sum` is regenerated. That's a last resort, because `db:diff` would then show drift. |
| The planner doesn't use the index for `q <% expr OR expr LIKE ...`. | Step 2.9's `EXPLAIN` shows it. The fallback is a `UNION` of the two conditions, each of which uses the index on its own, deduplicated by `parcel_id`. |
| `word_similarity` at 0.6 doesn't find `fixtrue way`, or finds too much. | Step 2.5 measures it and updates the spec with the owner's agreement. |
| `float32` round-tripping of `similarity` through the cursor misorders ties. | The value comes from SQL as `real`, travels as `float`, and goes back as `$6::real`, so every step is exact in 32 bits. A unit test round-trips the extremes and a subnormal value. |
| The 500,000-row index build blocks address writes in production for longer than expected. | Step 2.9 reports the build time. If it's long, the owner can run the migration in a quiet window. A `CONCURRENTLY` variant needs the tooling changes rejected in the decisions table. |
| The pagination section's rewrite contradicts the existing RPCs until `pagination-without-edges` lands. | The README says so explicitly, and that intent is merged. |

## Out of scope for this plan (per spec)

Owner names; `legacy_id`
([`../parcel-number-search/intent.md`](../parcel-number-search/intent.md));
removing edges from the existing RPCs
([`../pagination-without-edges/intent.md`](../pagination-without-edges/intent.md));
other record types; filters; abbreviation expansion, accent folding,
highlighting and suggestions; backward pagination and `totalCount`;
`system_as_of`; and any write path.
