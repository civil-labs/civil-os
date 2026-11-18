# Intent: expose and search parcel numbers

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-18 |
| **Status** | Draft, spawned by [`../parcel-search/spec.md`](../parcel-search/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

Jurisdictions and their staff identify parcels by their own parcel numbers,
which civil-os stores as `parcels.legacy_id`. The public API never returns
`legacy_id`, so a client can neither show a parcel's number nor find a parcel
from one. Parcel search ([`../parcel-search/spec.md`](../parcel-search/spec.md))
deliberately leaves it out, because that feature adds no new kind of public
data.

## Proposed outcome

1. **A decision on whether `legacy_id` is public**, and under what name in
   the API (for example `parcel_number`).
2. If it is, **parcel reads return it**, and **a search RPC of its own matches
   it**, alongside `SearchParcelsAddresses`, whose name limits it to
   addresses. Matching is exact or by prefix, after normalisation suited to
   parcel numbers (separators and leading zeros). It can reuse
   `core.search_normalize` or need one of its own.

## Constraints

- Adds to the API without breaking anything: a new RPC and new optional
  fields only.
- Only synthetic parcel numbers (`FX-PARCEL-<n>`) in fixtures, docs and tests.
- `legacy_id` is on `parcels` and has no history, so `legal_as_of` doesn't apply
  to it. Any normalised index on it needs a migration and `mise run db:validate`.

## Open questions (carry into spec)

- Is the parcel number public in every deployment, or does it need a
  per-deployment switch?
- How are parcel numbers formatted across jurisdictions, and what
  normalisation makes `12-345-67`, `1234567` and `012-345-67` match?
- Does the records app need one search box across addresses and parcel
  numbers? If so, does it call both RPCs and merge, or is a combined RPC
  worth adding?
