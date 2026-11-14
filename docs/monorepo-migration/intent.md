# Intent: consolidate civil-os into one open-source Go monorepo

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-13 |
| **Status** | Draft. Awaiting product-owner review; accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` (public) |

## Problem

civil-os is split across seven repos: civil-gateway, civil-database-reader,
civil-database-editor, civil-tile-server, civil-blueprints, civil-api-go and
civil-database-schema. That split causes the following problems:

- **API changes are slow and drift.** A proto change takes three hops: edit
  blueprints, CI pushes generated code to civil-api-go, then each service runs
  `go get @latest`. The services now pin different API versions (editor is on
  April's, gateway and reader on August's).
- **The editor service is non-functional.** It only stubs two RPCs that no longer
  exist. The gateway's `UpdateParcel` returns success without writing anything.
  No cluster deploys the editor.
- **No single place holds the application's context.** That context is what
  upcoming AI-native development needs, and what the planned ELT scripts and
  frontend apps need to build against the app.
- **No coherent release.** Each repo versions and publishes on its own, so there is
  no single version of "civil-os" to ship to the live clusters, publish as open
  source, or tell a user to run.
- **Hard to adopt as open source.** There is no README, no local quickstart, no
  architecture overview and no tests. The gateway is hard-wired to AWS Cloud Map
  for service discovery.

The only person affected today is the author, who is the sole developer and
operator. Future open-source users and contributors will also run into these
problems.

## Proposed outcome

civil-os is a single public repo with one Go module. In it:

1. **One commit can change the whole stack.** A proto change, its generated code
   and every service that uses it land together.
2. **One semver version describes the app.** A release tag `vX.Y.Z` produces every
   service image and the DB migration ConfigMap artifact at that version. The tag
   also serves as the Go module version.
3. **The public API is released separately.** "Public API" releases
   (`public-api-vX.Y.Z`) carry their own semver, because the public contract
   follows different compatibility rules from the app. The JS SDK is published to
   npm as `@civil-labs/civil-os-api` at that version. The release is named for the
   API, not for JS, so Python, Rust or Swift SDKs can join later.
4. **Parcel writes work end to end.** The database-editor is renamed to
   **database-writer** everywhere. It implements `ParcelWriteService.UpdateParcel`
   for real, and the gateway's public `UpdateParcel` routes to it.
5. **A newcomer or an AI agent can understand and run the app from the repo.**
   The repo provides a README, an architecture overview, a `CLAUDE.md`, and
   `docker compose up` on synthetic data.
6. **Tests cover the stack:** unit, gateway handler, integration (Postgres + PostGIS)
   and end-to-end smoke tests, all running in CI.
7. **Backend discovery is cloud-neutral.** The gateway finds backends through
   Kubernetes DNS, with no AWS hard coupling.

## Decisions made during brainstorm

**Repo structure**
- **One Go module** at the root. Per-service dependency versions are not worth the
  extra complexity.
- **History:** importing each source repo's history is nice to have, not required.
  All source repos are already public, so publishing their history leaks nothing new.

**API and service design**
- **Generated code lives in the repo.** civil-api-go is retired and the repo
  commits the generated Go code.
- **Mesh generated code is internal.** It goes under `internal/` so external Go
  users can't import it. Only the public API's Go code can be imported.
- **The Go public API code follows the app version** (`vX.Y.Z`), not the Public
  API version. Splitting it into its own nested module stays possible later; doing
  it before 1.0 is cheapest.
- **Mesh services use Read/Write naming.** `ParcelsService` becomes
  `ParcelReadService` (served by the reader) and `ParcelWriteService` (served by
  the writer). Every other mesh service takes the same pattern, for example
  `LandUseReadService` and `ZoningReadService`.
- **The public API keeps a single `ParcelsService`**, so external clients don't see
  the read/write split.
- **`UpdateParcel` semantics:** `properties` is a JSON blob that *replaces* the
  stored value. No audit record for now.
- **Authorization for writes is out of scope.** It will come later through OPA
  (added separately) and DB role permissions (handled in the cluster
  infrastructure repos).
- **Backend discovery:** the gateway connects to a plain Kubernetes Service DNS
  name. Load balancing happens per connection, through Istio ambient (ztunnel);
  the docs will note this limitation. The gocloud.dev blob dependency for instance
  metadata stays, since it already works on any cloud.

**Release and publishing**
- **Images:** `ghcr.io/civil-labs/civil-os/{gateway, database-reader,
  database-writer, tile-server}`. There is no migrations container image; the
  migrations ship only as a ConfigMap artifact.
- **Release workflow:** trunk-based, with a protected `main` and squash-merged PRs
  whose titles are conventional commits.
  - release-please runs in manifest mode with two release streams: the app
    (`vX.Y.Z`) and the Public API (`public-api-vX.Y.Z`).
  - Before 1.0, breaking changes bump the minor version.
  - Every merge to `main` also publishes `:sha-<short>` images. There is no
    `latest` tag.
- **License:** keep the existing MIT LICENSE. The README is the only other
  open-source file needed for now.

## Affected users and systems

**Moved into civil-os:** civil-gateway, civil-database-reader,
civil-database-editor (renamed database-writer), civil-tile-server,
civil-blueprints and civil-database-schema.

**Replaced:**
- **civil-api-go** is replaced by generated code inside the repo.
- **civil-api-js** (git remote currently `civil-api-ts`) is replaced by npm
  publishing.

Both repos are archived later, together with the other source repos.

**Downstream, not changed by this effort:**
- civil-labs-helm-charts, civil-os-deploy-gitops and the jurisdiction deploy repos
  must be updated to the new image names. That is a separate follow-up.
- civil-lvt-shift installs `github:civil-labs/civil-api-js` and must switch to the
  npm package before that repo is archived.

**Explicitly out of scope:**
- **Other repos:** civil-elt (internal), civil-idp and civil-proxy (retired), and
  the helm chart, which is proprietary and must never move into civil-os.
- **New SDK languages:** non-JS SDKs.
- **Service extras:** audit logging and OPA authorization.
- **Dependency automation:** Renovate or Dependabot.

## Constraints

- **Nothing customer- or jurisdiction-specific goes in civil-os, ever.** That
  covers configs, data, names and hostnames. Test fixtures and the compose
  quickstart use synthetic data only.
- **Do not edit** the helm charts, the gitops repo or any jurisdiction-specific
  repo.
- **Archive the source repos only after the helm follow-up has moved the live
  clusters.** Until then, the old repos and old images keep serving the clusters.
- **Refactoring:** allowed only where it makes code testable and leaves behavior
  unchanged, for example interfaces in front of external clients. The required
  changes are the read/write split, the writer implementation and the AWS removal.
- **Agent access:** the agent never pushes to `main`; all work lands through PRs
  that the owner approves.
- **Timing:** no deadline and no freeze window.

## Success criteria

1. One commit can change a proto, its generated code and every consumer.
2. All four images build from civil-os under `ghcr.io/civil-labs/civil-os/<service>`.
3. Source history is preserved where practical (nice to have).
4. `docker compose up` works from a fresh clone and serves the full stack on
   synthetic data.
5. Every test layer (unit, handler, integration, e2e smoke, contract checks) passes
   in CI.
6. A release-please dry run produces both an app release and a Public API release.
7. The editor-to-writer rename is complete everywhere in the repo, and parcel
   updates persist end to end through the gateway.
8. The README, architecture overview and `CLAUDE.md` exist and are accurate.
9. No AWS *coupling* remains in the gateway: backend discovery uses no cloud
   API. *(Amended 2026-11-14 by owner decision during PR 2 review: optional
   S3 support for instance metadata, via gocloud.dev `s3blob`, stays and still
   pulls in the AWS SDK. Originally "No AWS SDK dependency remains in the
   gateway.")*

## Manual steps for the owner (GitHub and npm configuration)

The repo will document exactly what to set for each step. None of them can be
done by the agent.

1. Create the `civil-labs` npm org. Add an `NPM_TOKEN` repo secret, or configure
   npm trusted publishing for the civil-os repo.
2. Set up branch protection on `main`: require PRs, the CI checks and owner
   approval; allow squash merges only.
3. Install a release-please GitHub App or token, so release PRs and tags can
   trigger the image and SDK workflows. Tags created with `GITHUB_TOKEN` don't
   trigger other workflows.
4. Grant GHCR package permissions. The first push creates the
   `civil-os/<service>` packages; set their visibility to public.
5. Make the repo public once the first release is verified.
6. Later, after the helm follow-up: switch civil-lvt-shift to npm, then archive the
   source repos, civil-api-go and civil-api-js, each with a README pointer to
   civil-os.

## Open questions (carry into spec)

1. **Migration ConfigMap publishing.** The old workflow commits the ConfigMap back
   to the repo, which conflicts with a protected `main`. Should the ConfigMap be
   attached to the GitHub Release as an asset instead?
2. **Breaking-change policy for the Public API before 1.0.** Should `buf breaking`
   against the last `public-api-v*` tag warn or fail?
3. **Name of the civil-api-js repo.** Its git remote is `civil-api-ts`. Confirm
   which repo is the real one before archiving.
4. **Git history import.** Decide whether history is imported, given it's only
   nice to have and may complicate the restructure (renames, `internal/` move).
