# Plan: skip the code CI jobs on docs-only pull requests

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `5bd621e`, #26), which implements [`intent.md`](intent.md) (`6298c6f`, #18) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-15 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One implementation PR** (`ci/docs-only-skip`, title
  `ci: skip the code jobs on docs-only PRs`), then a short series of
  throwaway PRs to check the workflow on GitHub. The agent never pushes to
  `main`; the owner merges.
- **Each step is one commit** with a conventional-commit message.
- **Plan and code stay in sync.** A departure from this plan is recorded here
  in the same commit. A departure from the spec stops the work until the owner
  agrees.
- **Throwaway PRs are drafts**, titled `test(ci): ... (do not merge)`, and are
  closed without merging once checked. Their branches are deleted.
- **`ci:` doesn't trigger a release.**

## Spec amendment (in this PR)

The owner chose, while this plan was drafted, to skip the breaking check at the
**job level** instead of with a workflow-level `paths` filter:

- **Why:** `docs/github-setup.md` lists `Public API breaking changes` as a
  required check, but the live `Main Squash Only` ruleset doesn't require it
  (checked 2026-11-15). A workflow-level `paths` filter is only safe while the
  check isn't required: a required check that never runs leaves the PR
  pending forever. A job-level skip is safe either way.
- **Side effect:** without a `paths` filter, `breaking.yml` runs on every PR,
  so the spec's "to verify" about `paths` on the `edited` event no longer
  applies. Editing a title always re-runs the check, as today.
- `spec.md` is updated in the same commit as this plan (the decision row, the
  `breaking.yml` section and the workflow test row).

The doc and ruleset disagreement itself isn't resolved here. It belongs to
[`../breaking-check-baseline/intent.md`](../breaking-check-baseline/intent.md),
which changes the same check.

## Implementation PR (`ci/docs-only-skip`)

**1. Classification scripts and their test** (`ci: add changed-file classification scripts`)

- `scripts/changed-code.sh`: reads paths on stdin and prints `true` if any
  path is code, `false` if all are docs (`^docs/` or `\.md$`, case-sensitive).
  An empty list prints `true`.
- `scripts/touches-public-api.sh`: the same interface. It prints `true` if any
  path is `api/proto/public/...`, `api/buf.yaml` or `api/buf.lock`, or if the
  list is empty. It prints `false` otherwise.
- Both are POSIX `sh` with `set -eu`, using only `grep`. A missing or unreadable
  stdin is an error, not `false`.
- `scripts/changed-code_test.sh`: runs every case in the spec's
  classification table through `changed-code.sh`, plus a table for
  `touches-public-api.sh`:

  | Paths | Expected |
  |---|---|
  | `api/proto/public/civil/public/parcels/v1/parcels.proto` | `true` |
  | `api/buf.yaml` | `true` |
  | `api/buf.lock` | `true` |
  | `api/proto/mesh/civil/mesh/parcels/v1/parcels.proto` | `false` |
  | `api/buf.gen.public.yaml` | `false` |
  | `services/gateway/main.go`, `docs/x.md` | `false` |
  | `api/proto/public/README.md` | `true` (a README change can't break anything, but erring towards running is cheap) |
  | (empty) | `true` |

  It prints one line per failing case and exits non-zero if any case fails.
- **Local checks:** `sh scripts/changed-code_test.sh` passes, and
  `sh -n` on all three files is clean. To prove the test can fail, swap the
  output of one rule and confirm the test fails, then restore it.

**2. `ci.yml`** (`ci: skip the code jobs on docs-only PRs`)

- **New job `changes`** (name `Detect docs-only changes`) with
  `permissions: {contents: read, pull-requests: read}`. Its steps:
  1. `actions/checkout`, pinned to the same SHA as the other jobs.
  2. `sh scripts/changed-code_test.sh`.
  3. Classify, with `GH_TOKEN`, the PR number and `changed_files` passed via
     `env`, never interpolated into the script:
     - if the event isn't `pull_request`, output `code=true`;
     - else, if `changed_files >= 3000`, output `code=true`;
     - else, list the files with
       `gh api --paginate "repos/$GITHUB_REPOSITORY/pulls/$PR/files" --jq '.[] | .filename, (.previous_filename // empty)'`,
       pipe them to `scripts/changed-code.sh`, and write the result as `code`.
     - The step uses `set -eu` (and `pipefail` under `bash`), so a failed
       `gh api` call fails the job.
- **The four code jobs** (`go`, `proto`, `sdk`, `e2e`) get `needs: changes`
  and `if: ${{ !cancelled() && needs.changes.outputs.code != 'false' }}`.
  Their names and steps don't change.
- **`customer-data`** is unchanged: no `needs`, always runs.

**3. `breaking.yml`** (`ci: skip the breaking check when no public proto changes`)

- In the existing `buf-breaking` job, before the `mise` setup, add a step
  `id: scope` that lists the PR's files the same way and pipes them to
  `scripts/touches-public-api.sh`. It writes `public=true|false`. With
  `changed_files >= 3000` it writes `public=true` without listing.
- The job needs `pull-requests: read` for the files API. Add it at the job
  level next to `contents: read`.
- The `mise` and `buf breaking` steps get
  `if: steps.scope.outputs.public != 'false'`. If the scope step fails, the
  job fails, which is visible and doesn't skip anything silently.
- When skipped, a final step prints a notice:
  `::notice::No public API files changed; buf breaking skipped`.
- The workflow's `on:` block is unchanged (no `paths`), so the check runs and
  reports on every PR, and re-runs when the title is edited.

**4. Docs** (`docs: note the docs-only CI skip`)

- `CLAUDE.md`, under "CI also runs these": a line saying PRs that only touch
  `docs/` or `*.md` skip the Go, protos, SDK and e2e jobs.
- `docs/github-setup.md`, under the required checks:
  - `Detect docs-only changes` is deliberately **not** required. If it fails,
    the code jobs run anyway.
  - The breaking check skips itself when no public API file changed, so it's
    safe to require or not.
  - Leave the existing list as it is, including `Public API breaking changes`.
    Reconciling it with the live ruleset belongs to the breaking-check-baseline
    feature.

**Checks before opening the PR:**
- `sh scripts/changed-code_test.sh`.
- `actionlint` on both workflows, run with a pinned version through
  `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7`. It isn't
  installed locally, and `go run` needs no install.
- `mise run check` (unaffected, but cheap).

This PR changes `.github/` and `scripts/`, so it isn't docs-only: its own CI
runs every job, which is the first check that code PRs still run everything.

## Verification on GitHub (after the implementation PR merges)

Workflow logic can't run locally, so each spec test row is checked on GitHub.
The results go in a comment on the implementation PR.

| # | How | Expected |
|---|---|---|
| V1 | The merge of the implementation PR (push to `main`) | `changes` outputs `code=true`, and every job runs. |
| V2 | Draft PR changing only `docs/docs-only-ci/plan.md` (a one-line note, then closed) | The four code jobs show as **skipped**, `No customer-specific data` and `conventional-title` run, and GitHub shows the PR as mergeable. `Public API breaking changes` passes with the "skipped" notice. |
| V3 | Draft PR changing one comment in a Go file | Every code job runs. `Public API breaking changes` skips `buf breaking`. |
| V4 | Draft PR renaming `scripts/check-denylist.sh` into `docs/` | Every code job runs, because the old name is code. (It fails `customer-data`, which is expected and irrelevant.) |
| V5 | Draft PR changing one comment in a public proto, regenerated | Every code job runs, and `buf breaking` runs and passes. |
| V6 | Draft PR adding `exit 1` to the classify step | `Detect docs-only changes` fails, and the four code jobs still **run**. This is the fail-safe. |

V2–V6 are closed unmerged, and their branches are deleted. Any unexpected
result stops the verification, and the fix goes in a follow-up `ci:` PR.

## Risks

| Risk | Mitigation |
|---|---|
| A broken `changes` job skips tests. | The condition is `!cancelled() && ... != 'false'`, so any failure runs the code jobs. V6 checks it. |
| The rule calls code "docs" (for example a future `go:embed` of a `.md` file). | The rule only skips for `docs/` and `.md`, and the spec's docs note in `CLAUDE.md` says so. A new embed of Markdown would need the rule narrowed. Note it in the PR description. |
| `GITHUB_TOKEN` lacks `pull-requests: read` on fork PRs. | The repo takes PRs from branches. On a fork, the files call fails and the code jobs run (fail-safe). |
| Re-runs of a skipped job after new commits. | Each push re-runs the whole workflow, including `changes`, so a docs PR that gains a code commit runs everything. |
| The breaking-check-baseline change edits `breaking.yml` too. | Whichever lands second rebases. The step added here is independent of the `--against` baseline. |

## Out of scope

As in the spec: skipping on push to `main`, a merge queue, the breaking-check
baseline, speeding up the code jobs, skipping by other kinds of change, and
reconciling `docs/github-setup.md` with the live ruleset.

<!-- V2 check: docs-only PR. -->
