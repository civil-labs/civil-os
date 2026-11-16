# Spec: check public API breaking changes against the PR's base

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-16 |
| **Status** | Decisions signed off by the owner on 2026-11-16. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the behaviour: what `buf breaking` compares against, when
a break is allowed, what the release PR checks in addition, and how it's
validated. The order of the work goes in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| The baseline | **The first parent of the checked-out merge commit**, not `github.event.pull_request.base.sha`. On `pull_request`, `actions/checkout` checks out `refs/pull/N/merge`, which is the PR's head merged into the base branch's current tip, and `HEAD^1` is that tip. Comparing `HEAD` with `HEAD^1` checks exactly the change this PR would make to the base. `base.sha` in the event payload can lag behind the base tip that the merge ref was built from: the intent's `edited` re-runs are a case where it does. If it lags, any break merged to `main` in between would be blamed on this PR, which is the bug this work fixes. |
| Release-time check | **Yes, on the release-please PR, as a second step in the same job.** When the PR's head branch starts with `release-please--`, the job also runs `buf breaking` against the last `public-api-v*` tag. If that finds a break, the public API's new version in `.release-please-manifest.json` must be a breaking bump over the tag: the major version before 1.0 is 0, so the minor must rise; from 1.0, the major must rise. Otherwise the job fails and tells the owner how to correct the release (below). With no tag yet, the step is skipped with a notice. This catches a break merged without `!`, whether the per-PR check was skipped, failed and was merged anyway (it isn't required today), or the PR was merged by an admin bypass. *Amended 2026-11-16, while planning:* the step runs on every release-please PR, whatever the public-API scope step reports. A release of the root package alone doesn't touch `api/proto/public/`, so a scope-gated step would miss a break merged under a title release-please ignores (for example `refactor:`), which never reaches a public API release. |
| Merge-group and push events | **No new triggers.** There's no merge queue, and `push` to `main` has no PR title to read. If a merge queue is added, its `merge_group` trigger needs the same `HEAD^1` baseline (a merge-group commit's first parent is the queue's base) and a way to read each grouped PR's title. That's work for the merge-queue change, and it's recorded in a workflow comment. |
| Edited titles | **They still work.** `edited` re-runs the job. The merge ref, and so `HEAD^1`, is the current base, not the base at the time the PR was opened, so fixing a title re-checks against the right baseline. A base-branch change also arrives as `edited` and re-checks against the new base. A body-only edit re-runs the check too; that's cheap and harmless. |
| Required check | **Ask the owner to add `Public API breaking changes` to the `Main Squash Only` ruleset** once this lands and has passed on a few unrelated PRs. With the baseline fixed, it's no longer red for unrelated reasons, and `docs/github-setup.md` already lists it as required. The ruleset is outside the repo, so the owner makes this change. |

## Behaviour

### Per-PR check (every PR)

The job's existing steps don't change up to and including the public-API
scope step (`scripts/touches-public-api.sh`, from
[`../docs-only-ci/spec.md`](../docs-only-ci/spec.md)). When the scope is
`false`, the job prints its notice and succeeds, as today.

Otherwise, in `api/`:

```sh
base=$(git rev-parse HEAD^1)
echo "Comparing against the PR's base, $base"
buf breaking proto/public --against "../.git#ref=${base},subdir=api/proto/public"
```

- **No break:** the step passes.
- **A break, title marked breaking** (`type!:` or `type(scope)!:`, the same
  regex as today): the step passes with a `::warning::`.
- **A break, title not marked:** the step fails with the existing `::error::`
  message.
- **`HEAD` isn't a merge commit** (no `HEAD^1`, or `HEAD^2` is missing): the
  step fails with an error naming the problem. A check that can't find its
  baseline must not pass. This shouldn't happen on `pull_request`, where
  GitHub only runs the workflow when the merge ref exists.

The title stays in the `PR_TITLE` environment variable and is never
interpolated into the script. `fetch-depth: 0` stays: the release-time step
needs the tags, and the per-PR step needs the base commit, which a full fetch
includes.

The "no public-api release yet" skip goes away from the per-PR step. It
belonged to the tag baseline, and a base commit always exists.

### Release-time check (release-please PRs only)

Runs after the per-PR step, only when `github.head_ref` starts with
`release-please--`, and **whatever the scope step reports**. A release PR
touches `api/proto/public/CHANGELOG.md` only when the public API is part of
the release. A root-only release doesn't, and a break merged under a title
release-please ignores would then never be checked. With the step always
running, that release fails: the break is found, and the public API version
hasn't moved. The `buf` setup runs whenever either check needs it.

1. Find the last tag:
   `git describe --tags --abbrev=0 --match 'public-api-v*' HEAD^1`. No tag:
   print a notice and pass.
2. `buf breaking proto/public --against "../.git#tag=${tag},subdir=api/proto/public"`.
   No break: pass.
3. If there's a break, read the new version, `.["api/proto/public"]` in
   `.release-please-manifest.json` from `HEAD`, and compare it with the tag's
   version:
   - old major `0`: pass if the new minor is greater;
   - old major `1` or more: pass if the new major is greater;
   - otherwise fail with:
     `::error::buf breaking finds a breaking change since $tag, but this release is not a breaking version bump. A breaking change was merged without a breaking PR title. Add a commit to main with a "Release-As: X.Y.Z" footer, or a "fix!:" commit describing the break, and let release-please update this PR.`

The version comparison is a small POSIX `sh` script,
`scripts/is-breaking-bump.sh OLD NEW`, which prints `true` or `false`, with a
table test `scripts/is-breaking-bump_test.sh` that the job runs before the
script, as `changed-code_test.sh` does.

The release-time step doesn't read the PR title: release-please writes
`chore: release main`, which is never marked breaking.

### Workflow comments

The header comment drops "against the last public-api release (spec §4.3)"
and describes both comparisons, citing this spec. A comment by the trigger
notes the `merge_group` follow-up above.

## Version bump cases

`scripts/is-breaking-bump_test.sh` covers at least:

| Old | New | Expected |
|---|---|---|
| `0.2.0` | `0.3.0` | `true` |
| `0.2.0` | `0.2.1` | `false` |
| `0.2.0` | `1.0.0` | `true` |
| `1.4.2` | `2.0.0` | `true` |
| `1.4.2` | `1.5.0` | `false` |
| `0.9.0` | `0.10.0` | `true` (numeric, not string, comparison) |
| `0.2.0` | `0.2.0` | `false` |
| `0.2.0` | `garbage` | non-zero exit |

## Tests

Workflow logic can't run locally, so it's validated on throwaway PRs after
the change merges, all closed without merging. To put a break on a base
without touching `main`, use a throwaway base branch: `test/bcb-base`, cut
from `main`, with one commit deleting a public proto field. The workflow has
no branch filter, so PRs into that branch run it.

| Case | Expected |
|---|---|
| PR into `test/bcb-base` changing only a Go comment | Scope `false`, skipped, passes. |
| PR into `test/bcb-base` adding a comment to a public proto, title `docs: ...` | `buf breaking` runs against the base, which already has the break, and passes. Under the old workflow this PR fails, which is the bug, and the re-run of #22's case from the intent. |
| PR into `main` deleting a public field, title `feat: ...` | Fails. |
| Same PR, title edited to `feat!: ...` | Re-runs on `edited` and passes with a warning. |
| The next real release-please PR | Per-PR step passes; release-time step runs against the last tag and passes. |
| Release-time failure | Simulated with a throwaway PR whose head branch is named `release-please--test`, carrying a public field deletion and a manifest bump of the patch version only: the release-time step fails with the error above. |

The intent's criterion "re-running the check on #22's head passes" can't be
met literally: re-running #22's old workflow run uses the old workflow file.
The second row reproduces its situation instead.

## Docs to update

- `docs/github-setup.md`: drop the note that the check isn't in the live
  ruleset and points to this intent, and say the owner should add it to the
  required checks.
- `docs/monorepo-migration/spec.md` §4.3 says the check runs "against the last
  `public-api-v*` tag". Add a dated amendment pointing to this spec, rather
  than rewriting an accepted spec.

## Out of scope

A merge queue, checking the mesh module, changing the title regex or
`pr-title.yml`, publishing changes in `public-api-release.yml`, and making the
check required (an owner action on the ruleset).
