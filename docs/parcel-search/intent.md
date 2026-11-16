# Intent: search parcels by address and owner

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Draft, spawned by [`../address-party-read-api/spec.md`](../address-party-read-api/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The public API has no way to find a parcel without already holding an ID. A
client can only start from:

- a `parcel_id` or `feature_id` it already has;
- a parcel picked on the map from the vector tiles;
- a polygon (`GetSalesComparables`, `GetEquityComparables`);
- a link from another record: a transfer, or, with the address and party read
  API, an address or a party.

The records app's most common starting point is a person typing what they
know: a street address, part of one, or an owner's name. None of the lookups
by ID can serve that, and the address and party read API deliberately doesn't
grow into it (its spec, decision "Parcel search").

## Proposed outcome

1. **A search RPC** that takes a free-text query and returns matching parcels,
   ranked, with enough on each result to show a list: the parcel ID, the
   matched address or owner name, and what matched.
2. **It searches at least:**
   - parcels' current formatted addresses;
   - current owners' names.
3. **It follows the existing patterns:**
   - only visible parcels, under the parcel visibility rule;
   - attributes at `legal_as_of`, defaulting to now;
   - cursor pagination;
   - `NO_SIDE_EFFECTS`.
4. **Fixtures and tests** cover:
   - partial matches;
   - case and punctuation;
   - an owner with several parcels;
   - a voided party or address that must not match.

## Constraints

- **Read-only, and it adds to the API without breaking anything.**
- **No new kind of data:** addresses and owner names are already public.
- **The database does the matching.** It needs indexes suited to partial text
  (for example `pg_trgm` GIN indexes), which means a migration, an extension
  check against what PostGIS deployments allow, and `mise run db:validate`.
- **Only synthetic data**, including the search examples in docs and tests.

## Success criteria

- Typing part of a fixture address, or an owner name, returns the right
  parcels first.
- A voided parcel, party or address never appears.
- On a jurisdiction-sized dataset, a search returns within an agreed latency
  budget.

## Open questions (carry into spec)

- **Scope of results.** Parcels only, or also addresses and parties as results
  in their own right (a search across record types)?
- **Matching.** Trigram similarity, full-text search, prefix matching, or a
  mix? How are ranking ties broken, and how is ranked pagination kept stable?
- **Other keys.** Should it also match `legacy_id` (parcel numbers as
  jurisdictions know them), and is that public?
- **Address normalisation.** Should abbreviations (`St` / `Street`) and unit
  numbers be normalised, or left to the matching?
- **Extension availability.** Is `pg_trgm` available in every deployment's
  PostGIS image? If not, what's the fallback?
- **Latency budget.** What's acceptable, and on what dataset size is it
  measured?
