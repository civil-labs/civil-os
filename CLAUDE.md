# civil-os: notes for coding agents

A parcel-data platform as one Go module: gateway → database-reader /
database-writer / tile-server (Martin) → PostGIS. See `docs/architecture.md`.
This repo is **public**.

## Commands

```sh
mise run generate          # regenerate api/gen, internal/gen, sdk/js/gen from api/proto
mise run check             # go build, go vet, buf lint, unit + handler tests
mise run test:integration  # testcontainers PostGIS: reader, writer, migrations (needs Docker)
mise run up                # build + start the compose stack on synthetic data
mise run test:e2e          # starts/rebuilds the stack, then runs the smoke tests
TOKEN=$(mise run token)    # bearer token for the local test user
mise run down              # stop the stack and delete its database
mise run db:diff           # Atlas, local only: the owner runs it, never an agent (see below)
mise run db:validate       # Atlas, local only: the owner runs it, never an agent (see below)
```

CI also runs these, so run them before you push:
- `buf format --diff --exit-code` (in `api/`);
- a generated-code freshness check: `git diff --exit-code api/gen/go internal/gen/go` after `generate`;
- `buf breaking` on the public module.

PRs that only change `docs/` or `*.md` files skip CI's Go, protos, SDK and e2e
jobs (`scripts/changed-code.sh`). If a docs change affects anything a build or
test reads, run `mise run check` yourself.

## Layout

| Path | What |
|---|---|
| `api/proto/public` | Public API, versioned and released as `public-api-vX.Y.Z` + npm. Breaking changes need `!` in the PR title. |
| `api/proto/mesh` | Gateway ↔ backend API, internal, no compatibility promise |
| `api/gen/go`, `internal/gen/go` | **Generated, committed. Never edit by hand.** |
| `services/<svc>` | One `package main` per service, plus a `dockerfile` built from the repo root |
| `database/schema`, `database/migrations` | Atlas HCL schema; generated plain-SQL migrations + `atlas.sum` |
| `database/fixtures/synthetic.sql`, `internal/testutil` | Synthetic data and fixed IDs shared by tests and compose |
| `compose.yaml`, `dev/` | Local stack with throwaway credentials |
| `docs/<feature>/{intent,spec,plan}.md` | Design docs: an intent must be merged before its spec, and a spec before its plan |

## Rules

- **No customer or jurisdiction data, ever:** no real place names, hostnames,
  deployment codes, IDs or configs, in code, fixtures, docs or commit messages.
  Use the synthetic fixtures. CI greps the tree against a secret denylist.
- **Conventional commits.** PRs are squash-merged and the PR title becomes the
  commit that release-please reads. Use `feat:`, `fix:`, `docs:`, `test:`,
  `ci:`, `build:`, `refactor:` and `chore:`, with `!` for breaking changes.
  Only `feat`, `fix` and other user-facing types trigger a release.
- **Mesh naming:** `<Entity>ReadService` is served by the reader and
  `<Entity>WriteService` by the writer. New services use the plural public
  domain name (`AddressesReadService`, `PartiesReadService`); see
  `docs/mesh-plural-service-names/intent.md`. The public API keeps its own names.
- **Config:** env vars are `CIVIL_OS_<SERVICE>_<SETTING>`. Services refuse to
  start if a pre-rename `CIVIL_*` variable is set. Update the table in
  `docs/architecture.md` when you add one.
- **Never push to `main`.** Work on a branch and open a PR. The owner merges.
- **No Atlas in CI or compose.** It's licensed for local use only.
- **Agents never run Atlas**, even when it's logged in. Agents never run
  `db:diff`, `db:validate` or any `atlas` command, and never write files in
  `database/migrations`. Change the HCL, let the owner review it, and ask
  them to run the commands. Follow the `atlas-migrations` skill
  (`.claude/skills/atlas-migrations/SKILL.md`).
- **Don't touch the deployment repos** (Helm chart, gitops, jurisdiction repos)
  from here.

## Known mistakes

- Editing generated code. Change the `.proto` and run `mise run generate`.
- Forgetting that current reads hide voided parcels (`NOT p.is_voided`). Every
  new query needs that filter too.
- Assuming the tile server only serves `get_parcel_tiles`. Martin publishes
  every geometry table (`docs/tile-source-allowlist/intent.md`).
- `mise` refuses to run tasks in an untrusted checkout. Run `mise trust`, or set
  `MISE_TRUSTED_CONFIG_PATHS` the way CI does.
- release-please `extra-files` paths are relative to the package unless they
  start with `/`.
- protobuf-es output needs `import_extension=js` for Node ESM. It's already set
  in `api/buf.gen.public.yaml`.
- Adding a bind mount to `compose.yaml` without `z`, or dropping it as
  unexplained. SELinux hosts need it (`docs/compose-selinux-relabel/spec.md`).
  Never use it on a path outside the repo.
