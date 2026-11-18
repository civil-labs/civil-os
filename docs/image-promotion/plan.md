# Plan: promote release images from main instead of rebuilding

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `1979065`, #57), which implements [`intent.md`](intent.md) (#16) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-18 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **One PR**, on branch `ci/image-promotion`, titled
  `ci: promote release images from main instead of rebuilding`, as the spec
  says. `ci:` doesn't trigger a release. The agent never pushes to `main`. The
  owner merges.
- **Each step is one commit** with a conventional-commit message.
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same commit. If the departure changes the spec,
  the agent stops and asks the owner first.
- **Nothing is pushed to `ghcr.io` before merge.** Step 1 runs against a
  throwaway local registry. Pulling the public images anonymously is fine.
- **The developer's container setup isn't changed.** A temporary rootless
  podman socket and a local registry container are fine. Both are removed
  afterwards.

## Steps

**1. Check that `imagetools create` keeps the digest** (no commit)

This is the spec's gating check. On a throwaway local registry
(`registry:2.8.3` on `localhost:5000`, under podman):

1. Copy the real index `ghcr.io/civil-labs/civil-os/gateway:sha-6d3fdc8` with
   `skopeo copy --all --preserve-digests` to `localhost:5000/gateway:sha-6d3fdc8`.
   That's an OCI index with a `linux/amd64` manifest and its attestation
   manifest, the shape the spec describes. Check that the copy's digest equals
   the source's, `sha256:76fb3417…`.
2. Run
   `docker buildx imagetools create --tag localhost:5000/gateway:v0.0.0-check localhost:5000/gateway:sha-6d3fdc8`.
3. Compare the two tags' digests with `skopeo inspect --raw … | sha256sum`
   and `imagetools inspect`.

- **Equal:** record the output for the PR description, and go on.
- **Different, or the local setup can't run it:** stop. Ask the owner whether
  to amend the spec to use a byte-for-byte tag such as `crane tag`, or to run
  the check against a throwaway `ghcr.io` repository. Nothing in steps 2–4
  starts before then.

Remove the registry container afterwards.

**2. Promotion script** (`ci: add a script that promotes a main image to a release tag`)

`scripts/promote-image.sh IMAGE COMMIT_SHA RELEASE_TAG`, POSIX `sh`, following
the other scripts in `scripts/`. It implements the spec's "On a `v*` tag" steps
2–5 for one service:

- **Arguments:** it computes `sha-<first 7 of COMMIT_SHA>`. It exits 2 on
  usage errors: a wrong argument count, a commit SHA that isn't 40 hex
  characters, or a release tag that isn't `vX.Y.Z`. The last is a small
  addition to the spec. A non-semver `v*` tag used to publish nothing, and
  failing loudly matches the intent's "never silently". If the owner
  disagrees, this goes back to the spec.
- **Waiting:** it polls `<image>:sha-<short>` every `PROMOTE_INTERVAL`
  seconds (default 15), for up to `PROMOTE_TIMEOUT` seconds (default 1200).
  On timeout it exits 1, with the spec's message on stderr.
- **Existing release tag:** same digest → exit 0 without creating anything.
  Different digest → exit 1, with both digests.
- **Promotion:** `imagetools create`, then the digest-equality check, which
  exits 1 if they differ.
- **Output:** one Markdown table row on stdout (service, tag, digest), for
  `$GITHUB_STEP_SUMMARY`.
- **Fake for tests:** registry calls go through `$IMAGETOOLS`, which defaults
  to `docker buildx imagetools`. A test points it at a fake.

`scripts/promote-image_test.sh` is a table test in the style of
`is-breaking-bump_test.sh`. Its fake `imagetools` serves digests from a temp
directory and logs every `create`. With `PROMOTE_INTERVAL=0` and a short
`PROMOTE_TIMEOUT`, the cases are:

| Case | Expectation |
|---|---|
| `sha` present, release absent | exit 0; one `create`; summary row |
| `sha` appears after two polls | exit 0; one `create` |
| `sha` never appears | exit 1; the timeout message; no `create` |
| release exists, same digest | exit 0; no `create` |
| release exists, different digest | exit 1; no `create` |
| `create` yields a different digest | exit 1 |
| full SHA in, `sha-` plus the first 7 characters looked up | checked in the fake's log |
| bad argument count / short SHA / tag `vfoo` | exit 2 |

`ci.yml`'s `changes` job, which runs on every PR, runs the test next to
`changed-code_test.sh`.

**3. Workflow** (`ci: promote release images from main instead of rebuilding`)

`images.yml`:
- **Header comment:** "main builds and pushes :sha-<short>. An app release tag
  vX.Y.Z adds that tag to the main image for the same commit, without
  rebuilding (docs/image-promotion/spec.md)."
- **`build` job:** the current `image` job, renamed.
  - `if: github.ref == 'refs/heads/main'`.
  - `DOCKER_METADATA_SHORT_SHA_LENGTH: "7"` in its env.
  - metadata-action `tags:` is just `type=sha,prefix=sha-`. The `semver`
    line and the `enable` condition go.
  - `flavor: latest=false` and everything else stay.
- **New `promote` job:**
  - `if: startsWith(github.ref, 'refs/tags/v')`.
  - The same four-service matrix, with `fail-fast: false`.
  - `timeout-minutes: 25`.
  - Permissions: `contents: read` and `packages: write`.
  - Steps:
    1. checkout;
    2. `setup-buildx-action` and `login-action`, pinned as today;
    3. `sh scripts/promote-image_test.sh`, so the script is tested before it's
       trusted, as `migrations-release.yml` does;
    4. `sh scripts/promote-image.sh "$IMAGE" "$GITHUB_SHA" "$GITHUB_REF_NAME" >> "$GITHUB_STEP_SUMMARY"`,
       with the values passed via `env`, never interpolated.
- **Lint:** run `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7` on
  the workflows. The tool isn't added to `mise.toml`.

**4. Docs** (`docs: releases promote the main images`)

*Amended during implementation:* `README.md`'s release paragraph is updated
too. It repeated the "every merge to `main`" sentence.

- **`docs/architecture.md`:**
  - the App row of the release table: `:vX.Y.Z` is the `main` image of the
    tagged commit, promoted, with the same digest;
  - the bullets: the version label reads `sha-<short>`, and
    `org.opencontainers.image.revision` has the full commit.
- **`docs/github-setup.md`:**
  - the first-release checklist line says the tag promotes images;
  - a "Release images missing" entry: re-run the `main` Images run for the
    commit, then **Re-run failed jobs** on the tag run. As a last resort, use
    the `imagetools create` command, then compare digests.

## Checks (before the PR is ready)

- `sh scripts/promote-image_test.sh` passes.
- `actionlint` is clean on `.github/workflows/`.
- Step 1's digest comparison, with its output in the PR description.
- `mise run check`, which CI runs anyway.
- Read the rendered `images.yml` against the spec's behaviour section, line by
  line. The workflow can't run before it merges.

## After merge (on the next app release)

The agent, or the owner, checks the following and posts the results as a
comment on the merged PR:

- for all four services, `:vX.Y.Z` and `:sha-<short>` have the same digest,
  via `skopeo inspect --raw` or the registry API;
- `gh attestation verify oci://ghcr.io/civil-labs/civil-os/<service>:vX.Y.Z --repo civil-labs/civil-os`
  passes for each service;
- the tag's Images run has only `promote` jobs. Its duration, from
  `gh run list --workflow images.yml`, is under a minute when the `main` run
  finished first;
- the summary has four rows.

If any of these fails, the next step is a `fix:` PR. Release tags aren't moved
by hand, except with the documented `imagetools create` fallback.

## Risks

| Risk | Mitigation |
|---|---|
| `imagetools create` re-serialises the index, so the digest changes. | Step 1 gates everything on it, and the script's digest check fails a release rather than publishing a different image. |
| The release tag's run starts before `main` has pushed `:sha-<short>`. | Expected. The script waits up to 20 minutes. The slowest recent `main` run took 343 s. |
| `main` build fails for the release commit. | The tag run times out with a message that says what to do. The recovery is in `docs/github-setup.md`. |
| A bug in the script blocks a release. | It's table-tested in every PR and again in the release run before use. The `imagetools create` fallback still works by hand. |
| `actionlint` can't catch logic errors in conditions such as `if: startsWith(...)`. | The line-by-line review above, and the post-merge checks on the first real release. |
