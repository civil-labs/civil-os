# Spec: fix the neighborhoods read API

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Decisions signed off by the owner on 2026-11-16. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: the proto changes, how the reader resolves
definitions and assignments, the fixtures and the tests. The order of the
work goes in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Default definition on parcel reads | **None. `neighborhood_id` stays empty unless the request names a definition.** A client finds the legal definition with `GetNeighborhoodDefinitions`, which becomes a cacheable GET (#7). A default of "the only legal definition" would quietly change what existing requests return as soon as a second legal definition is added, which the schema allows. |
| Neighborhood-to-definition link | **Derived from `parcel_neighborhood_definitions`, with no migration.** A definition uses a neighborhood when at least one assignment row links them, at any time, for a parcel that isn't voided. A neighborhood can belong to several definitions. A neighborhood with no such assignments belongs to none. That's a known limit (see below). |
| Lookup by ID | **No `...ById` RPCs.** Both tables are small reference data, read whole like land uses and zoning. With `NO_SIDE_EFFECTS` the whole-table reads can be cached. |
| Field change strategy | **Change the type of field 1 in place**, keeping its number and its name `neighborhoods`, in both the public and mesh protos. On the wire this is mild. `NeighborhoodDefinition` keeps `Neighborhood`'s field 1 (`id`) and field 2 (`name`), so an old binary client still decodes both and ignores `is_legal` as an unknown field. JSON clients see the same key, plus `is_legal`. Generated code does break, since the map's value type changes, so `buf breaking` fails and the PR title carries `!`. Pre-1.0, that's a minor bump. |

## Further decisions

| Question | Decision |
|---|---|
| Which assignment applies at `t` | **An assignment row applies at `t` when `legal_valid_range @> t`, or when the range is null.** The schema allows a null range only for a non-legal definition (`chk_legal_must_have_time`). A non-legal row with a range is honoured like a legal one. At most one row applies per parcel and definition: the exclusion constraint guarantees that for legal definitions, and the unique index `idx_unique_non_legal_parcel_neighborhood` for non-legal ones. So the parcel join can't duplicate a parcel. |
| `GetNeighborhoods` with an unknown definition | **An empty map, not an error.** That's consistent with unknown IDs in the other reads, such as the address and party lookups. |
| Time on `GetNeighborhoods` | **None.** "Uses" means "has ever been assigned", so the request takes no `legal_as_of`. A client that needs a parcel's neighborhood at a time reads the parcel. |
| Order of `neighborhood_definition_ids` | Ascending by definition public ID, so responses are deterministic. |
| Voiding | None of the three neighborhood tables has `is_voided`. Assignments to voided parcels don't count towards "uses", following `CLAUDE.md`'s rule that every new query filters voided parcels. The parcel read already hides voided parcels. |
| Mesh service name | `NeighborhoodReadService` keeps its name. Renaming it belongs to [`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md). |

## Public API: `civil.public.neighborhoods.v1`

```proto
service NeighborhoodService {
  // Returns every neighborhood definition.
  rpc GetNeighborhoodDefinitions(GetNeighborhoodDefinitionsRequest) returns (GetNeighborhoodDefinitionsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // Returns the neighborhoods, each with the definitions that use it, or
  // only those one definition uses.
  rpc GetNeighborhoods(GetNeighborhoodsRequest) returns (GetNeighborhoodsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message NeighborhoodDefinition {
  string id = 1;     // unchanged
  string name = 2;   // unchanged
  // Whether the definition has legal meaning. A legal definition's
  // assignments are dated, so a parcel's neighborhood depends on legal_as_of.
  bool is_legal = 3;
}

message Neighborhood {
  string id = 1;     // unchanged
  string name = 2;   // unchanged
  // The definitions that assign at least one (non-voided) parcel to this
  // neighborhood, at any time, ordered by ID. Empty for a neighborhood no
  // definition uses.
  repeated string neighborhood_definition_ids = 3;   // new
}

message GetNeighborhoodDefinitionsResponse {
  // Keyed by the UUID neighborhood definition ID.
  map<string, NeighborhoodDefinition> neighborhoods = 1;   // type changed (#1)
}

message GetNeighborhoodsRequest {
  // Only the neighborhoods this definition uses. Every neighborhood when
  // unset. An unknown ID returns an empty map.
  optional string neighborhood_definition_id = 1 [(buf.validate.field).string.uuid = true];   // new
}

message GetNeighborhoodsResponse {
  // Keyed by the UUID neighborhood ID.   (documentation corrected, #3)
  map<string, Neighborhood> neighborhoods = 1;
}
```

Existing gnostic annotations stay. New fields get gnostic descriptions with
the wording above.

### `civil.public.parcels.v1` (documentation only)

- `ParcelDetails.neighborhood_id`: "The parcel's neighborhood under the
  requested `neighborhood_definition_id`, at `legal_as_of` for a legal
  definition. Unset when no definition is requested, or the parcel has no
  neighborhood under it then."
- `neighborhood_definition_id` on both parcel-details requests: "A
  neighborhood definition ID from `NeighborhoodService.GetNeighborhoodDefinitions`.
  Without one, `neighborhood_id` is unset."

These are description changes only, so they aren't breaking.

## Mesh API

`civil.mesh.neighborhoods.v1` gets the same changes without validation or
gnostic annotations: field 1 of `GetNeighborhoodDefinitionsResponse` becomes
`map<string, NeighborhoodDefinition>`, `Neighborhood` gains
`repeated string neighborhood_definition_ids = 3`, and `GetNeighborhoodsRequest`
gains `optional string neighborhood_definition_id = 1`. The mesh doesn't
set idempotency levels, like the other mesh services.

## Reader

- **`GetNeighborhoodDefinitions`** selects `public_id`, `name` and `is_legal`
  (#2) and returns `NeighborhoodDefinition`.
- **`GetNeighborhoods`** runs one query: neighborhoods `LEFT JOIN` the
  definitions that use them (an aggregate over `parcel_neighborhood_definitions`
  joined to non-voided parcels and to `neighborhood_definitions` for the
  public ID), with `array_agg(DISTINCT nd.public_id::text ORDER BY ...)`. With
  a `neighborhood_definition_id` (`$1::uuid`), it keeps only neighborhoods
  whose list contains that ID. Each returned neighborhood still lists all its
  definitions, not just the requested one. A non-UUID ID reaching the reader
  directly (the mesh has no validation) fails with `InvalidArgument`.
- **Parcel details** (`parcels.go`): the `pnd` join changes from
  `pnd.legal_valid_range @> $1` to
  `(pnd.legal_valid_range @> $1 OR pnd.legal_valid_range IS NULL)` (#6). The
  rest of the join stays: no definition requested, no neighborhood.

No migration. The existing indexes on `parcel_neighborhood_definitions`
(`parcel_id`, `neighborhood_id`, `neighborhood_definition_id`) cover these
queries, and the tables are small.

## Gateway

`services/gateway/neighborhoods.go` maps `NeighborhoodDefinition` (with
`is_legal`) and the new `Neighborhood` field, and passes
`neighborhood_definition_id` through. The handler is already registered with
validation, CORS and auth.

## Fixtures

Added to `database/fixtures/synthetic.sql`, with constants in
`internal/testutil/fixtures.go`. Definition 1 and neighborhoods 1 and 2 exist
today: definition 1, `Fixture districts`, is legal, and assigns parcels 1–10
to `North Fixture` (1) and 11–20 to `South Fixture` (2) from 2020-01-01.

| Fixture | Rows | Covers |
|---|---|---|
| Non-legal definition 2 | `Fixture market areas`, `is_legal = false` | `is_legal` in `GetNeighborhoodDefinitions`. |
| Neighborhood 3 | `Fixture Market Area`, used only by definition 2 | a neighborhood used by only one definition |
| Neighborhood 4 | `Unused Fixture`, no assignments | a neighborhood with no definitions |
| Definition 2 assignments | parcels 1–5 → neighborhood 3, null range; parcel 6 → neighborhood 3, `[2020-01-01, 2023-01-01)`; parcel 7 → neighborhood 1, null range | parcels assigned under both definitions; a null range; a dated non-legal row; neighborhood 1 shared by both definitions |
| Parcel 10's legal history | definition 1: South (2) for `[2020-01-01, 2023-01-01)`, then North (1) from 2023-01-01. Today it's North from 2020. | a legal assignment resolved at `legal_as_of` |

The existing assignment insert `CROSS JOIN`s every definition. It must be
restricted to definition 1, or it would insert legal rows for definition 2,
which the composite foreign key on `(neighborhood_definition_id, is_legal)`
rejects. The existing integration test reads parcel 3's neighborhood under
definition 1, and that doesn't change.

## Tests

| Layer | Coverage |
|---|---|
| Reader integration: definitions | Two definitions: 1 legal, 2 not, each with its name. |
| Reader integration: neighborhoods | Unset: neighborhoods 1 (`[1, 2]`), 2 (`[1]`), 3 (`[2]`) and 4 (`[]`). Definition 1: neighborhoods 1 and 2, with neighborhood 1 still listing both definitions. Definition 2: neighborhoods 1 and 3. Unknown UUID: empty. Non-UUID sent directly to the reader: `InvalidArgument`. |
| Reader integration: parcels | Parcel 3: neighborhood 1 under definition 1, 3 under definition 2, unset with no definition. Parcel 7 under definition 2: neighborhood 1. Parcel 6 under definition 2: unset now, neighborhood 3 at 2022. Parcel 10 under definition 1: North now, South at 2022. Parcel 12 under definition 2: unset. Both by parcel ID and by feature ID. |
| Gateway handler tests | `TestPublicRPCs` (full mapping round-trip, including `is_legal` and `neighborhood_definition_ids`) and `TestPublicRPCValidation` (non-UUID `neighborhood_definition_id`) cover both RPCs once the request has an ID field. A Connect GET to each RPC succeeds. |
| e2e (`test/e2e`) | `GetNeighborhoodDefinitions` returns definition 1 as legal; `GetNeighborhoods` for definition 2 returns neighborhoods 1 and 3; parcel 3 under definition 2 returns neighborhood 3. One call goes over Connect GET. |

## Docs to update

- `api/proto/public/README.md`: a "Neighborhoods" section on how a client picks
  a definition (`GetNeighborhoodDefinitions`, `is_legal`), lists its
  neighborhoods (`GetNeighborhoods` with the ID) and reads a parcel's
  neighborhood (`neighborhood_definition_id` on the parcel read, with
  `legal_as_of` for a legal definition).
- The `NeighborhoodService` row in the README's service table.

## Release

One PR, `fix!: return neighborhood definitions and their neighborhoods`. It
carries `!` because `buf breaking` flags the field's type change and the
idempotency change. Pre-1.0, release-please makes that a public-API minor bump. The PR
description says what an un-upgraded client sees (above).

## Known limits

- A neighborhood is only linked to a definition through parcel assignments.
  A newly created neighborhood with no parcels yet belongs to no definition,
  and `GetNeighborhoods` with a definition ID leaves it out. A direct link
  needs a migration and is out of scope.
- "Uses" ignores time, so a definition's list includes neighborhoods it only
  used in the past.

## Out of scope

Writing definitions, neighborhoods or assignments; a default definition on
parcel reads; `...ById` lookups; neighborhood geometry; renaming the mesh
service; and a direct neighborhood-to-definition column.
