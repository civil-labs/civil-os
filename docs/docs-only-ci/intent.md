# Intent: skip the code CI jobs on docs-only pull requests

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-15 |
| **Status** | Draft, written at the owner's request after a docs-only PR (#17) ran the full CI suite. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

Every pull request runs the whole of `.github/workflows/ci.yml`: Go build,
vet, unit and integration tests (a PostGIS testcontainer), protos and
generated code, the JS SDK pack and import, and the docker compose e2e
stack. A PR that only changes design docs (`docs/<feature>/{intent,spec,plan}.md`)
can't affect any of them, but waits on all of them. Design docs are a large
share of PRs, because every feature goes through intent, spec and plan.

The obvious fix doesn't work here. The `Main Squash Only` ruleset makes these
checks required:

- `Go build, vet, unit and integration tests`
- `Protos and generated code`
- `JS SDK packs and imports`
- `E2E (docker compose)`
- `No customer-specific data`
- `conventional-title`

Skipping the workflow with `paths-ignore` means the required checks never
report, so GitHub shows them as "Expected — Waiting for status" and the PR
can't merge.

## Proposed outcome

1. **Docs-only is decided by the files a PR touches, not its title.** A PR is
   docs-only when every changed file is under `docs/` or ends in `.md`. For
   renamed files, both the old and the new path count, so moving code into
   `docs/` isn't docs-only.
2. **Anything else runs everything.** The rule is an allowlist of docs paths,
   so a new top-level file or directory runs the full suite by default.
3. **The four code jobs are skipped at the job level** (`if:` on a small
   `changes` job's output), not the workflow level. A job skipped by `if:`
   reports success, so the required checks pass and docs PRs can merge.
4. **The customer-data check always runs.** Docs are where real names are most
   likely to slip in. The PR-title check is in its own workflow and is
   unaffected.
5. **Pushes to `main` always run the full suite.**

## Constraints

- **No new third-party action unless needed.** The changed-file list can come
  from `gh api repos/{repo}/pulls/{n}/files`, which is already on the runners.
  If an action is used, it's pinned to a commit SHA like every other action.
- **No change to the ruleset's required checks.** The job names stay the same
  so the ruleset keeps working without being edited.
- **No Markdown is part of a build or test today.** Nothing uses `go:embed`,
  and the SDK packs `sdk/js/README.md` but doesn't test it. If that changes,
  the docs rule must be narrowed.

## Success criteria

- A PR changing only files under `docs/` or `*.md` shows the four code jobs as
  skipped, runs `No customer-specific data`, and can merge.
- A PR changing any other file runs every job, as today.
- A PR that renames a Go file into `docs/` runs every job.
- Pushes to `main` run every job.

## Open questions (carry into spec)

- **Which jobs skip.** All four code jobs, or only e2e? Nothing in Go, protos
  or the SDK reads Markdown, so skipping all four looks safe.
- **The docs rule.** Is `docs/**` plus `**/*.md` right, or should Markdown
  that ships (`sdk/js/README.md`, `api/proto/public/README.md`) still run the
  SDK job?
- **Other workflows.** Should `breaking.yml` (`buf breaking`) skip on docs-only
  PRs too? It isn't a required check, so it can use `paths-ignore`.
- **Large PRs.** The files API lists at most 3,000 files per PR. Treat a PR
  at that limit as code, to be safe?
