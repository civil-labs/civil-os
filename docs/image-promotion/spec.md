# Spec: promote release images from main instead of rebuilding

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-17 |
| **Status** | Draft. The decisions below are proposals, for the owner to sign off. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) (#16) |

This spec sets out the behaviour: what triggers a build or a promotion, how
promotion finds and waits for the `main` image, how it fails, and the docs.
The order of the work goes in `plan.md`.

## The problem, confirmed

On 2026-11-17, `gateway:v0.5.0` and `gateway:sha-6d3fdc8` were checked. Both
are tags for the same commit, and both are OCI image indexes (one
`linux/amd64` manifest plus its provenance attestation manifest). Their
digests differ:

| Tag | Digest |
|---|---|
| `v0.5.0` | `sha256:ba2c35c8…3634a50` |
| `sha-6d3fdc8` | `sha256:76fb3417…e341429a` |

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Fallback when the `main` image is missing | **Fail. Never build on a tag.** A fallback build would bring back what the intent removes: a release image nobody tested, published without anyone noticing. The fix is to re-run the failed `main` Images run, then re-run the failed tag run with GitHub's **Re-run failed jobs**. That's a re-run, not a `workflow_dispatch`, so it respects the intent's constraint. The documented `imagetools create` command stays as the manual way out. |
| How to wait | **Poll the registry for `:sha-<short>`**, not the `main` workflow run. The registry is what promotion actually needs, so it can't race the run's status, and the job needs no `actions: read` and no run lookup. A failed `main` build shows up as a timeout. That's slower than failing fast, but bounded. |
| How long | **Every 15 s, for up to 20 minutes, per service.** The slowest of the last 30 `main` Images runs took 343 s, and the typical one takes 40–200 s, so 20 minutes is more than three times the worst case. A run where the image is already there finishes in seconds. |
| Attestations | **Nothing extra.** The build-time provenance has the digest as its subject. It's pushed to the registry as a referrer of that digest, and stored by digest in GitHub's attestation store. `:vX.Y.Z` resolves to the same digest, so `gh attestation verify oci://ghcr.io/civil-labs/civil-os/<service>:vX.Y.Z --repo civil-labs/civil-os` finds it. The plan checks this on the first promoted release. |
| `org.opencontainers.image.version` label | **Accept `sha-<short>`.** Labels are inside the image config, so any change after the build changes the digest, which defeats the point. The commit is the honest version of a build. The release version is the tag, and `org.opencontainers.image.revision` (the full commit SHA) ties the two together. `docs/architecture.md` says so. |
| Multi-platform | **Required now, at no cost.** Promotion tags the image index the build pushed, not a platform manifest inside it. That's already necessary today, because the build pushes an index (the platform manifest plus its attestation). An ARM platform added later needs no change. |

## Further decisions

| Question | Decision |
|---|---|
| How a tag is added | **`docker buildx imagetools create --tag <image>:vX.Y.Z <image>:sha-<short>`**, the same command as the documented manual fix. It's then checked: promotion fails unless both tags resolve to the same digest afterwards. Whether `imagetools create` copies a single-source index unchanged is verified first, in the plan's step 1. If it doesn't, the plan stops, and the spec is amended to use a byte-for-byte tag such as `crane tag`, before any workflow changes. |
| The `sha-<short>` name | **Seven characters, pinned on both sides.** The build sets `DOCKER_METADATA_SHORT_SHA_LENGTH: 7` (metadata-action's current default, made explicit). Promotion computes `sha-${GITHUB_SHA::7}`. If metadata-action's default changes, the two can't drift apart. |
| Which commit | **The commit the tag points to (`GITHUB_SHA` on the tag push).** release-please tags the release PR's merge commit on `main`, which is the commit the concurrent `main` run builds. A tag on a commit that was never built on `main`, such as a hand-made tag on a branch, times out and fails with the same message. |
| Workflow shape | **One workflow, two jobs**, chosen by the ref. `build` runs on pushes to `main` and is unchanged apart from the tags. `promote` runs on `v*` tags. Both use the same four-service matrix, with `fail-fast: false`, so one service's failure doesn't hide the others'. Concurrency stays per ref, so the two runs proceed in parallel, as they must. |
| Tags the build pushes | **Only `sha-<short>`.** The `type=semver` line is removed from metadata-action, and `build` no longer runs on tags at all. |
| `promote` permissions | `contents: read` and `packages: write`. No `attestations`, no `id-token`, no build cache. |
| Re-tagging an existing release | **Refused.** If `:vX.Y.Z` already exists with a different digest, promotion fails rather than moving a published tag. If it already has the right digest, for example on a re-run, it passes. |
| Old releases (≤ v0.5.0) | **Left as they are.** They stay rebuilt images. Re-pointing them would move published tags under running deployments. |

## Behaviour

### On a push to `main`: `build`

As today, without the semver tag: build, push `:sha-<short>`, attest. F9 holds.

### On a `v*` tag: `promote`, per service

1. Log in to `ghcr.io`.
2. Wait: run `docker buildx imagetools inspect <image>:sha-<short>` every 15 s,
   until it succeeds or 20 minutes pass. On timeout, fail with:
   `<image>:sha-<short> not found after 20 min. Did the main Images run for <sha> fail? Re-run it, then re-run this job.`
3. If `:vX.Y.Z` already exists: pass if its digest equals `:sha-<short>`'s.
   Otherwise, fail with both digests and stop.
4. Promote, using the command above.
5. Verify: inspect both tags. Fail unless the digests are equal.
6. Write a job summary line: service, tag, digest.

F7 holds: every app release publishes `:vX.Y.Z` for all four services, or
the run fails visibly.

### Unchanged

`migrations-release.yml`, `public-api-release.yml`, `release-please.yml` and
the packages' settings.

## Docs to update

- `docs/architecture.md`, release table and bullets:
  - `:vX.Y.Z` is the `main` image for the tagged commit, promoted, with the
    same digest;
  - the version label reads `sha-<short>`, and the revision label has the
    full commit.
- `docs/github-setup.md`:
  - the first-release checklist says the `v*` tag promotes `:vX.Y.Z` images;
  - a short "Release images missing" entry: re-run the `main` Images run, then
    re-run the tag run's failed jobs. As a last resort, the `imagetools create`
    command, followed by the digest check.
- The comment at the top of `images.yml`.

## Tests

The workflow can't run in CI before it merges, so checking it is part of the
plan:

| Check | When |
|---|---|
| `actionlint` on `images.yml` | In the PR |
| `imagetools create` on a single-source OCI index keeps the digest. Shown against a throwaway local registry, with an index shaped like the real ones (platform manifest plus attestation). | Plan step 1, before the workflow changes |
| The next app release: for all four services, `:vX.Y.Z` and `:sha-<short>` have the same digest; `gh attestation verify` passes on `:vX.Y.Z`; the tag run builds nothing, and finishes in under a minute when `main` is done first | After merge, on the first release |
| The failure path: a tag with no `sha` image fails with the message above | Not tested on the real registry, to avoid a junk release tag. The wait-and-fail logic is a shell script, unit-tested with a fake `inspect` command. |

## Release

One PR, `ci: promote release images from main instead of rebuilding`. `ci:`
doesn't trigger a release.

## Out of scope

- Backfilling or re-pointing releases up to v0.5.0.
- A `workflow_dispatch` trigger (the owner decided against it).
- ARM images.
- Signing beyond the existing build provenance.
- The Helm chart, the gitops repo and the jurisdiction repos.
