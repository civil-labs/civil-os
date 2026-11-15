# Intent: reject negative real property transfer amounts in the database

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft, written during the real property transfers API spec at the owner's request. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

`core.real_property_transfers.transfer_amount` is `numeric(19,4)`, nullable,
with no check on its sign. A negative amount can be stored, although no real
transfer has one: it can only come from a loading or entry error.

Other numeric columns on the same tables are already constrained. For example,
`valid_share_transfer` keeps `transferred_share` in `(0, 1]` on both transfer
party tables. The amount is the exception.

The real property transfers API
([`../real-property-transfers-api/spec.md`](../real-property-transfers-api/spec.md))
returns a negative amount as stored and documents it as a data error, so that
reconciliation can find it. That handles bad rows already in the database, but
nothing stops new ones.

## Proposed outcome

1. **A check constraint on `real_property_transfers.transfer_amount`** rejects
   negative values. `NULL` stays allowed (gifts and nominal transfers with no
   recorded amount).
2. **The change ships in the same migration as the recordings table**, the
   owner's decision, so deployments take one schema change instead of two.
3. **Existing negative rows are found and resolved before the constraint is
   enforced** in each deployment, so the migration doesn't fail on data it
   can't fix.
4. **The transfers API documentation is updated** once the constraint is
   enforced everywhere: `transfer_amount` can no longer be negative.

## Constraints

- **No Atlas in CI or compose.** The migration is generated locally and
  validated with `mise run db:validate`, like every other.
- **Only synthetic data.** Tests use invented amounts. No real transfer
  amounts or counts of bad rows from any deployment appear in this repo.
- **Deployment repos aren't edited from here.** Whatever loads transfers in a
  deployment may start getting constraint violations. Coordinating that
  happens outside this repo.
- **The history table is a record of past states.** Constraining
  `real_property_transfers_history.transfer_amount` could reject rows that
  faithfully record a past bad value.

## Success criteria

- Inserting or updating a transfer with a negative `transfer_amount` fails
  with a check violation. `NULL` and positive amounts still succeed.
- A migration integration test (`mise run test:integration`) covers both.
- The migration applies cleanly to the synthetic fixtures and to a database
  that has had its negative rows resolved.
- The transfers API docs no longer describe negative amounts as possible.

## Open questions (carry into spec)

- **Zero.** Is `0` a valid amount, or must a transfer with no consideration
  use `NULL`? This decides between `>= 0` and `> 0`.
- **Existing rows.** How are negative amounts in a deployment resolved: set to
  `NULL`, corrected from the source records, or the transfer voided? Who does
  it, and does this repo ship a query that finds them?
- **Rollout.** Add the constraint `NOT VALID` and validate it later, so new
  bad rows are blocked before old ones are cleaned up? Check whether Atlas can
  express that, or whether the migration needs hand-written SQL.
- **History table.** Leave `real_property_transfers_history` unconstrained,
  or constrain it too?
- **Other amounts.** Should other money columns without a sign check be
  covered by the same migration?
