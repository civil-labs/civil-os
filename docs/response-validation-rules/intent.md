# Intent: document response values in comments, not validation rules

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-24 |
| **Status** | Draft, at the owner's request during the full and half bathrooms spec. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The gateway validates messages with connectrpc's protovalidate interceptor
(`validate.NewInterceptor()` in `services/gateway/main.go`). It checks **requests** only.
Yet several public messages that only ever appear in responses carry
`buf.validate` rules:

| Message (`civil.public.*.v1`) | Rules today | Reached through |
|---|---|---|
| `parcels.ParcelDetails` | 15 | `GetParcelsWithImprovementSummaryBy…Response` |
| `parcels.ParcelImprovementsSummary` | 11 | the same |
| `parcels.GetNumericalParcelAttributeStatsByIdResponse` | 17 | itself |
| `parcels.GetCategoricalParcelAttributeStatsByIdResponse` | 2 | itself |
| `parcels.ParcelAffordances` | 6 | nothing: no message references it |
| `neighborhoods.Neighborhood` | 1 | `GetNeighborhoodsResponse` |
| `neighborhoods.NeighborhoodDefinition` | 1 | `GetNeighborhoodDefinitionsResponse` |
| `valuations.Valuation` | 1 | `GetValuationsResponse` |

(Counts are lines with a `buf.validate` option, on 2026-11-24.)

These rules are never enforced: not by the gateway, and not by any client (neither the JS
SDK nor opencama-record-keeper uses protovalidate). They read like guarantees, such as
`gte: 0.0`, a UUID format or a decimal pattern, that nothing checks. New fields copy them
because the neighbouring fields have them; the full and half bathrooms spec nearly did.

Some of them still carry information a consumer needs: that an ID is a UUID, that a value
is never negative, the decimal-string pattern of money values, or a list's maximum length.
Dropping them without saying so elsewhere would lose that.

## Proposed outcome

1. **Remove every `buf.validate` rule from response-only messages** in the public protos,
   starting with the eight messages above.
2. **Move what the rules said into the fields' comments,** so API consumers still see each
   field's possible values: for example "A UUID", "Always >= 0", "A decimal string with up
   to 15 digits before the point and 4 after", "At most 50 items". Fields whose rule says
   nothing a consumer needs (such as `float = {}`) just lose the rule.
3. **A check that stops rules coming back:** a test in the gateway or `api/` that walks
   the public descriptors and fails when a message reachable only from responses has a
   `buf.validate` option. Messages used in both requests and responses keep their rules.
4. **`ParcelAffordances`:** the spec decides whether to delete it, since nothing uses it,
   or keep it for a planned use.

## Constraints

- **No wire or behavior change.** Removing validation options changes no field, number or
  type, and the interceptor never ran them. It isn't a breaking change, and `buf breaking`
  shouldn't flag it. The spec confirms that.
- **Generated code is regenerated** (`mise run generate`), since the descriptors embedded in
  `api/gen` and `sdk/js/gen` change.
- **Gnostic OpenAPI annotations stay.** They're documentation and are published.
- **The mesh protos have no `buf.validate` rules,** so they don't change.

## Success criteria

- No response-only public message has a `buf.validate` option, and the new check enforces
  it in CI.
- Every field that lost a rule which said something has a comment that says the same thing.
- `mise run check` and `buf breaking` pass with no breaking change.

## Out of scope

- Request validation, which is unchanged.
- Adding response validation, for example a server-side interceptor that validates what the
  gateway returns.
- Fields added by in-flight specs: the full and half bathrooms spec already adds its fields
  without rules and documents their values in comments.

## Open questions (carry into spec)

1. **Where the check lives:** a Go test over the generated descriptors, or a `buf` lint
   plugin.
2. **How "response-only" is computed:** reachability from RPC output types, minus anything
   reachable from input types.
3. **`ParcelAffordances`:** delete or keep.
4. **Comment style:** a fixed phrasing for ranges and formats, so comments read the same
   across the API.
