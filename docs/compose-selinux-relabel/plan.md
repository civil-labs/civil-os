# Plan: relabel compose bind mounts for SELinux hosts

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md), proposed in the same PR at the owner's request, with no intent |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-17 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One implementation PR**, on branch `build/compose-selinux-relabel`,
  titled `build: relabel compose bind mounts for SELinux hosts`. It's opened
  after this spec and plan merge. The agent never pushes to `main`. The owner
  merges.
- **Each step is one commit** with a conventional-commit message.
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Verification before "done".** The checks below are run locally, and their
  output goes in the PR description.
- **The developer's container setup isn't changed.** A temporary rootless
  podman socket is fine for verification. Docker contexts, daemon config and
  the system podman socket are left alone.

## Steps

**1. Baseline** (no commit)

On an SELinux host in Enforcing mode (`getenforce`), with `DOCKER_HOST`
pointing at a rootless podman socket, and no override file:
- run `docker compose up --build --wait`;
- record that `migrate` fails with
  `cannot open /dev-scripts/migrate.sh: Permission denied`;
- run `docker compose down -v`.

This confirms that the failure the fix targets still happens. It goes in the
PR description.

**2. Compose** (`build: relabel compose bind mounts for SELinux hosts`)

In `compose.yaml`:
- append `,z` to the options of the six bind mounts listed in the spec;
- above the `migrate` volumes, add a comment: "`z`: on SELinux hosts (e.g.
  podman on Fedora), relabel the mounted repo files so containers can read
  them. Ignored elsewhere. Only on paths inside the repo. See
  docs/compose-selinux-relabel/spec.md."

Then check `docker compose config`. Each mount must render as
`read_only: true` and `bind: {selinux: z}`, and nothing else in the rendered
config may change: compare it with `main`'s output.

**3. Docs** (`docs: running the local stack on SELinux hosts`)

- `README.md`, Quickstart: the sentence from the spec.
- `CLAUDE.md`, Known mistakes: the entry from the spec.

## Checks

- `docker compose config` as in step 2, before and after, with the diff in
  the PR description.
- **Podman on SELinux:** repeat step 1's setup and run `mise run test:e2e`,
  which must pass. `ls -Z dev/migrate.sh database/fixtures` must show
  `container_file_t`. Tear down with `mise run down`.
- **Docker without SELinux:** the CI `E2E (docker compose)` job passes on the
  PR. `compose.yaml` isn't docs, so the job runs.
- `mise run check` isn't affected, but run it anyway, since CI does.

## Risks

| Risk | Mitigation |
|---|---|
| Docker Desktop on macOS or Windows rejects `z`, instead of ignoring it. | Accepted for now, unverified: no Mac is available to test on, and CI doesn't cover Docker Desktop. The PR description says so. If a Docker Desktop user later reports that `up` fails, revisit the spec. A per-platform override file is the fallback. |
| Someone adds `z` to a broad path later and relabels a home directory. | The `compose.yaml` comment and the `CLAUDE.md` entry both restrict it to paths inside the repo. Review catches the rest. |
| A checkout on NFS or FUSE fails to relabel and stops the stack under podman. | It already fails there today. The README sentence names the requirement. |
| Relabeling leaves the checkout's files as `container_file_t`, and that surprises someone. | It's documented in the spec. `restorecon -R <path>` restores the default label. |
