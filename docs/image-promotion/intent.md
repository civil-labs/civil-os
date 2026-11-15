# Intent: release images are promoted from main, not rebuilt

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

When a release PR merges, `images.yml` builds every service image **twice from
the same commit**:

1. The push to `main` builds and pushes `:sha-<short>` (spec F9).
2. release-please tags that same commit `vX.Y.Z`. The tag push starts a
   second, independent build that pushes `:vX.Y.Z` (spec F7).

The two builds come from identical source, but they aren't the same image:

- **The release isn't the image that was tested.** `:vX.Y.Z` has a different
  digest from `:sha-<short>`. The OCI labels differ, and a base image or
  dependency fetched at build time could change between the two runs. What
  CI and anyone running `main` exercised isn't, byte for byte, what gets
  released.
- **The provenance attestations differ** for what is meant to be one artifact.
- **It wastes build time** on every release, for all four services.

This came up in practice. Restoring `:v0.2.1` meant re-tagging
`:sha-4cd953b` by hand, which is the behavior the workflow should have.

## Proposed outcome

1. **Build once.** Only pushes to `main` build images.
2. **A release tag promotes.** On a `vX.Y.Z` tag, the workflow adds the
   `vX.Y.Z` tag to the existing `:sha-<short>` manifest for the tagged commit,
   for each service. Nothing is rebuilt.
3. **The release digest equals the `main` digest.** `:vX.Y.Z` and
   `:sha-<short>` resolve to the same manifest, and the attestation made at
   build time applies to both.

## Constraints

- **Spec F7 and F9 still hold.** Every merge to `main` publishes `:sha-<short>`,
  every app release publishes `:vX.Y.Z` for all four services, and `:latest` is
  never published.
- **Handle the race.** The `main` build and the tag push start at almost the
  same moment, so promotion has to wait for the `:sha-<short>` images, with a
  bounded wait.
- **A release never silently goes without images.** If the `:sha-<short>`
  image for the tagged commit doesn't exist or never appears (for example,
  the `main` build failed), the workflow fails visibly. Whether it should also
  fall back to building is a spec decision.
- **No manual dispatch.** The owner decided against a `workflow_dispatch`
  trigger. Manual fixes stay the documented `docker buildx imagetools create`
  command.
- Nothing changes for the migrations ConfigMap or the npm release workflows.
- The Helm chart, the gitops repo and the jurisdiction repos aren't edited from
  here.

## Success criteria

- On the next app release, `:vX.Y.Z` and `:sha-<short>` of the tagged commit
  have the same digest for all four services.
- The tag-triggered Images run builds nothing and finishes in well under a
  minute once the `main` build is done.
- A tag on a commit with no `:sha-<short>` image makes the Images run fail
  with a clear message, not hang or publish nothing.
- `docs/architecture.md` and `docs/github-setup.md` describe releases as
  promotions.

## Open questions (carry into spec)

- **Fallback.** If the `main` build failed, should the tag run fail, or build
  the image itself as before?
- **Waiting.** Should promotion poll the registry for the `:sha-<short>`
  manifest, or wait on the `main` workflow run through the GitHub API? And
  how long should it wait?
- **Attestations.** Does the build-time attestation (subject: the digest)
  need anything extra for verifiers who look up `:vX.Y.Z`, or is the shared
  digest enough?
- **Labels.** A promoted image keeps the `main` build's OCI labels, so its
  `org.opencontainers.image.version` label reads `sha-<short>`. Is that
  acceptable, or should the `main` build set labels that don't depend on the
  tag?
- **Multi-platform.** Images are `linux/amd64` only today. If ARM is added
  later, promotion has to copy the whole image index, which `imagetools create`
  does. Should the spec require that now?
