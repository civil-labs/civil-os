# Spec: skip the code CI jobs on docs-only pull requests

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft. Decisions marked **Proposed** need the owner's sign-off. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the behaviour: what counts as docs-only, which jobs skip,
how the workflow decides, and how it fails safe. The order of the work goes in
`plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Which jobs skip | **Proposed:** all four code jobs in `ci.yml`: `Go build, vet, unit and integration tests`, `Protos and generated code`, `JS SDK packs and imports` and `E2E (docker compose)`. Nothing they build or test reads Markdown: no `go:embed`, no test fixture, and no script reads a `.md` file (checked on 2026-11-15; the only references are comments). |
| The docs rule | **Proposed:** a file is docs when its path starts with `docs/` or ends in `.md`, including shipped READMEs. `sdk/js/README.md` is packed into the npm tarball, but the SDK job tests packing and importing, not the README, so a README-only change gains nothing from it. `CHANGELOG.md` files are docs too. Release-please PRs still run everything, because they also change `.release-please-manifest.json` and `sdk/js/package.json`. |
| Other workflows | **Proposed:** `breaking.yml` gets a positive `paths` filter: it runs only when `api/proto/public/**`, `api/buf.yaml` or `api/buf.lock` changes. It isn't a required check, so skipping the whole workflow is safe. A positive filter is narrower than "not docs-only": a Go-only PR can't break the public API either. This touches the same file as [`../breaking-check-baseline/intent.md`](../breaking-check-baseline/intent.md); whichever lands second rebases. If the breaking check is ever made required, this filter must move to the job level like the others. |
| Large PRs | **Proposed:** a PR whose `changed_files` count is 3,000 or more is treated as code without listing its files. The pull request files API returns at most 3,000 files, so a longer list can't be checked completely. |

## Behaviour

### The `changes` job

A new job in `ci.yml`, named `Detect docs-only changes`, runs first on every
event:

| Event | Output `code` |
|---|---|
| `push` to `main` | `true`, without looking at files |
| `pull_request` with `changed_files >= 3000` | `true`, without listing files |
| `pull_request` otherwise | `false` if every path is docs, else `true` |

The file list comes from `gh api --paginate repos/{repo}/pulls/{n}/files`.
Each entry contributes `filename` and, for renames, `previous_filename`, so
moving a Go file into `docs/` counts as code.

The job needs `pull-requests: read`, granted at the job level. The workflow's
default stays `contents: read`.

### Classification script

The rule lives in `scripts/changed-code.sh`, not inline YAML, so it can be
tested:

- **Input:** one path per line on stdin.
- **Output:** prints `true` if any path is code, `false` if every path is docs.
  An empty list prints `true` (nothing to prove it's docs-only).
- **Docs:** `^docs/` or `\.md$`, matched case-sensitively. `README.MD` or
  `notes.markdown` count as code. Erring towards running tests is the safe
  direction.
- POSIX `sh`, no dependencies beyond `grep`, so it runs on the runner and
  locally.

`scripts/changed-code_test.sh` runs a table of cases through it (below), and
the `changes` job runs the test before the script, so a broken rule fails the
job instead of silently skipping tests.

### Skipping the code jobs

Each of the four code jobs gets:

```yaml
needs: changes
if: ${{ !cancelled() && needs.changes.outputs.code != 'false' }}
```

- A job skipped by `if:` reports success to required checks, so a docs-only PR
  can merge. Workflow-level `paths-ignore` would leave the required checks
  pending forever.
- **Fail-safe:** the condition is `!= 'false'`, not `== 'true'`, and
  `!cancelled()` overrides the default "skip when a needed job failed". If the
  `changes` job fails, errors or produces no output, the code jobs run.
  Without this, a broken `changes` job would skip every test and the required
  checks would still pass.
- `No customer-specific data` has no `needs` and always runs.
- The job names don't change, so the `Main Squash Only` ruleset needs no edit.

The `changes` job itself is not added to the required checks. Its failure
can't let untested code through, because of the fail-safe above.

### `breaking.yml`

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened, edited]
    paths:
      - api/proto/public/**
      - api/buf.yaml
      - api/buf.lock
```

GitHub evaluates `paths` against the PR's whole diff, so editing the title of
a PR that changes public protos should still re-run the check. That's what
lets a `!` added to the title clear a breaking change. **To verify** on a real
PR (see Tests), since the documentation doesn't spell out `edited`. If it
doesn't hold, the filter moves into the job as an `if:` on the same
`changes`-style file list.

## Classification cases

`scripts/changed-code_test.sh` covers at least:

| Paths | Expected |
|---|---|
| `docs/x/intent.md` | `false` |
| `docs/diagram.svg` | `false` |
| `README.md`, `api/proto/public/README.md`, `CHANGELOG.md` | `false` |
| `docs/x/spec.md`, `services/gateway/main.go` | `true` |
| `.github/workflows/ci.yml` | `true` |
| `scripts/changed-code.sh` | `true` |
| `mise.toml` | `true` |
| `notes.markdown` | `true` |
| `README.MD` | `true` |
| `docsfoo/x.txt` | `true` |
| (empty) | `true` |

Renames are covered by the job feeding both names to the script, so a rename
from `services/x.go` to `docs/x.go` contains a code path.

## Tests

| Layer | Coverage |
|---|---|
| Script | `scripts/changed-code_test.sh`, run by the `changes` job on every event and locally with `sh scripts/changed-code_test.sh`. |
| Workflow | Checked on real PRs after the change merges, since workflow logic can't run locally: a docs-only PR shows the four code jobs as skipped and can merge; a PR with one Go change runs everything; a PR renaming a Go file into `docs/` runs everything; the next push to `main` runs everything. A proto-only PR runs `breaking.yml`, and re-runs it when its title is edited; a Go-only PR doesn't run it. |
| Fail-safe | One throwaway PR where the `changes` job is forced to fail (for example `exit 1` in a test commit, never merged): the code jobs still run. |

## Docs to update

- `CLAUDE.md`: under Commands, note that docs-only PRs skip the code jobs, so
  run `mise run check` locally if a docs change touches anything a test reads.
- `docs/github-setup.md`: note that `Detect docs-only changes` is deliberately
  not a required check, and why.

## Out of scope

Skipping jobs on push to `main`, a merge queue (`merge_group`), the
breaking-check baseline itself, caching or speeding up the code jobs, and
skipping jobs for other kinds of change (for example Go-only PRs skipping the
SDK job).
