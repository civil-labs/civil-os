# Plan: check public API breaking changes against the PR's base

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `e68a7da`, #36), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-16 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One implementation PR** (`ci/breaking-check-baseline`, title
  `ci: check public API breaking changes against the PR's base`), then a
  short series of throwaway PRs to check the workflow on GitHub. The agent
  never pushes to `main`. The owner merges.
- **Each step is one commit** with a conventional-commit message.
- **Plan and code stay in sync.** A departure from this plan is recorded here
  in the same commit. A departure from the spec stops the work until the owner
  agrees.
- **Throwaway PRs are drafts**, titled `test(ci): ... (do not merge)` unless
  a case needs a different title, and are closed without merging once
  checked. Their branches, and the scratch base branch, are deleted.
- **`ci:` doesn't trigger a release.**

## Clarification of the spec (recorded here, no spec change)

The spec says release PRs "change `api/proto/public/CHANGELOG.md`, so the
scope step already reports `true` for them". That's only true when the public
API is part of the release. A release of the root package alone touches
`CHANGELOG.md`, `.release-please-manifest.json` and `sdk/js/package.json`, and
the scope step would report `false`.

The release-time step therefore runs on **every** release-please PR,
whatever the scope step says. Otherwise, a break merged under a title that
release-please ignores (for example `refactor:`) would never reach a public
API release, and the release-time check would never see it. With the step
always running, that release PR fails: the break is found, and the public API
version hasn't moved. That outcome is what the spec intends. The `mise` setup
step runs when either check needs `buf`.

## Implementation PR (`ci/breaking-check-baseline`)

**1. Version-bump script and its test** (`ci: add a breaking-version-bump check script`)

- `scripts/is-breaking-bump.sh OLD NEW`: POSIX `sh` with `set -eu`. It prints
  `true` when NEW is a breaking bump over OLD and `false` otherwise:
  - old major `0`: `true` if the new major is greater, or the majors are equal
    and the new minor is greater;
  - old major `1` or more: `true` if the new major is greater.
  - Both arguments must match `^[0-9]+\.[0-9]+\.[0-9]+$`. Anything else, or
    the wrong number of arguments, exits `2` with a message on stderr.
    Pre-release suffixes aren't used by this repo's release-please config, so
    they're rejected rather than guessed at.
  - Compares with shell arithmetic (`-gt`), so `0.10.0` beats `0.9.0`.
- `scripts/is-breaking-bump_test.sh`: every row of the spec's version bump
  table, plus `1.0.0 → 1.0.1` (`false`), a missing argument (exit `2`), and a
  pre-release `0.3.0-rc.1` (exit `2`). It follows the shape of
  `changed-code_test.sh`: one line per failing case, exits non-zero if any
  fail.
- **Local checks:** `sh scripts/is-breaking-bump_test.sh` passes, and `sh -n`
  on both files is clean. To prove the test can fail, flip one comparison and
  confirm the test fails, then restore it.

**2. Per-PR baseline** (`ci: compare buf breaking against the PR's base`)

In `.github/workflows/breaking.yml`, the step
`buf breaking against the last public-api release` becomes
`buf breaking against the PR's base`:

```sh
set -euo pipefail
# The checkout is refs/pull/N/merge: HEAD^1 is the base tip it was built from.
if ! base=$(git rev-parse --verify --quiet 'HEAD^1') \
   || ! git rev-parse --verify --quiet 'HEAD^2' >/dev/null; then
  echo "::error::HEAD is not a merge commit, so the PR's base can't be found."
  exit 1
fi
echo "Comparing against the PR's base, $base"
if buf breaking proto/public --against "../.git#ref=${base},subdir=api/proto/public"; then
  exit 0
fi
# (title check unchanged)
```

- The tag lookup and the "no public-api release yet" skip are removed from
  this step.
- `buf breaking`'s exit status is captured with `if`, so `set -e` doesn't end
  the script before the title check.
- `PR_TITLE` stays in `env`. The regex, `::warning::` and `::error::` messages
  don't change.
- `fetch-depth: 0` stays. Its comment changes to say the tags are for the
  release-time check.
- The header comment describes both comparisons and cites this spec, replacing
  "(spec §4.3)". A comment by `on:` notes that a future `merge_group` trigger
  needs the same `HEAD^1` baseline and a way to read each queued PR's title.
- Already checked locally (2026-11-16): `buf breaking` accepts a full commit
  SHA as `ref=`. It passes between two recent commits, and finds #21's
  deletions against the `public-api-v0.1.0` commit (exit 100).

**3. Release-time check** (`ci: check release PRs against the last public-api release`)

In the same job:

- A step `id: release` sets `release=true` when `GITHUB_HEAD_REF` (read from
  the environment, never interpolated) starts with `release-please--`, else
  `false`. Today the branch is `release-please--branches--main` (#22).
- The `mise` setup step's condition becomes
  `steps.scope.outputs.public != 'false' || steps.release.outputs.release == 'true'`.
- A new final step, `buf breaking against the last public-api release`, with
  `if: steps.release.outputs.release == 'true'`, working directory `api/`:
  1. `sh ../scripts/is-breaking-bump_test.sh`.
  2. Find the tag with
     `git describe --tags --abbrev=0 --match 'public-api-v*' HEAD^1`. If there
     isn't one, print `::notice::No public-api release yet; skipping the
     release-time check` and pass.
  3. Run `buf breaking proto/public --against "../.git#tag=${tag},subdir=api/proto/public"`.
     If it passes, the step passes.
  4. If it fails, read
     `new=$(jq -r '.["api/proto/public"]' ../.release-please-manifest.json)`
     (`jq` is on the runner image) and `old=${tag#public-api-v}`. Then run
     `sh ../scripts/is-breaking-bump.sh "$old" "$new"`. On `true`, print a
     `::notice::` naming the breaking bump and pass. On `false`, fail with the
     spec's `::error::` message. On a script error, fail.
- This step doesn't read `PR_TITLE`.
- Step order: the per-PR step runs first, as today. If the per-PR step fails,
  the release-time step is skipped. That's acceptable, since the job has
  already failed.

**4. Docs** (`docs: point the public API breaking check at the PR's base`)

- `docs/github-setup.md`: replace the parenthetical "It isn't in the live
  ruleset at the moment; see …intent.md" with a note that the check compares
  against the PR's base. Also say the owner should add it to the required
  checks once it has run green on a few PRs.
- `docs/monorepo-migration/spec.md` §4.3: after the `buf breaking` bullet,
  add a dated note: *Amended 2026-11-16: per-PR checks now compare against
  the PR's base, and release PRs also compare against the last tag. See
  [`../breaking-check-baseline/spec.md`](../breaking-check-baseline/spec.md).*

**Checks before opening the PR:**
- `sh scripts/is-breaking-bump_test.sh` and `sh scripts/changed-code_test.sh`.
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7` on
  `breaking.yml` (it isn't installed locally, so `go run` fetches it).
- Locally, the per-PR command on a hand-made merge commit: in a scratch clone,
  merge a branch that deletes a public field into `main` with `--no-ff`, then
  run the step's script body with `PR_TITLE` set to a plain title and then to
  a `feat!:` title. The first exits 1, the second exits 0 with the warning.
- `mise run check` (unaffected, but cheap).

The implementation PR's own run is the first real check: it changes no public
proto, so the scope step reports `false` and the job passes.

## Verification on GitHub (after the implementation PR merges)

The spec's test rows, run as draft PRs. Results go in a comment on the
implementation PR.

Setup: push `test/bcb-base`, cut from `main`, with one commit that deletes a
field from a public proto message and regenerates. It is a branch, not a PR.

| # | How | Expected |
|---|---|---|
| V1 | Draft PR into `test/bcb-base`, one Go comment changed | The scope step reports `false`, `buf breaking` is skipped, and the check passes. |
| V2 | Draft PR into `test/bcb-base`, one comment added to a public proto, title `test(ci): ... (do not merge)` | `Comparing against the PR's base, <sha of test/bcb-base>`, and the check passes despite the break already on the base. This reproduces #22's situation. |
| V3 | Draft PR into `main` deleting a public field, title `test(ci): ... (do not merge)` | Fails with the `::error::`. |
| V4 | V3 with its title edited to `test(ci)!: ... (do not merge)` | Re-runs on `edited` and passes with the `::warning::`. |
| V5 | Draft PR into `main` from a branch named `release-please--test`: a public field deletion, plus the public API version in `.release-please-manifest.json` bumped by patch only, titled `test(ci)!: ... (do not merge)` so the per-PR step passes | The release-time step runs against the latest tag, finds the break, and fails with the release-time `::error::`. |
| V6 | V5 with the manifest bumped by minor instead | The release-time step passes with its `::notice::`. |
| V7 | The next real release-please PR | Both steps run and pass. |

V1–V6 are closed unmerged. Their branches and `test/bcb-base` are deleted
afterwards. V7 is recorded when it happens. Any unexpected result stops the
verification, and the fix goes in a follow-up `ci:` PR.

`release-please--test` doesn't collide with the bot: release-please only
looks for `release-please--branches--main`.

## Risks

| Risk | Mitigation |
|---|---|
| A merge ref whose first parent isn't the base. | GitHub builds `refs/pull/N/merge` with the base as first parent. The step checks that `HEAD^2` exists, and V2 confirms the parent in its log line. |
| `base.sha` vs `HEAD^1` confusion in a later edit. | The comment on the step says why it uses `HEAD^1`. |
| A break merged unmarked under a type that release-please ignores. | The release-time step runs on every release PR (the clarification above). Making the check required closes the gap at the PR. |
| The release-time error blocks a release the owner wants out. | The check isn't required yet. Once it is, the fix is a `Release-As:` footer or a `fix!:` commit, as the error says. |
| `jq` missing on a future runner image. | The step fails visibly instead of passing. |
| Fork PRs. | Unchanged from today: the repo takes PRs from branches. |

## Out of scope

As in the spec: a merge queue, the mesh module, the title regex or
`pr-title.yml`, `public-api-release.yml`, and editing the ruleset (an owner
action).
