# Intent: one legal-time rule for valuation reads

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-25 |
| **Status** | Draft, spawned by [`../improvement-read-api/spec.md`](../improvement-read-api/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

`GetParcelsWithImprovementSummaryByParcelId` takes a `valuation_id` and a
`legal_as_of`, and reads two valuation tables with different rules
(`services/database-reader/parcels.go`):

| Field | Table | Legal-time filter |
|---|---|---|
| `ParcelDetails.market_land_value`, `assessed_land_value` | `parcel_valuations` | `pv.legal_valid_range @> legal_as_of` |
| `ParcelImprovementsSummary.total_market_improvement_value`, `total_assessed_improvement_value` | `improvement_valuations` | none |

Both tables have the same shape and constraints: a `legal_valid_range`, a
unique index on `(<record>_id, valuation_id)`, and an exclusion constraint
that stops one record's rows from overlapping in legal time.

So the two halves of one response can disagree. Suppose a valuation's rows are
valid from 2026-01-01 and a client reads it with `legal_as_of` in 2025 (or
leaves it unset before that date). The response:

- leaves the land values unset, because the parcel row isn't in range;
- still returns the improvement totals, because nothing checks their rows.

A parcel whose improvements' rows have different ranges gets a total that
mixes in-range and out-of-range values. The fixtures don't show this, because
every valuation row starts on 2020-01-01 and is open-ended.

The improvement read API ([`../improvement-read-api/spec.md`](../improvement-read-api/spec.md))
adds per-improvement values, and its spec reads them the way `parcel_valuations`
is read. Without a rule for the whole API, the summary and the new read would
also disagree with each other.

## Proposed outcome

1. **One documented rule** for what `legal_as_of` means when a `valuation_id`
   is requested, applied to every read of `parcel_valuations` and
   `improvement_valuations` in the public API.
2. **The parcel summary follows it**, as a `fix:` with no proto change, so
   its land values and improvement totals agree.
3. **The proto descriptions say it**: the `valuation_id` and value fields
   state when a value is unset.
4. **Fixtures and tests** with a valuation row whose range doesn't contain
   now, so the rule is tested and not just assumed.

## Constraints

- **No proto change and nothing breaking.** Only behaviour and descriptions
  change.
- **No migration unless the open questions call for one.** If they do, the
  owner runs Atlas (see `.claude/skills/atlas-migrations/SKILL.md`).
- **Only synthetic data.**

## Success criteria

- For any `valuation_id` and `legal_as_of`, the land values and the
  improvement totals in one response come from rows chosen by the same rule.
- Summing the improvement read API's per-improvement values over a parcel's
  `improvement_ids` gives the summary's totals, for any `legal_as_of`.

## Open questions (carry into spec)

- **What a valuation row's legal range means.** Is it the period the value
  applies to (a tax year), or when the value was recorded? That decides
  whether filtering on `legal_as_of` is right at all, or whether
  `valuation_id` alone should pick the row.
- **Filter or not.** Should both tables be filtered by `legal_as_of` (today's
  parcel behaviour), or should neither be, with the valuation choosing the row
  on its own?
- **The exclusion constraints ignore `valuation_id`.** One parcel or
  improvement can't have overlapping rows even in *different* valuations, so
  two rolls (say a 2025 and a 2026 valuation) need disjoint ranges. Is that
  intended, or should `valuation_id` be part of the constraint? This decides
  whether `legal_as_of` and `valuation_id` say the same thing twice.
- **`valuations.valuation_date`.** Should it play a part in choosing rows, or
  in the default `legal_as_of` when a valuation is requested?
- **Comparables.** `GetSalesComparables` and `GetEquityComparables` don't read
  valuations today. Should the rule be written so that they follow it if they
  ever do?
