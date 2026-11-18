# Spec: relabel compose bind mounts for SELinux hosts

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-17 |
| **Status** | Draft, for the owner's review. Accepted when merged. |
| **Intent** | None. The owner asked for a spec and plan directly, in the same PR, on 2026-11-17. |

This spec sets out the change to `compose.yaml` and the docs that go with it.
The order of the work goes in [`plan.md`](plan.md).

## Problem

`docker compose up` fails on a host that enforces SELinux, such as Fedora or
RHEL, when the container runtime applies SELinux labels. Rootless podman does
this by default. The first service with a bind mount fails:

```
sh: 0: cannot open /dev-scripts/migrate.sh: Permission denied
```

The cause:

- Files in a checkout under the home directory are labeled `user_home_t`.
- Podman runs each container as the confined type `container_t`, which can't
  read `user_home_t`.
- A bind mount keeps the host file's label, so the read is denied, even
  though the Unix permissions allow it and the mount is `:ro`.

`migrate` fails, so `seed` and every Go service behind it never start. The same
host with Docker Engine works, because Docker leaves SELinux support off unless
`"selinux-enabled": true` is set in `daemon.json`. CI runs Ubuntu with
AppArmor, so it never sees this.

## Decisions

| Question | Decision |
|---|---|
| Fix | **Add the `z` option to every bind mount in `compose.yaml`.** On a host enforcing SELinux, the runtime relabels the mounted host path to `container_file_t` with no MCS categories, before the container starts. Anywhere else, the option is ignored. |
| `z` or `Z` | **Lowercase `z` (shared).** Uppercase `Z` labels the path with one container's private MCS categories. A second container, or the same service recreated with new categories, would be denied again. No mount here needs to be private. |
| Alternatives rejected | **`security_opt: [label=disable]`** turns off SELinux confinement for the container. It works without touching host files, but it weakens isolation for anyone who runs the stack. **Copying the files into images** means a rebuild for every fixture or script change, and the local stack is meant to pick them up from the checkout. **Documenting a per-developer override file** pushes the fix onto every podman user. |
| Syntax | **Keep the short syntax, adding `z` after `ro`**, e.g. `./dev/migrate.sh:/dev-scripts/migrate.sh:ro,z`. Compose parses the comma-separated options into `read_only: true` and `bind.selinux: z`. That's the same result as the long syntax, with a smaller diff. |
| Scope of relabeling | **Only the six existing bind mounts**, all repo-relative and narrow. The rule for new mounts: `z` only on a path inside the repo, never on `.`, the home directory or a system path. Relabeling those can break logins and system services. |
| Named volume `db-data` | **Unchanged.** The runtime labels named volumes itself. |
| Testcontainers (`mise run test:integration`) | **Out of scope.** It uses no bind mounts, and it already passes under podman. Pointing testcontainers at podman (`DOCKER_HOST`, Ryuk) is a developer environment question, not a repo change. |
| Supporting podman officially | **Not promised.** The README keeps "Docker with Compose v2" as the requirement. This change removes one known blocker on SELinux hosts. CI doesn't run podman. |

## The change

`compose.yaml`, with every bind mount gaining `z`:

| Service | Mount |
|---|---|
| `migrate` | `./database/migrations:/migrations:ro,z` |
| `migrate` | `./dev/migrate.sh:/dev-scripts/migrate.sh:ro,z` |
| `seed` | `./database/fixtures:/fixtures:ro,z` |
| `seed` | `./dev/seed.sh:/dev-scripts/seed.sh:ro,z` |
| `dex` | `./dev/dex.yaml:/etc/dex/config.yaml:ro,z` |
| `gateway` | `./dev/instance-metadata.json:/etc/civil-os/instance-metadata.json:ro,z` |

A comment above the first one explains why, and points to this spec.

No other file under `services/`, `dev/` or `database/` changes.

## Effects on the host

- **The relabel persists.** After a run, the mounted files and directories are
  labeled `container_file_t`. Editors, git and shells run unconfined, so they
  aren't affected. Git doesn't record SELinux labels, so nothing shows in a
  diff or reaches another clone.
- **New files can come back as `user_home_t`.** A file git rewrites, such as
  `dev/migrate.sh` after a pull, can take its directory's label again. The
  next `up` relabels it, because `z` applies on every container start.
- **On hosts without SELinux** (Ubuntu CI, Docker Engine with SELinux off, and
  Docker Desktop on macOS and Windows), `z` is ignored. Docker Desktop is the
  one case not checked by CI. See the plan's risks.
- **Filesystems without SELinux labels** (NFS, some FUSE mounts): the relabel
  can fail and stop the container. This affects only a checkout on such a
  filesystem under an SELinux runtime, which already fails today.

## Docs to update

- `README.md`, Quickstart: one sentence saying that the stack also runs under
  podman on SELinux hosts. The bind mounts carry `z` for that, and they need a
  checkout on a filesystem with SELinux labels.
- `CLAUDE.md`, Known mistakes: "Adding a bind mount to `compose.yaml` without
  `z`, or dropping it as unexplained. SELinux hosts need it
  (`docs/compose-selinux-relabel/spec.md`). Never use it on a path outside the
  repo."

## Tests

| Check | Expectation |
|---|---|
| `docker compose config` | Each of the six mounts renders as `type: bind`, `read_only: true`, `bind: {selinux: z}`. |
| CI `E2E (docker compose)` on Ubuntu | Passes unchanged. `z` is ignored. |
| Local: rootless podman on an SELinux host in Enforcing mode, with no override file | `mise run test:e2e` passes, and `ls -Z` on the mounted paths shows `container_file_t`. Before the change, `migrate` fails with the error above. |

## Release

One PR, `build: relabel compose bind mounts for SELinux hosts`. `build:`
doesn't trigger a release, and the public API is untouched.

## Out of scope

- Running CI under podman.
- Rootless podman setup for testcontainers.
- Any change to the Helm chart or deployment repos, which don't use compose.
