# Intent: pre-load countries and US administrative areas in the schema

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-23 |
| **Status** | Draft. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

`core.countries` and `core.administrative_areas` have no reference data in
civil-os. The migrations create both tables but insert nothing. The only rows
the repo's own tooling writes are the synthetic fixtures' `ZZ` "Testland" and
`ZZ-FX` "Fixture Province", in `database/fixtures/synthetic.sql`. Any real
rows come from outside civil-os, from whatever loads data into the database.

That causes four problems:

- **ID collisions.** The fixtures insert `ZZ` and `ZZ-FX` through the identity
  columns, so both take ID 1. Rows loaded later with their own fixed IDs can
  collide with them and be skipped, leaving every address in that country or
  state without a country or administrative area. The compose stack shows
  this today.
- **Missing administrative areas.** US mailing addresses use more than the 50
  states: the District of Columbia (`DC`), the territories, and the Armed
  Forces mail codes (`AA`, `AE`, `AP`) for APO, FPO and DPO military and
  diplomatic mail. Nothing in civil-os guarantees rows for any of them.
- **Missing countries and names.** Mailing addresses abroad name places that
  aren't sovereign states, such as Hong Kong, Puerto Rico, Bermuda, the Cayman
  Islands and Curaçao. Others use a formal or older name, such as "Czech
  Republic", "Russian Federation", "Korea, Republic of" or "Taiwan, Province
  of China", where a table might hold the short name. Either way, the address
  is left without a country.
- **No single owner.** Whether a database has countries or states at all
  depends on what has been loaded into it. An empty environment has none, and
  nothing in civil-os says which codes should exist, what they're called, or
  what their IDs are.

## Proposed outcome

1. **The schema pre-loads the countries** through a migration: every ISO
   3166-1 entry, including dependent territories such as `HK`, `PR`, `BM`,
   `KY` and `CW`, keyed by its alpha-2 code.
2. **The schema pre-loads the US administrative areas** through a migration:
   - the 50 states;
   - the District of Columbia (`DC`);
   - the 5 inhabited territories (`AS`, `GU`, `MP`, `PR`, `VI`);
   - the Armed Forces mail codes: `AA` (Armed Forces Americas), `AE` (Armed
     Forces Europe) and `AP` (Armed Forces Pacific).

   Codes and names match the USPS state abbreviations, so a mailing address's
   state matches a row by `code`.
3. **A country can be found by the names addresses actually use:** its ISO
   short name and common alternatives, such as the ISO formal name or a
   former name.
4. **Every environment has the same rows** after migrating, whatever has been
   loaded into it, including the compose stack and the e2e database.
5. **The fixtures stop colliding with real rows.** `ZZ` and `ZZ-FX` keep
   working for the tests, but no longer take an ID a real country or area
   needs.

## Constraints

- **Existing databases keep their rows and IDs.** Databases that already hold
  countries or administrative areas reference them from `address_attributes`.
  The migration must not renumber or duplicate them.
- **Reference data only.** No changes to the public API, or to what the
  readers return beyond addresses that now resolve to a country or an
  administrative area.
- **Migrations follow the Atlas rules.** The schema change goes through the
  `atlas-migrations` skill. Agents don't write files in `database/migrations`
  or run Atlas, so the owner generates the migration.
- **No jurisdiction data.** The rows are generic reference data from ISO 3166-1
  and USPS; nothing here names a customer or jurisdiction.

## Success criteria

- A freshly migrated database holds every ISO 3166-1 country and all 59 US
  administrative area codes listed above, with their names.
- Migrating a database that already holds some of these rows adds the missing
  ones without renumbering or duplicating any.
- No address is left without a country because its country is a dependent
  territory or is written under an alternative name the schema lists.
- No US mailing address in a state, `DC`, a territory, `AA`, `AE` or `AP` is
  left without an administrative area.
- The synthetic fixtures, the integration tests and the e2e tests still pass.

## Open questions (carry into spec)

- **Fixed IDs or lookup by code.** Should the migration assign fixed IDs, so
  every database agrees on them, or should IDs stay generated and everything
  resolve by `code` only? Fixed IDs need the identity sequences moved past
  the reserved range, and must match what existing databases already hold.
- **How Atlas carries the data.** The schema is HCL and the migrations are
  generated. Does the seed go in as a hand-reviewed data migration the owner
  adds, or does Atlas need another mechanism for reference rows?
- **Where alternative country names live.** A separate alias table, or an
  array column on `countries`? Which alternatives are in scope: the ISO formal
  names, former ISO names, or any name seen in real addresses?
- **Puerto Rico and the other US territories** are both ISO countries (`PR`,
  `GU`, `VI`, `AS`, `MP`) and US administrative areas. Which does an address
  with country "Puerto Rico" resolve to: the country `PR`, or the US with the
  area `PR`?
- **Unique indexes on `code`.** Nothing stops two rows sharing a code today.
  Should `countries.code` be unique? For administrative areas, does that need
  a country column first, since non-US codes such as Canada's `ON` would share
  the table?
- **The fixtures' IDs.** Should `ZZ` and `ZZ-FX` take IDs outside the reserved
  ranges, or should the fixtures look up their rows by code like everything
  else?
