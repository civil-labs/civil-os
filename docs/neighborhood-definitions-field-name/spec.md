# Spec: rename the definitions map to `neighborhood_definitions`

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-17 |
| **Status** | Draft, for the owner's review. Accepted when merged. |
| **Intent** | None. The owner asked for a spec and plan, in the same PR as the change, on 2026-11-17. |

## Problem

`GetNeighborhoodDefinitionsResponse` holds `NeighborhoodDefinition`s, but its
map field is called `neighborhoods`, in both the public and mesh protos. That
reads as if it held neighborhoods, the same as `GetNeighborhoodsResponse`'s
field.

The name was kept on purpose in
[`../neighborhoods-api-fixes/spec.md`](../neighborhoods-api-fixes/spec.md), to
keep the JSON key stable while the value type changed. That shipped in
`public-api-v0.3.0`. This spec reverses that one decision. The rest of the
earlier spec stands.

## Decisions

| Question | Decision |
|---|---|
| New name | **`neighborhood_definitions`** (JSON `neighborhoodDefinitions`, Go `NeighborhoodDefinitions`). It matches the message type and the RPC name. |
| Field number | **Unchanged, `1`.** The map's key and value types don't change. |
| Rename in place, or add a field | **Rename in place.** Adding field 2 and deprecating field 1 would mean filling both for a release, for a response that's a few rows of reference data, pre-1.0. The break is small and stated in the PR. |
| Mesh | **Renamed too**, the same way. The mesh has no compatibility promise. The field number is unchanged, so a reader and gateway on different versions still agree on the binary wire during a rollout. |
| `GetNeighborhoodsResponse.neighborhoods` | **Unchanged.** It does hold neighborhoods. |
| Description | Unchanged: "Map of neighborhood definitions keyed by the UUID neighborhood definition ID". |

## What changes for clients

| Client | Effect |
|---|---|
| Binary protobuf (Connect, gRPC, gRPC-Web), not upgraded | **None.** Field 1 and the map entry's fields 1 and 2 are unchanged on the wire. The old generated name still decodes it. |
| JSON, not upgraded | The key changes from `neighborhoods` to `neighborhoodDefinitions`. Depending on its unknown-field setting, an old client either fails to parse the response or sees no definitions. |
| Regenerated code (Go, TypeScript) | Doesn't compile until `Neighborhoods` / `neighborhoods` becomes `NeighborhoodDefinitions` / `neighborhoodDefinitions` on this response. |

`buf breaking` fails, as expected, on the field name, its `json_name`, and the
generated map-entry type (`NeighborhoodsEntry` → `NeighborhoodDefinitionsEntry`,
with the same fields). The PR title carries `!`. Pre-1.0, release-please makes
that a minor bump.

## Code

- `api/proto/public/.../neighborhoods.proto` and
  `api/proto/mesh/.../neighborhoods.proto`: rename field 1, then run
  `mise run generate`.
- `services/database-reader/neighborhoods.go`: build the response with
  `NeighborhoodDefinitions`.
- `services/gateway/neighborhoods.go`: read `NeighborhoodDefinitions` from the
  mesh, and set it on the public response.
- Tests that read definitions: `TestReferenceData`,
  `TestNeighborhoodDefinitions` and the e2e `neighborhoods` subtest. They use
  the new name, and their expectations don't change.

No reader query, fixture or migration changes.

## Tests

The existing coverage carries over unchanged:
- `TestPublicRPCs` round-trips every field of the response, so it covers the
  gateway's mapping under the new name.
- The reader integration tests and the e2e subtest check the definitions'
  content.

No new test is needed, because a rename that missed a spot doesn't compile.

## Release

One PR, `fix!: rename the neighborhood definitions map to neighborhood_definitions`,
holding this spec, the plan and the change.
