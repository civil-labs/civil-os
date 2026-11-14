# GitHub and npm setup

These are the one-time settings the repository needs, outside the code, for CI
and releases to work. Only a repository or organization admin can make them.

## 1. Merge settings and branch rules

In **Settings → General → Pull Requests**:
- [ ] Allow **squash merging** only, with the default commit message set to
  **"Pull request title"**. release-please reads the squashed title as a
  conventional commit.
- [ ] Turn off merge commits and rebase merging.

In **Settings → Rules → Rulesets**, add one ruleset for `main`:
- [ ] Require a pull request, with at least one approval, before merging.
- [ ] Allowed merge method: squash.
- [ ] Block force pushes and deletions.
- [ ] Require these status checks to pass. They appear in the list once they've
  run at least once:
  - `Go build, vet, unit and integration tests`
  - `Protos, generated code and breaking changes`
  - `JS SDK packs and imports`
  - `E2E (docker compose)`
  - `No customer-specific data`
  - `conventional-title`
- [ ] Bypass list: repository admins only, if anyone. The release-please App
  doesn't need to bypass, because its release PRs are merged like any other.

> **History import (done once, in PR #1).** The pre-monorepo histories were
> brought in with merge commits, and PR #1 was merged with "Create a merge
> commit". A squash would have flattened that history. If a source repo is ever
> re-imported, turn merge commits on for that one PR, then off again.

## 2. Repository secrets

In **Settings → Secrets and variables → Actions → Repository secrets**:

| Secret | Used by | Value |
|---|---|---|
| `RELEASE_PLEASE_APP_ID` | `release-please.yml` | The App ID of the GitHub App in step 3 |
| `RELEASE_PLEASE_PRIVATE_KEY` | `release-please.yml` | A private key (`.pem` contents) generated for that App |
| `CUSTOMER_DENYLIST` | `ci.yml` | Newline-separated terms that must never appear in the repo: jurisdiction names, deployment codes, hostnames. Matched case-insensitively as fixed strings. CI prints only file names, never the matching terms. |
| `BUF_TOKEN` *(optional)* | `ci.yml`, `public-api-release.yml` | A buf.build token, if remote-plugin rate limits become a problem |

Workflows triggered from forks don't receive secrets. The denylist check skips
with a warning in that case, and those PRs need a maintainer's review.

## 3. release-please GitHub App

Tags created with the default `GITHUB_TOKEN` don't trigger other workflows. A
GitHub App token makes the release tags start the image, migration and npm
workflows.

- [ ] Create a GitHub App under the `civil-labs` organization, with no webhook
  and these repository permissions:

  | Permission | Access |
  |---|---|
  | Contents | Read and write |
  | Pull requests | Read and write |
  | Issues | Read and write (for release labels) |
  | Metadata | Read (set automatically) |

- [ ] Install it on the `civil-os` repository only.
- [ ] Save its App ID and a private key as the secrets in step 2.

## 4. GitHub Container Registry

The first push from `images.yml` creates the packages
`ghcr.io/civil-labs/civil-os/{gateway,database-reader,database-writer,tile-server}`.

For each package, in **Organization → Packages → *package* → Package settings**:
- [ ] Set **visibility** to public.
- [ ] Under **Manage Actions access**, make sure `civil-os` has **Write** access.
- [ ] Link the package to the `civil-os` repository, if GitHub didn't already.

## 5. npm

The `public-api-release.yml` workflow publishes `@civil-labs/civil-os-api` with
[trusted publishing](https://docs.npmjs.com/trusted-publishers), so there's no
long-lived npm token.

- [ ] Create the **`civil-labs`** organization on npmjs.com (the free plan
  covers public packages).
- [ ] Turn on two-factor authentication for your npm account. npm requires it
  to publish.
- [ ] Publish a placeholder once, because npm only shows trusted-publisher
  settings for a package that exists. Use version `0.0.0`, so it doesn't
  collide with a real release:

  ```sh
  mise run generate
  cd sdk/js
  npm login
  npm pkg set version=0.0.0
  npm publish --access public --provenance=false
  git checkout package.json
  npm deprecate @civil-labs/civil-os-api@0.0.0 "Placeholder; use 0.1.0 or later"
  ```

- [ ] In **@civil-labs/civil-os-api → Settings → Trusted Publisher**, add
  **GitHub Actions** with:
  - organization `civil-labs`;
  - repository `civil-os`;
  - workflow filename `public-api-release.yml`;
  - no environment;
  - allowed action **npm publish** only.
- [ ] Under **Publishing access**, choose **"Require two-factor authentication
  and disallow tokens"**.

If you'd rather use a token, add an `NPM_TOKEN` repository secret instead. The
workflow passes it to npm when it's set.

## 6. After the first releases

- [ ] The first release PRs propose `v0.2.0` for the app and
  `public-api-v0.1.0` for the public API.
- [ ] The `v*` tag publishes `:vX.Y.Z` images and attaches
  `migrations-configmap.yaml` to the release.
- [ ] The `public-api-v*` tag publishes to npm and attaches `openapi.yaml`.
- [ ] Clusters switch to the new image names and `CIVIL_OS_*` variables
  together, using the table in [architecture.md](architecture.md#configuration).
  That rollout happens outside this repository.
- [ ] Once clusters run civil-os images, archive the pre-monorepo repositories
  (the old service, schema and API repos), each with a README pointing here.
