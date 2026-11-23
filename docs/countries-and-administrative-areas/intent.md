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
`ZZ-FX` "Fixture Province", in `database/fixtures/synthetic.sql`.

The real rows come from outside civil-os. The data importers seed both tables
themselves, with fixed IDs and `INSERT … ON CONFLICT (<id>) DO NOTHING`:

- countries: 194 sovereign states, with ISO 3166-1 alpha-2 codes and short
  names (`Czechia`, `Russia`, `Taiwan`), as IDs 1–194;
- administrative areas: the 50 US states and 5 territories, as IDs 1–55.

That causes four problems:

- **ID collisions.** The fixtures insert `ZZ` and `ZZ-FX` through the identity
  columns, so both take ID 1. An importer that then inserts its own row 1
  skips it without an error: the first country in its list, and Alabama. Every
  address in that country or state then has no country or administrative
  area. The compose stack shows this today.
- **Missing administrative areas.** The importers' list has no District of
  Columbia (`DC`) and no Armed Forces mail codes (`AA`, `AE`, `AP`). US
  mailing addresses with those states are real (`AA`, `AE` and `AP` are APO,
  FPO and DPO military and diplomatic mail), and an import resolves them to no
  administrative area. The importers' list is being extended with the four
  codes, but civil-os still has no rows of its own.
- **Missing countries and names.** Mailing addresses abroad name places that
  aren't sovereign states, such as Hong Kong, Puerto Rico, Bermuda, the Cayman
  Islands and Curaçao, so they match no country. Others use a formal or older
  name, such as "Czech Republic", "Russian Federation", "Korea, Republic of"
  or "Taiwan, Province of China", which doesn't equal the stored short name.
  Both kinds are left without a country.
- **No single owner.** Whether a database has countries or states at all
  depends on whether an importer has run against it. An empty environment, or
  one loaded by a different importer, has none, and nothing in civil-os says
  which codes should exist, what they're called, or what their IDs are.

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
   former name, so an importer doesn't need its own alias list.
4. **Every environment has the same rows** after migrating, whether or not an
   importer has run, including the compose stack and the e2e database.
5. **The fixtures stop colliding with real rows.** `ZZ` and `ZZ-FX` keep
   working for the tests, but no longer take an ID a real country or area
   needs.

## Constraints

- **Existing databases keep their rows and IDs.** Environments an importer
  has already loaded have countries 1–194 and administrative areas 1–55 (and
  56–59 once the importer adds `DC`, `AA`, `AE` and `AP` in that order), all
  referenced by `address_attributes`. The migration must not renumber or
  duplicate them. New countries take IDs after 194.
- **Importers keep working.** Their seeding must stay safe to run against a
  database the migration has seeded, until they rely on the schema instead.
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
- An importer's existing seeding, run against it, inserts nothing and changes
  nothing.
- After an import, no address is left without a country because its country
  is a dependent territory or is written under an alternative name the schema
  lists.
- After an import, no US mailing address in `AL`, `DC`, `AA`, `AE` or `AP` is
  left without an administrative area.
- The synthetic fixtures, the integration tests and the e2e tests still pass.

## Open questions (carry into spec)

- **Fixed IDs or lookup by code.** Should the migration insert the importers'
  IDs (countries 1–194, areas 1–59), so existing databases and importers
  agree, or should IDs stay generated and everything resolve by `code` only?
  Fixed IDs need the identity sequences moved past the reserved range.
- **How Atlas carries the data.** The schema is HCL and the migrations are
  generated. Does the seed go in as a hand-reviewed data migration the owner
  adds, or does Atlas need another mechanism for reference rows?
- **Where alternative country names live.** A separate alias table, an array
  column on `countries`, or matching rules in the importers? Which
  alternatives are in scope: the ISO formal names, former ISO names, or any
  name seen in real addresses?
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
- **Who owns the lists afterwards.** Once civil-os seeds these rows, should
  importers stop seeding them, or keep checking that the rows they expect
  exist?
