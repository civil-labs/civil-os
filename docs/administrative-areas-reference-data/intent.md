# Intent: pre-load US administrative areas in the schema

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-23 |
| **Status** | Draft. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

`core.administrative_areas` has no reference data in civil-os. The migrations
create the table but insert nothing. The only row the repo's own tooling
writes is the synthetic fixtures' `ZZ-FX` "Fixture Province", in
`database/fixtures/synthetic.sql`.

The real rows come from outside civil-os: the data importers seed the 50
states and 5 territories themselves, with fixed IDs 1–55, using
`INSERT … ON CONFLICT (administrative_area_id) DO NOTHING`. That causes three
problems:

- **Missing codes.** That list has no District of Columbia (`DC`) and no
  Armed Forces mail codes (`AA`, `AE`, `AP`). US mailing addresses with those
  states are real (`AA`, `AE` and `AP` are APO, FPO and DPO military and
  diplomatic mail), and an import resolves them to no administrative area.
  The importer's list is being extended with the four codes, but civil-os
  still has no rows of its own.
- **ID collisions.** The fixtures insert `ZZ-FX` through the identity column,
  so it takes ID 1. An importer that then inserts Alabama as ID 1 skips it
  without an error, and every `AL` address in that database has no
  administrative area. The compose stack shows this today.
- **No single owner.** Whether a database has states at all depends on
  whether an importer has run against it. An empty environment, or one loaded
  by a different importer, has none, and nothing in civil-os says which codes
  should exist or what their IDs are.

## Proposed outcome

1. **The schema pre-loads the US administrative areas** through a migration:
   - the 50 states;
   - the District of Columbia (`DC`);
   - the 5 inhabited territories (`AS`, `GU`, `MP`, `PR`, `VI`);
   - the Armed Forces mail codes: `AA` (Armed Forces Americas), `AE` (Armed
     Forces Europe) and `AP` (Armed Forces Pacific).
2. **Codes and names match the USPS state abbreviations**, so a mailing
   address's state matches a row by `code`.
3. **Every environment has the same rows** after migrating, whether or not
   an importer has run, including the compose stack and the e2e database.
4. **The fixtures stop colliding with real rows.** `ZZ-FX` keeps working for
   the tests, but no longer takes an ID a real area needs.

## Constraints

- **Existing databases keep their rows and IDs.** Environments an importer
  has already loaded have IDs 1–55 (and 56–59 once the importer adds `DC`,
  `AA`, `AE` and `AP` in that order), referenced by `address_attributes`. The
  migration must not renumber or duplicate them.
- **Importers keep working.** Their seeding must stay safe to run against a
  database the migration has seeded, until they rely on the schema instead.
- **Reference data only.** No changes to the public API, or to what the
  readers return beyond addresses that now resolve to an administrative area.
- **Migrations follow the Atlas rules.** The schema change goes through the
  `atlas-migrations` skill. Agents don't write files in `database/migrations`
  or run Atlas, so the owner generates the migration.
- **No jurisdiction data.** The rows are generic US reference data; nothing
  here names a customer or jurisdiction.

## Success criteria

- A freshly migrated database holds all 59 codes listed above, with their
  names.
- An importer's existing seeding, run against it, inserts nothing and changes
  nothing.
- After an import, no US mailing address in `AL`, `DC`, `AA`, `AE` or `AP` is
  left without an administrative area.
- The synthetic fixtures, the integration tests and the e2e tests still pass.

## Open questions (carry into spec)

- **Fixed IDs or lookup by code.** Should the migration insert the
  importers' IDs (1–59), so existing databases and importers agree, or should
  IDs stay generated and everything resolve areas by `code` only? Fixed IDs
  need the identity sequence moved past 59.
- **How Atlas carries the data.** The schema is HCL and the migrations are
  generated. Does the seed go in as a hand-reviewed data migration the owner
  adds, or does Atlas need another mechanism for reference rows?
- **A unique index on `code`.** Nothing stops two rows sharing a code today.
  Should `code` be unique, and does that need a country column first, since
  non-US codes such as Canada's `ON` would share the table?
- **The fixture's ID.** Should `ZZ-FX` take an ID outside the reserved range,
  or should the fixtures look up their area by code like everything else?
- **Countries too.** `core.countries` has the same gap and the same fixture
  collision (`ZZ` takes ID 1, so an importer's country with ID 1 is skipped).
  Is it part of this intent, or an intent of its own?
- **Who owns the list afterwards.** Once civil-os seeds these rows, should
  importers stop seeding them, or keep checking that the rows they expect
  exist?
