# Intent: request IDs for safe retries (AIP-155)

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-27 |
| **Status** | Draft, spawned by [`../address-writes/spec.md`](../address-writes/spec.md) and written in its PR. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

A write whose response is lost can't be retried safely. The client can't
tell whether it was applied. `CreateAddress` would make a second address,
and later creates for other records have the same problem. The other address
writes fail harmlessly on a repeat, but the client gets an error instead of
the original result.

## Proposed outcome

Implement [AIP-155](https://google.aip.dev/155) in full for every public
write RPC: the address writes, `UpdateParcel` (or `CorrectParcel`), and every
later write.

- Every write request gains an optional `string request_id`, a UUID chosen
  by the client and reused for its retries.
- A request with a `request_id` already seen gets the first request's
  response, and nothing is written again.
- The same `request_id` with a different request is an error.
- How long a `request_id` is remembered is documented on the API.

## Questions for the spec

- **Where the record lives.** Proposed: a table in the database, written by
  the database-writer in the same transaction as the write. A write and its
  record then commit or roll back together. The gateway has no state.
- **What is stored:** the RPC, a hash of the request, the response, and when
  it was made. Is it keyed per caller (the token's subject) or global?
- **Retention** (24 hours? 7 days?) and what deletes expired rows, given
  that there's no scheduler in the stack today.
- **A duplicate that arrives while the first is still running.** Wait for
  the first, or return `Aborted` so the client retries?
- **Error responses.** Does a failed first attempt get remembered, or only
  successes? AIP-155 only requires that a repeat of a success returns the
  same response.
- **The error code for a reused ID with a different request:**
  `InvalidArgument` or `AlreadyExists`?
- **Whether `request_id` is required** on creates, or optional everywhere.
- **Schema change:** a new table, so the owner generates the migration
  ([`atlas-migrations` skill](../../.claude/skills/atlas-migrations/SKILL.md)).

## Out of scope

- Client-chosen record IDs (AIP-133 user-specified IDs). Request IDs cover
  retries without them.
