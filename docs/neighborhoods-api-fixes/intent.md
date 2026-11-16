# Intent: fix the neighborhoods read API

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The neighborhoods API can't be used as it stands. A client can't tell which
neighborhood definitions exist or which one is legal, can't tell which definition a
neighborhood belongs to, and gets no neighborhood on a parcel unless it already knows a
definition's ID.

The schema models neighborhoods like this:

- `core.neighborhood_definitions`: named groupings of parcels, each with `is_legal`.
  Several definitions can exist, and none, one or several of them can be legal.
- `core.neighborhoods`: the neighborhoods themselves (`public_id`, `legacy_id`, `name`).
  They have no column linking them to a definition.
- `core.parcel_neighborhood_definitions`: assigns a parcel to one neighborhood under one
  definition. For a legal definition, it has a `legal_valid_range`, with no overlaps per
  parcel and definition. For a non-legal one, the range may be null, and there's one row
  per parcel and definition.

These problems are in the current protos and the reader:

| # | Where | Problem |
|---|---|---|
| 1 | `GetNeighborhoodDefinitionsResponse` (public and mesh) | It's typed `map<string, Neighborhood> neighborhoods`, not a map of `NeighborhoodDefinition`. The `NeighborhoodDefinition` message, which has `is_legal`, is defined but never used. Clients get each definition's ID and name, but not whether it's legal. |
| 2 | `database-reader/neighborhoods.go` | The definitions query doesn't select `is_legal`, so even a correctly typed response couldn't carry it. |
| 3 | `GetNeighborhoodsResponse` docs | The map is documented as "keyed by the UUID neighborhood definition ID". It's keyed by the neighborhood's own ID. |
| 4 | `GetNeighborhoods` | It returns every neighborhood, with no way to tell or filter which definition each belongs to. The schema only links them through the per-parcel table. |
| 5 | `ParcelDetails.neighborhood_id` | It's empty unless the request passes `neighborhood_definition_id`. The documentation says so, but a client has no way to find the right ID, because of #1. |
| 6 | Parcel reader join | It joins `parcel_neighborhood_definitions` on `legal_valid_range @> legal_as_of`. A non-legal definition's assignments may have a null range, which never matches, so **a non-legal definition never returns a neighborhood**, even when one is assigned. |
| 7 | `NeighborhoodService` RPCs | They don't set `idempotency_level = NO_SIDE_EFFECTS`, unlike the other lookup services (land uses, zoning), so they can't be called with Connect GET or cached as safe reads. |

## Proposed outcome

1. **`GetNeighborhoodDefinitions` returns definitions:**
   - a map of `NeighborhoodDefinition` keyed by definition ID;
   - each with `id`, `name` and `is_legal`;
   - in both public and mesh protos, with the reader selecting `is_legal`.
2. **Neighborhoods can be read per definition.**
   - `GetNeighborhoods` takes an optional `neighborhood_definition_id`. With one, it
     returns only the neighborhoods that definition uses, through
     `parcel_neighborhood_definitions`.
   - Each returned neighborhood says which definition or definitions it belongs to.
   - The response documentation is corrected (#3).
3. **Parcels return their neighborhood for non-legal definitions too.** The parcel join
   treats a null `legal_valid_range` on a non-legal assignment as always valid (#6).
4. **The neighborhood RPCs are marked `NO_SIDE_EFFECTS`**, like the other lookup services.
5. **The synthetic fixtures cover:**
   - two definitions, one legal and one not;
   - a neighborhood used by only one of them;
   - a parcel assigned under both;
   - a non-legal assignment with a null range.

   Every RPC then gets handler tests and reader integration tests against these
   fixtures.

## Constraints

- **This is a breaking change.** Changing the type of
  `GetNeighborhoodDefinitionsResponse`'s field breaks generated clients, and `buf breaking`
  flags it. The PR title carries `!`. Pre-1.0, that's a minor bump.
  - To limit the break, the field can be renamed to `neighborhood_definitions` with a new
    field number, and the old field reserved. Old clients then see an empty map instead
    of mis-typed data.
- **Read-only.** Writing definitions or assignments is a separate feature.
- **No schema change is planned.** If the spec decides neighborhoods need a direct link
  to their definition (#4), that's a migration, and it must be called out there.
- **Only synthetic data.** No real neighborhood or definition names in fixtures, docs or
  tests.

## Success criteria

- `GetNeighborhoodDefinitions` returns every definition with the correct `is_legal`.
- `GetNeighborhoods` with a definition ID returns exactly the neighborhoods that
  definition uses in the fixtures. Without one, it returns all of them, each labeled
  with its definitions.
- A parcel requested with the non-legal fixture definition returns its neighborhood.
  With the legal one, it returns the neighborhood valid at `legal_as_of`.
- Both neighborhood RPCs can be called with Connect GET.
- The public API README describes how a client picks a definition and reads a parcel's
  neighborhood.

## Open questions (carry into spec)

- **Default definition on parcel reads.** When a request has no
  `neighborhood_definition_id`, should parcel reads use the legal definition if there's
  exactly one? Or should they stay empty, as now? Several legal definitions are allowed
  by the schema, so a default needs a rule for that case.
- **Neighborhood-to-definition link.** Should it be derived from
  `parcel_neighborhood_definitions` (no migration, but a neighborhood with no parcels
  belongs to no definition), or should `neighborhoods` get a `neighborhood_definition_id`
  column (a migration, and it rules out sharing a neighborhood between definitions)?
- **Lookup by ID.** Should there be `GetNeighborhoodsById` and
  `GetNeighborhoodDefinitionsById`, or are whole-table reads enough, as with land uses
  and zoning?
- **Field change strategy.** Change the field's type in place, or add a new field and
  reserve the old one? Both are breaking. The second fails more safely for clients that
  haven't upgraded.
