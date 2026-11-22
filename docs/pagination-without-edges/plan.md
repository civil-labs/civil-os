# Plan: paginate without edges

| | |
|---|---|
| **Implements** | [`spec.md`](spec.md) (accepted in `9bd59eb`, #72), which implements [`intent.md`](intent.md) |
| **Author** | Claude (drafted for Jackson, owner / approving engineer) |
| **Date** | 2026-11-21 |
| **Status** | Draft. Accepted when merged. |

## Ground rules

- **Branches and PRs.** The civil-os work lands in one PR, as the spec
  requires, on its own short-lived branch. The agent never pushes to `main`.
  The owner merges.
- **Each step is one commit** with a conventional-commit message, and every
  commit passes `mise run check`. The PR is squash-merged, so the commits
  only matter for review.
- **Plan and code stay in sync.** If implementation departs from this plan,
  `plan.md` is updated in the same PR. If the departure changes the spec, the
  agent stops and asks the owner first.
- **Verification before "done".** The PR's checks are run locally, and their
  output goes in the PR description before the PR is reported as done.
- **Generated code is never edited by hand.** Proto changes go through
  `mise run generate`, and the regenerated `api/gen/go`, `internal/gen/go`
  and `sdk/js/gen` are committed with the `.proto` change.
- **Synthetic data only.** No change to `database/fixtures/synthetic.sql`
  (see step 1.2).
- **No Atlas, no migration.** Nothing touches the schema.

## Environment

Docker is reachable from the session (server 29.5.3 on 2026-11-21), so
`mise run test:integration` and `mise run test:e2e` run locally.

## PR overview

| PR | Repo | Branch | Title | Depends on | Release effect |
|---|---|---|---|---|---|
| 1 | civil-os | `feat/pagination-without-edges` | `feat!: return paginated results without edges` | — | public-api 0.4.1 → 0.5.0, Go module minor |
| 2 | opencama-record-keeper | `feat/civil-os-api-0.5` | `feat!: read civil-os 0.5 pages without edges` | PR 1 released to npm | records app minor |

PR 2 is listed here so the release order is in one place. It's planned and
reviewed in the records app's repo. Nothing in the deployment repos is
changed from here.

## Spec correction found while planning

**A commit footer won't reach the changelog.** The spec says the squash
commit carries a `BREAKING CHANGE:` footer with the migration note. The repo
squash-merges with the title as the subject and a **blank** body
(`squash_merge_commit_message: BLANK`), so a footer written in the branch's
commits is dropped. Instead, the PR description ends with a release-please
commit override, which release-please reads from the merged PR in place of
the squash commit:

```
BEGIN_COMMIT_OVERRIDE
feat!: return paginated results without edges

BREAKING CHANGE: Paginated responses no longer wrap results in edges. Read
`response.transfers` (or `parcels`, `improvements`, `parties`) instead of
`response.edges[].node`, and keep paging with `page_info.end_cursor`.
Per-result cursors are gone.
END_COMMIT_OVERRIDE
```

The owner checks that the release-please PR's `api/proto/public/CHANGELOG.md`
entry shows the note before merging it. If it doesn't, the override can be
edited on the merged PR and release-please re-run. The spec's Release
section is corrected to say this in the same PR as this plan. The contract
doesn't change.

## PR 1: civil-os (`feat/pagination-without-edges`)

### 1.1 Protos, reader, gateway and existing tests (`feat!: return paginated results without edges`)

One commit, because the generated code, the handlers and the tests that read
`Edges` don't compile apart.

**Public protos** (`api/proto/public/civil/public/`):

- `transfers/v1/transfers.proto`: delete `RealPropertyTransferEdge`. In
  `GetTransfersByParcelIdResponse`, `GetTransfersByFeatureIdResponse` and
  `GetTransfersByPartyIdResponse`, replace `repeated RealPropertyTransferEdge
  edges = 1` with `repeated RealPropertyTransfer transfers = 1`, with a
  comment giving the order (the RPC's existing order) and "Fewer than `first`
  only on the last page."
- `addresses/v1/addresses.proto`: delete `AddressParcelEdge` and
  `AddressImprovementEdge`. `GetParcelsByAddressIdResponse` gets `repeated
  AddressParcel parcels = 1`, `GetImprovementsByAddressIdResponse` gets
  `repeated AddressImprovement improvements = 1`, and
  `GetPartiesByAddressIdResponse` gets `repeated civil.public.parties.v1.Party
  parties = 1`. Reword any comment on `AddressParcel` or
  `AddressImprovement` that calls it a node or an edge.
- `parties/v1/parties.proto`: delete `PartyEdge` and `PartyParcelEdge`.
  `GetParcelsByPartyIdResponse` gets `repeated PartyParcel parcels = 1`.
- `pagination/v1/pagination.proto`: the header comment becomes "Pagination
  state for a page of results." The Relay link goes. Field comments stay.

**Mesh protos** (`api/proto/mesh/civil/mesh/`): the same changes in
`transfers/v1`, `addresses/v1` and `parties/v1`, with the same names and
numbers. In `pagination/v1`, the `PageInfo` comment drops "Relay-style" and
the link, and `IdCursor`'s comment says "a page-boundary cursor" instead of
"an edge cursor". `TransferCursor`'s comment in `transfers/v1` gets the same
rewording.

Then `mise run generate`, and `buf format --diff --exit-code` in `api/`.

**Reader** (`services/database-reader/`):

- `id_cursor.go`: `pageByID` returns `([]T, *paginationv1.PageInfo, error)`.
  It encodes cursors for `nodes[0]` and `nodes[len(nodes)-1]` only (one call
  when the page has one item), and sets `StartCursor` and `EndCursor` from
  them. The error for a failed encode stays `Internal: cursor encoding error`.
- `addresses.go`, `parties.go`: the four handlers put the nodes straight into
  `Parcels`, `Improvements` and `Parties`. The edge-building loops go.
- `transfers.go`: `pageTransfers` returns `([]*transfersv1.RealPropertyTransfer,
  *paginationv1.PageInfo, error)`, encoding the first and last key only. The
  three handlers set `Transfers`.

**Gateway** (`services/gateway/`):

- `transfers.go`, `addresses.go`, `parties.go`: delete `mapTransferEdge`,
  `mapAddressParcelEdge`, `mapAddressImprovementEdge`, `mapPartyEdge` and
  `mapPartyParcelEdge`. Each node mapping inside them becomes a named mapper
  (`mapTransfer` already exists; add `mapAddressParcel`,
  `mapAddressImprovement` and `mapPartyParcel`, and reuse `mapParty`), used as
  `mapSlice(meshRes.Msg.Transfers, mapTransfer)` and so on.

**Existing tests:**

- `services/database-reader/transfers_integration_test.go`,
  `extents_party_transfers_integration_test.go`,
  `address_party_integration_test.go`: read the new fields in place of
  `Edges[i].GetNode()`. The helpers (`parcelsAtAddress`, `partyParcels` and
  the others) return the field directly.
- The checks that compare `start_cursor` or `end_cursor` with an edge's
  cursor (`transfers_integration_test.go:360`,
  `address_party_integration_test.go:288`, and
  `test/e2e/smoke_test.go:249`) instead check that both are set on a
  non-empty page, and that `after = end_cursor` returns the next item and
  nothing already seen. The existing walks already do the second half.
- `services/gateway/transfers_test.go`: build the mesh response with
  `Transfers` and check the public one has the same items and `PageInfo`.
- `test/e2e/smoke_test.go`: the transfers, address and party checks read the
  new fields. The multi-page `GetParcelsByPartyId` walk stays.
- Cursor unit tests (`*_cursor_test.go`) don't change.

**Check:** `mise run check`, `mise run test:integration`.

### 1.2 Walks for the address lookups (`test: walk every page of the address lookups`)

The spec asks for a full walk on every paginated RPC. Today the transfers
lookups and `GetParcelsByPartyId` have one. The three address lookups don't.
In the shared fixtures only address 3's improvements (3 and 16) give a
lookup more than one result. Address 2's second parcel (21) has no geometry,
so it's never visible, and no address has two parties.

Adding fixture rows would change counts that other tests read
(`database/migrations_integration_test.go`, the search tests). So the new
test, `TestAddressLookupPagination` in `address_party_integration_test.go`,
works on a private database (`tu.FixtureDatabase(t).Pool(t)`, as
`voidAddress` does):

- **Improvements at address 3:** walk with `first = 1`. Expect improvements
  3 then 16, `has_next_page` true then false, and `has_previous_page` false
  then true.
- **Parcels at address 3:** update visible parcel 4's attributes row so its
  `address_id` is address 3's, then walk with `first = 1`. Expect parcels 3
  then 4. It's an `UPDATE`, not an insert: `no_overlapping_parcel_attributes`
  forbids a second row over the same range.
- **Parties at address 3:** update party 4's attributes row so its mailing
  address is address 3, then walk with `first = 1`. Expect parties 3 then 4.
- **Each walk** collects the IDs from every page and compares them with one
  `first = 100` call on the same database: the same IDs in the same order.

The updates are helpers next to `voidAddress`
(`legal_time_integration_test.go`), using fixture IDs from
`internal/testutil`.

**Check:** `mise run test:integration`.

### 1.3 README (`docs: describe pagination without edges`)

`api/proto/public/README.md`, Pagination section: replace the **Response**
bullet with the text in the spec's [Public README](spec.md#public-readme)
section, and delete the sentences about older RPCs returning edges and the
link to this intent. Nothing else in the section changes.

### 1.4 Verify and open the PR

Run, and paste the output into the PR description:

```sh
mise run check
mise run test:integration
mise run test:e2e
(cd api && buf format --diff --exit-code)
git diff --exit-code api/gen/go internal/gen/go   # after mise run generate
(cd api && buf breaking proto/public --against "../.git#ref=origin/main,subdir=api/proto/public")
grep -rnE 'Edge\b|edges' api/proto      # expect no output
```

`buf breaking` is expected to fail, listing the deleted messages and the
changed field 1 in all seven responses. The output goes in the PR
description to show that the break is exactly the one the spec intends. CI's
breaking-changes check passes with a warning because the title has `!`.

The PR description ends with the commit override from the spec correction
above, and says that the records app must move before a deployment picks up
this release.

### After the merge

- **The release-please PR** bumps the public API to 0.5.0. **The owner holds
  it** while the
  [`reverse-lookups-by-returned-type`](../reverse-lookups-by-returned-type/intent.md)
  move is still on its way into the same release (its spec and plan come
  next). Otherwise the owner merges it, which publishes
  `@civil-labs/civil-os-api` 0.5.0.
- Before merging it, the owner checks that its changelog entry carries the
  migration note.

## PR 2: the records app (`feat/civil-os-api-0.5`, in its own repo)

After `@civil-labs/civil-os-api` 0.5.0 is on npm:

- bump `@civil-labs/civil-os-api` to 0.5.0 in `package.json`;
- `src/components/PagedList.tsx`: `Page<T>` drops `edges`, and `PagedList`
  takes `rows: (page: P) => T[]`. The flattening becomes
  `query.data.pages.flatMap(rows)`;
- the five call sites (`TransfersList.tsx`, `PartyPage.tsx`, and three in
  `AddressPage.tsx`) pass `rows`, such as `(p) => p.transfers`;
- run its checks and `docs/mvp/smoke.md` against a civil-os stack on this
  release.

If the reverse-lookup move ships in the same release, PR 2 also switches the
four moved RPCs to their new services. It's still one records app PR.

**Deployment order:** a deployment picks up civil-os 0.5.x only together with
the records app release from PR 2, or after it. Both are pinned in the
deployment repos, which the owner changes.
