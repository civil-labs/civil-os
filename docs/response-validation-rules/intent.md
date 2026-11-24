# Intent: document and test the values responses can take

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-24 |
| **Status** | Draft, at the owner's request during the full and half bathrooms spec. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

API consumers learn what values a response field can take from two places, and civil-os
uses neither consistently:

| Channel | Reaches | From |
|---|---|---|
| Proto comments | TypeScript SDK users as JSDoc (hover in the IDE), Go users as doc comments, and OpenAPI as `description` | Leading comments on the field |
| `buf.validate` rules | OpenAPI as JSON Schema constraints: `protoc-gen-connect-openapi` turns `gte: 0` into `minimum: 0`, a `pattern` into `pattern`, a UUID rule into `format: uuid` | Field options |

The rules don't reach the TypeScript SDK's types, so for SDK users the comment is the only
place a range or format can be stated.

Today:

- **Comments are uneven.** Some fields explain their values, such as the transfer amount's
  decimal format and when it's unset. Many say nothing, or say something like "Physically
  impossible to be negative" without stating the range.
- **Rules are on some response fields and not others,** and nothing checks them. The
  gateway's protovalidate interceptor (`validate.NewInterceptor()` in
  `services/gateway/main.go`) checks **requests** only. A rule on a response field is a
  published promise in the OpenAPI spec that no test or runtime check verifies.

These public messages appear only in responses and carry rules (lines with a
`buf.validate` option, on 2026-11-24):

| Message (`civil.public.*.v1`) | Rules | Reached through |
|---|---|---|
| `parcels.ParcelDetails` | 15 | `GetParcelsWithImprovementSummaryBy…Response` |
| `parcels.ParcelImprovementsSummary` | 11 | the same |
| `parcels.GetNumericalParcelAttributeStatsByIdResponse` | 17 | itself |
| `parcels.GetCategoricalParcelAttributeStatsByIdResponse` | 2 | itself |
| `parcels.ParcelAffordances` | 6 | nothing: no message references it |
| `neighborhoods.Neighborhood` | 1 | `GetNeighborhoodsResponse` |
| `neighborhoods.NeighborhoodDefinition` | 1 | `GetNeighborhoodDefinitionsResponse` |
| `valuations.Valuation` | 1 | `GetValuationsResponse` |

Other response fields that have constraints carry no rule at all.

## Proposed outcome

1. **Every public response field has a comment** that states what it holds and the values it
   can take: its range or format, and when it's unset, null or 0. The comments use one fixed
   phrasing, so they read the same across the API and the SDK.
2. **Every response field with a constraint a rule can express has that rule,** so the
   OpenAPI spec carries it as a JSON Schema constraint. Rules that say nothing (such as
   `float = {}`) are removed. Where a rule can't express a constraint (for example "in steps
   of 0.5"), the comment carries it.
3. **Tests check responses against their rules.** The reader and gateway integration tests
   run `protovalidate` on every public response they receive, so a rule the server breaks
   fails CI. No runtime response validation.
4. **A check that new response fields get a comment,** so the gap doesn't reopen (for example
   a `buf lint` comment rule on the public module, or a test over the descriptors).
5. **`ParcelAffordances`:** the spec decides whether to delete it, since nothing uses it,
   or keep it for a planned use.

## Constraints

- **No wire or behavior change.** Comments and validation options change no field, number
  or type. The spec confirms that `buf breaking` doesn't flag adding or removing a rule.
- **A new rule must match what the server returns today.** If a test finds a response that
  breaks a rule, that's a data or reader bug to fix or a rule to correct, decided case by
  case; it isn't hidden by loosening the test.
- **Generated code is regenerated** (`mise run generate`), and the OpenAPI output changes
  with it.
- **The mesh protos are out of scope.** They have no rules and no external consumers.

## Success criteria

- Every public response field has a comment stating its possible values.
- The OpenAPI spec carries a constraint for every rule-expressible one.
- The integration tests validate every public response they receive, and pass.
- CI fails when a new public response field has no comment.

## Out of scope

- Request validation, which is unchanged.
- Validating responses at runtime in the gateway.
- Changing field types to express constraints (for example `uint32` for counts), which
  would be breaking.
- Fields added by in-flight specs, which follow this approach already: the full and half
  bathrooms spec gives its fields both comments and rules, and tests its responses.

## Open questions (carry into spec)

1. **The comment phrasing:** a fixed wording for ranges, formats, units and absence.
2. **How responses are validated in tests:** a helper the integration tests call, or a
   test-only interceptor on the client that validates every response.
3. **The comment check:** `buf lint`'s `COMMENT_FIELD` rule, or a custom check limited to
   response fields.
4. **`ParcelAffordances`:** delete or keep.
5. **Order of work:** all messages in one PR, or one service at a time.
