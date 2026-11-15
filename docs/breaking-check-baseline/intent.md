# Intent: check public API breaking changes against the PR's base, not the last release

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft, written at the owner's request after the release PR (#22) failed the breaking-change check. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

`.github/workflows/breaking.yml` runs `buf breaking` on every PR against the
**last `public-api-v*` tag**, and lets a failure pass only when the PR title is
marked breaking (`type!:`).

Once a breaking change merges, `main` differs from the last tag until the next
public API release is tagged. Every later PR builds on `main`, so it inherits
that difference and fails the check, even when it changes no proto at all.

This happened after #21 (`feat!: remove the unserved SalesService and
TileService`) merged. The release-please PR (#22, `chore: release main`)
touches only changelogs, the release manifest and `sdk/js/package.json`, but
failed with #21's four deletions:

```
Comparing against public-api-v0.1.0
Previously present file "civil/public/sales/v1/sales.proto" was deleted.
Previously present file "civil/public/tiles/v1/tiles.proto" was deleted.
Previously present file "google/api/annotations.proto" was deleted.
Previously present file "google/api/http.proto" was deleted.
```

The check isn't required by the `Main Squash Only` ruleset, so it doesn't block
merges. But it stays red on every PR until the release is tagged, and the owner
sometimes holds a release on purpose. During the real property transfers work,
the release is held until that feature's last PR merges. A check that is red
for unrelated reasons trains people to ignore it, and then a real breaking
change can slip through unmarked.

## Proposed outcome

1. **`buf breaking` compares the PR's head against the PR's base commit**
   (`github.event.pull_request.base.sha`), not the last tag. A PR fails only
   when it introduces a breaking change itself.
2. **The title rule is unchanged.** A PR that introduces a breaking change
   passes only when its title is marked breaking, which is what release-please
   reads from the squash commit.
3. **The release PR passes** whenever it contains no proto changes, which is
   always.

## Constraints

- **The title is untrusted input.** It stays in an environment variable and is
  never interpolated into the script, as today.
- **No new third-party actions.** The base commit is already in the checkout
  with `fetch-depth: 0`.
- **The public module only.** The mesh module stays free to change.
- **Pinned actions stay pinned** to commit SHAs.

## Success criteria

- A PR with no proto changes passes, even while `main` has unreleased
  breaking changes.
- A PR that removes or renames a public field, message, service or file fails
  unless its title is marked breaking, and passes when it is.
- The release-please PR passes.
- Re-running the check on #22's head passes.

## Open questions (carry into spec)

- **Release-time check.** Comparing against the base no longer checks the sum
  of changes since the last release. Should the release PR (or the
  `public-api-v*` tag workflow) also compare against the last tag, and verify
  that the release notes list a breaking change whenever `buf breaking` finds
  one? That would catch a breaking change merged without `!` if the per-PR
  check was skipped or overridden.
- **Merge-group and push events.** The workflow only runs on `pull_request`.
  If a merge queue is added later, it needs a `merge_group` trigger with the
  right base.
- **Edited titles.** The workflow re-runs on `edited`, so fixing a title
  re-checks. Confirm that still works with the new baseline.
