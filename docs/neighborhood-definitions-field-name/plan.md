# Plan: rename the definitions map to `neighborhood_definitions`

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md), proposed in the same PR at the owner's request, with no intent |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-17 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One PR** from the owner's branch `fix/definitions-name`, titled
  `fix!: rename the neighborhood definitions map to neighborhood_definitions`.
  It holds the spec, this plan and the change. The owner merges, and the squash
  keeps only the title.
- **The owner's commits stay as they are.** The agent adds commits on top. It
  doesn't rebase or rewrite them.
- **Generated code is never edited by hand.** It comes from
  `mise run generate`.

## Steps

**1. Protos and generated code** (done by the owner, in the branch's existing
`fix!: neighborhood definitions name` commits)

- Field 1 of `GetNeighborhoodDefinitionsResponse` is renamed to
  `neighborhood_definitions` in the public and mesh protos.
- `mise run generate`.
- The gateway sets `NeighborhoodDefinitions` on the public response.

**2. Callers** (`fix!: use neighborhood_definitions in the reader, gateway and tests`)

- The reader builds the mesh response with `NeighborhoodDefinitions`.
- The gateway reads `meshRes.Msg.NeighborhoodDefinitions`.
- `TestReferenceData`, `TestNeighborhoodDefinitions` and the e2e
  `neighborhoods` subtest read `NeighborhoodDefinitions`.

**3. Docs** (`docs: spec and plan for renaming the neighborhood definitions map`)

This spec and plan.

## Checks

- `go build ./...`, and `go vet` with no tags and with the `integration` and
  `e2e` tags.
- `mise run check`, and `buf format --diff --exit-code` in `api/`.
- Generated-code freshness: no diff in `api/gen/go` or `internal/gen/go` after
  `mise run generate`.
- `buf breaking` against `main` fails only on this field: the name, the
  `json_name` and the map-entry type. The output goes in the PR description.
- Integration and e2e: run by CI on the PR. Run locally too, when a container
  runtime is available.

## Risks

| Risk | Mitigation |
|---|---|
| A JSON client on `v0.3.0` breaks. | Stated in the PR and the release notes, through `!`. Binary clients are unaffected. |
| A caller of the old name is missed. | It doesn't compile, and `go vet` runs under all three build tags. |
