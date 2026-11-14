# Intent: one `mise run up` command for the local stack

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from the docs PR (#11) findings at the owner's request, without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

The monorepo spec lists `up` among the `mise` tasks (spec §2 and §9, and the
`CLAUDE.md` command list), but no such task exists. Today, running the stack
means remembering compose flags and the credentials in `dev/dex.yaml`:

- `docker compose up --build --wait`;
- then a long `curl` against Dex to get a token;
- then `docker compose down -v` to reset.

Every other workflow goes through `mise run …`: generate, check, the test
suites and database tasks. The stack is the exception, so people (and agents)
reach for compose by hand, forget `--build`, and end up testing stale images.

## Proposed outcome

1. **`mise run up`** builds and starts the full compose stack, waits until it's
   healthy, and prints the useful URLs: the gateway, Dex, and the test user.
2. **`mise run down`** stops the stack and deletes its data, for a clean
   reset.
3. **A way to get a token for the local test user** without copying the curl
   command from the README, such as `mise run token`.
4. The README, `CLAUDE.md` and `docs/architecture.md` use these tasks, and
   `mise run test:e2e` says to run `mise run up` first.

## Constraints

- **Plain compose still works.** `docker compose up --build --wait` stays a
  supported quickstart for people without mise. The tasks wrap compose and
  don't replace it.
- **No new tools** beyond what `mise.toml` pins and Docker. In particular, no
  `jq` dependency.
- **Local only.** The tasks never touch a real deployment or use real
  credentials. They use only the throwaway values in `dev/`.
- **No Atlas.** The stack keeps migrating with `dev/migrate.sh`.

## Success criteria

- From a fresh clone: `mise trust && mise install && mise run up`, then
  `mise run test:e2e`, passes.
- `mise run down` leaves no civil-os containers or volumes behind.
- The token task's output works as a bearer token in the README's curl
  examples.
- Every command in the README and `CLAUDE.md` still works as written.

## Open questions (carry into spec)

- **Rebuild behavior.** Should `up` always rebuild (`--build`), or offer a fast
  path that reuses images?
- **Reset vs stop.** Should `down` always delete the volume (`-v`), or should
  there be separate `down` and `reset` tasks?
- **Logs.** Should there be a `mise run logs [service]` task, or is
  `docker compose logs` enough?
- **CI.** Should the e2e CI job switch to `mise run up` so the task is
  exercised, or keep calling compose directly?
