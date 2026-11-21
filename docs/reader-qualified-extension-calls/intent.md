# Intent: the reader names the schema of every extension function it calls

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-21 |
| **Status** | Draft, written from a production `v0.5.1` rollout at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

After a production deployment moved to `v0.5.1`, address search failed in the
database reader:

```
failed to read parcel search: ERROR: function word_similarity(text, text) does not exist (SQLSTATE 42883)
```

The migrations install every extension (PostGIS, `pg_trgm`, `pgcrypto`,
`btree_gist`) in the `extensions` schema. The reader names its tables'
schemas (`core.`, `geo.`), but it calls extension functions and operators
without one. Those calls only resolve if the session's `search_path` includes
`extensions`.

civil-os assumes a database-level `search_path = core, geo, extensions, public`
"set by cluster infrastructure" (`docs/monorepo-migration/plan.md`,
`docs/parcel-search/spec.md`). Nothing sets it. The only deployment config that
did is commented out, and it names the old database. On that deployment's
read replica, `pg_trgm` is installed in `extensions` and a plain
`SELECT word_similarity(...)` fails.

- **The calls involved.** `word_similarity` and `<%` in `parcel_search.go`, and
  about 20 PostGIS calls (`ST_Extent`, `ST_XMin`/`ST_XMax`/`ST_YMin`/`ST_YMax`,
  `ST_Intersects`, `ST_GeomFromText`, `ST_X`/`ST_Y`, `ST_EstimatedExtent`) in
  `parcels.go` and `addresses.go`. The PostGIS ones are probably broken in
  production for the same reason, just not reported yet.
- **The tests can't catch it.** `internal/testutil/postgres.go` and
  `dev/migrate.sh` both run `ALTER DATABASE … SET search_path`, so every test
  and the compose stack provide the setting that production lacks.
- **The fix in production is a session setting.** The Helm chart now sets
  `PGOPTIONS=-c search_path=core,geo,extensions,public` on the reader. That
  works, but it makes correctness depend on deployment config that each new
  deployment has to repeat.

## Proposed outcome

1. **The reader's SQL names the schema of every extension function and
   operator it uses**, e.g. `extensions.word_similarity(...)`,
   `OPERATOR(extensions.<%)`, `extensions.ST_Extent(...)`. Its queries then
   work with any `search_path`.
2. **Tests run without the database `search_path`**, so an unqualified call
   fails in CI instead of in a cluster.
3. **The docs stop claiming that cluster infrastructure sets `search_path`.**
   Any remaining dependency on it is stated where it's relied on.

## Constraints

- No change to query results or plans: the trigram index
  (`idx_address_attributes_formatted_address_trgm`) must still drive address
  search. A qualified `OPERATOR(extensions.<%)` has to match the index's
  operator class.
- No schema migration is expected. The extensions stay in `extensions`.
- The chart's setting can stay as defence in depth. Removing it is a separate
  decision, made in the deployment repos.
- The tile server is out of scope: its connection string sets its own
  `search_path`, and Martin generates its own SQL.

## Success criteria

- A CI test runs the reader's queries against a database with no database- or
  role-level `search_path` (the default `"$user", public`), and all of them
  pass, address search and parcel extents included.
- `EXPLAIN` of the address search query still uses the trigram index.
- No reader query calls an extension function or operator without a schema
  (checked by a test or a lint over the reader's SQL).

## Open questions (carry into spec)

- Should tests drop the database `search_path` entirely, or keep one suite
  with it and add one without? Other services or SQL functions may rely on it.
- Do any SQL functions in the schema (not the reader's Go code) call extension
  functions unqualified? Those run with the caller's `search_path` unless they
  set their own.
- How is "no unqualified extension call" checked: a test that runs every
  query, or a static check over the SQL strings?
- Should `dev/migrate.sh` and the fixtures stop setting `search_path`, so the
  compose stack matches production?
