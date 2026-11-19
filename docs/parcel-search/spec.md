# Spec: search parcels by address

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-18 |
| **Status** | Draft. The owner decided result scope, matching, owner names, `legacy_id` and `pg_trgm` availability on 2026-11-18. Normalisation, the query length limit, the result cap and the latency budget are proposals awaiting sign-off in review. Accepted when merged. |
| **Intent** | [`intent.md`](intent.md) |

This spec sets out the contract: the proto, matching and ranking, visibility,
pagination, the schema change, the fixtures and the tests. The order of the
work and how the PRs are split go in `plan.md`.

## Decisions on the intent's open questions

| Question | Decision |
|---|---|
| Scope of results | **Parcels only.** One RPC, `ParcelsService.SearchParcelsAddresses`, returning parcels with the address that matched. A search across record types would need a `oneof` result, ranking across kinds and a cursor over mixed kinds. If the records app needs one, it can be a later RPC, and adding it won't break this one. |
| Matching | **Trigram matching with a prefix boost** (`pg_trgm`), on a normalised form of the text (see [Matching and ranking](#matching-and-ranking)). Hits are ranked in four tiers: prefix, word prefix, substring, then similar (typo-tolerant). Within a tier they're ordered by trigram word similarity, and ties go to `parcel_id`. Pagination uses a keyset cursor over `(tier, similarity, parcel_id)`, with `legal_as_of` fixed by the first page. |
| Other keys (`legacy_id`) | **Deferred.** The public API doesn't expose `legacy_id` today, and the intent says search adds no new kind of data. Making jurisdiction parcel numbers public, and then searchable, is a decision of its own: [`../parcel-number-search/intent.md`](../parcel-number-search/intent.md), written in this spec's PR. |
| Address normalisation | **Proposed: punctuation and case only.** Text is lower-cased, every run of characters other than letters and digits becomes one space, and the result is trimmed. Abbreviations (`St` / `Street`), unit designators and accents aren't normalised. Trigram similarity covers some of that, and the rest is listed in [Known limits](#known-limits). Expanding abbreviations is locale-specific and belongs in a follow-up, driven by what records-app users actually type. |
| Extension availability | **`pg_trgm` is available in every deployment's PostGIS image** (owner, 2026-11-18), so there's no fallback. It's a contrib module in `postgis/postgis:18-3.6`, which compose and testcontainers use. It's declared in the Atlas schema in the `extensions` schema, next to `postgis`, `btree_gist` and `pgcrypto`. |
| Latency budget | **Proposed: p95 ≤ 200 ms and p99 ≤ 500 ms**, measured at the reader (the mesh RPC, not through the gateway), for first pages of 25. The dataset is generated synthetically at 500,000 visible parcels and 500,000 addresses, with a fixed query set (see [Performance check](#performance-check)). It's checked locally before the feature PR merges, not in CI. |

## Further decisions

| Question | Decision |
|---|---|
| Owner names | **Not searched** (owner, 2026-11-18). This narrows the intent's outcome 2, which listed current owners' names, and its fixture items for an owner with several parcels and a voided party. Search matches only the parcel's current `formatted_address`. Owner-name search makes it much easier to list everything one person owns, and a mixed result list ranks two fields that can't be compared. Clients can still go from a party to their parcels with `PartiesService.GetParcelsByPartyId`. Adding owner names later is a new intent. Adding a field to the hit then doesn't break anything. |
| Where the RPC lives | **On `ParcelsService`**, because it returns parcels. Adding an RPC doesn't break anything. On the mesh it's `ParcelReadService.SearchParcelsAddresses`, which keeps that service's current name, as `GetParcelExtentsWGS84ByParcelId` did ([`../mesh-plural-service-names/intent.md`](../mesh-plural-service-names/intent.md)). |
| One hit per parcel | A parcel has one address at `t`, so it appears at most once. Several parcels sharing an address each appear, ordered by `parcel_id` within the same rank. |
| Minimum query | **Proposed: at least 3 characters after normalisation**, and at most 200 characters before it. Shorter queries can't use the trigram index selectively, and in a jurisdiction they would match most rows. The gateway checks the raw length (1..200). The reader checks the normalised length, because only it normalises, and returns `InvalidArgument` (`query must contain at least 3 letters or digits`). |
| Result cap | **Proposed: 1,000 hits per query.** Ranking can't stream: every candidate has to be scored before the first page can be cut. The cap bounds that work for broad queries such as `fixture way`. `has_next_page` is false after the 1,000th hit. The cap is documented on the RPC, because a client paging past it is told to refine the query. |
| No edges | **Results are `repeated ParcelSearchHit hits`, with no edge wrapper and no cursor on each hit** (owner, 2026-11-18). Paging only needs `page_info.end_cursor`. Every civil-os cursor is a keyset position, not a reference to a row, so a client that edits its cached list can still continue from `end_cursor`. The only thing a cursor on each result adds is resuming from the middle of a page, which nothing needs. The edge shape came from the Relay connection spec, but Relay-style client libraries (Relay, Apollo, urql) are GraphQL tools and can't read Connect responses. Connect clients, such as `@connectrpc/connect-query`'s infinite queries, read the next cursor from `page_info`. This is the first paginated RPC without edges. [`../pagination-without-edges/intent.md`](../pagination-without-edges/intent.md), written in this spec's PR, removes edges from the existing paginated RPCs. |
| Score in the response | **Not part of the contract.** The tier and the similarity are ranking internals and may change with tuning. Clients get the order. `end_cursor` carries them (see [Pagination](#pagination)), because base64 isn't encryption, but clients must not parse cursors or rely on what's in them. |
| Highlighting | **Not in this feature.** `formatted_address` is the stored text, so a client can highlight the query in it itself. |
| Idempotency | `idempotency_level = NO_SIDE_EFFECTS`, so it can be called with Connect GET, like the other new reads. |

## Public API: `civil.public.parcels.v1`

```proto
service ParcelsService {
  // ...existing RPCs...

  // Searches visible parcels by their address, as of legal_as_of. Results
  // are ranked, best first: an address starting with the query, then one
  // with a word starting with it, then one containing it, then one similar
  // to it. Matching ignores case and punctuation. At most 1,000 hits are
  // returned across all pages. Refine the query to see others.
  rpc SearchParcelsAddresses(SearchParcelsAddressesRequest) returns (SearchParcelsAddressesResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message SearchParcelsAddressesRequest {
  // Free text: part of an address. 1..200 characters. It must contain at
  // least 3 letters or digits once punctuation is removed.
  string query = 1 [(buf.validate.field).string = {min_len: 1, max_len: 200}];
  optional google.protobuf.Timestamp legal_as_of = 2;   // as on the other parcel reads
  optional int32 first = 3;    // 1..100, 25 when unset
  optional string after = 4;
}

message SearchParcelsAddressesResponse {
  // Best first. Fewer than `first` only on the last page.
  repeated ParcelSearchHit hits = 1;
  // start_cursor and end_cursor mark the first and last hit. Pass end_cursor
  // as `after` to read the next page.
  civil.public.pagination.v1.PageInfo page_info = 2;
}

message ParcelSearchHit {
  string parcel_id = 1;                       // uuid
  optional int64 feature_id = 2;              // geometry version at legal_as_of
  string address_id = 3;                      // uuid, the address that matched
  // The address that matched at legal_as_of, as stored (not normalised).
  string formatted_address = 4;
}
```

`legal_as_of` has the same validation and description as on
`GetParcelsWithImprovementSummaryByParcelIdRequest`, and `first` the same as
on the reverse lookups. `feature_id` is `optional`, as on `AddressParcel`.

`PageInfo` is the shared `civil.public.pagination.v1` message, unchanged. Its
comments define `start_cursor` and `end_cursor` as the cursors of the first
and last *edge*. This feature rewords them to say first and last *result*,
which is true of both shapes, as a comment-only change that doesn't break
anything. `start_cursor` is kept for symmetry and for backward pagination,
if it's ever added.

Every ID in a hit can be read with an existing RPC:

| Field | RPC |
|---|---|
| `parcel_id` | `GetParcelsWithImprovementSummaryByParcelId` |
| `address_id` | `AddressesService.GetAddressesById` |

The visibility rules below guarantee this.

## Matching and ranking

**Normalisation** is one immutable SQL function, used both in the index and in
the query, so the planner can match them:

```sql
-- core.search_normalize(text) → text
lower(btrim(regexp_replace(input, '[^[:alnum:]]+', ' ', 'g')))
```

`[:alnum:]` follows the database's locale, so letters with accents count as
letters but aren't folded. The function is `IMMUTABLE`, `PARALLEL SAFE` and
`STRICT`, and it sets its own `search_path` (`pg_catalog`). It's named for
search in general, not for addresses, so a later key such as the parcel
number can reuse it.

**Candidates** are the normalised `formatted_address` `t` of every visible
address on a visible parcel, against the normalised query `q`. A candidate
matches when `q <% t` (trigram word similarity at or above the threshold)
**or** `t LIKE '%' || q || '%'`. The reader escapes `%`, `_` and `\` in `q`,
although normalisation already removes them. Both conditions are served by
the GIN trigram index below.

**Rank** is computed per candidate:

| Tier | Condition | Example: query `2 fixture` |
|---|---|---|
| 0 | `t LIKE q || '%'` | `2 fixture way testville zz000` |
| 1 | `' ' || t LIKE '% ' || q || '%'` | `unit 4 2 fixture way …` (not in the fixtures) |
| 2 | `t LIKE '%' || q || '%'` | `12 fixture way testville zz000` |
| 3 | `q <% t` only | `3 fixture way …` (similar, not containing) |

Order: `tier ASC, word_similarity(q, t) DESC, parcel public_id ASC`.
`word_similarity` returns `real`, which is deterministic for the same inputs,
so the order is total and stable.

**The threshold** is pg_trgm's `word_similarity_threshold`, set with
`SET LOCAL` in the reader's transaction. It's the default, 0.6, which the plan
checked against the fixtures (owner, 2026-11-19). A two-letter swap such as
`fixtrue way` scores 0.5 and isn't found. Lowering the threshold to 0.5 would
find it, but then `old way` matches every fixture address. Dropped, doubled or
swapped-neighbour typos such as `fxture way` (0.73) and `fixture wya` (0.75)
are found. It's a reader constant, not an env var:
changing it changes results, and that should be a reviewed code change.

## Visibility rules

At time `t` (`legal_as_of`, or the snapshot from the first page; see
[Pagination](#pagination)):

1. **The parcel** is visible under the existing parcel rule: not voided, with
   a `parcel_geometry` row and a `parcel_attributes` row at `t`.
2. **The address** counts only when the parcel's attributes at `t` name it,
   the address isn't voided, and it has an `address_attributes` row at `t`.
   That row's `formatted_address` is what's matched and returned.

So a voided parcel or address never appears, and neither does text from a
historical row, such as address 5's old line now. Each was the intent's second
success criterion, apart from the voided party, which no longer applies.

## Pagination

As in [`../real-property-transfers-api/spec.md`](../real-property-transfers-api/spec.md#pagination):
forward only, `first` validated to 1..100 at the gateway and clamped by the
reader, opaque base64url cursors owned by the reader, keyset queries, and
`InvalidArgument` with the shared message for any bad cursor. The difference
is that the reader encodes only two cursors per page, the first and the last
hit, into `start_cursor` and `end_cursor`, and no cursor goes on each result.

```proto
// civil.mesh.pagination.v1
message SearchCursor {
  uint32 version = 1;                       // 1
  uint32 tier = 2;
  float similarity = 3;                     // word_similarity of the last hit
  string last_parcel_id = 4;
  bytes filter_hash = 5;
  google.protobuf.Timestamp as_of = 6;      // the t every page resolves at
}
```

- **Snapshot.** When `legal_as_of` is unset, the first page resolves "now"
  once and puts it in every cursor, so later pages rank against the same
  rows. This is stronger than `IdCursor`, which resolves "now" on each page.
  It's needed here because a rank, unlike an ID, can move between pages.
  Writes that land between pages, with a range that covers `as_of`, can still
  move a parcel. That's accepted, as in the other walks.
- **The filter hash** covers the RPC name, the **normalised** query and
  `legal_as_of` (set or unset, and its value). Queries that normalise the same
  (`12 Fixture Way` and `12, fixture-way`) share cursors. A cursor from
  another RPC or another query fails.
- **The keyset condition** is `(tier, -similarity, public_id) > ($tier,
  -$similarity, $last_parcel_id)`, with `$similarity` compared as `real`, and
  `LIMIT first + 1`. The 1,000-hit cap is applied before the keyset, inside
  the ranked candidate set, so each page sees the same capped list.
- **Decoding treats the cursor as untrusted input.** The filter hash is a
  plain SHA-256 of public inputs, as in the existing cursors. It catches a
  cursor reused by mistake, but a client can forge one. The reader therefore
  checks every decoded field before anything reaches SQL:
  - `version` is 1, and `filter_hash` matches the request;
  - `tier` is 0–3;
  - `similarity` is finite (not NaN or infinite) and within [0, 1];
  - `last_parcel_id` is a UUID;
  - `as_of` is set and passes the same check as `legal_as_of` (after year 0).
    When the request sets `legal_as_of`, `as_of` must equal it.

  Any failure is `InvalidArgument` with the shared cursor message. A cursor
  that passes these checks can only ask for what a client could already ask
  for: a position in the capped list, or a snapshot time, which
  `legal_as_of` allows anyway. So cursors aren't signed. An HMAC would need a
  key shared by every reader replica, a new env var and a rotation plan,
  and it would protect nothing these checks don't.

## Schema and migration

In `database/schema/schema.pg.hcl`:

- `extension "pg_trgm" { schema = schema.extensions }`.
- The function `core.search_normalize(text)` as above.
- One GIN expression index, `idx_address_attributes_formatted_address_trgm`,
  on `core.search_normalize(formatted_address)` with
  `extensions.gin_trgm_ops`.

These are a generated migration with `atlas.sum`, checked with
`mise run db:validate` by the owner. On a jurisdiction-sized table the index
takes some time to build. The plan decides whether the migration needs
`CREATE INDEX CONCURRENTLY`, which Atlas runs outside a transaction, and says
so in the PR. Nothing else in the schema changes, and no column is added.

The reader's unqualified names rely on the database `search_path`
(`core, geo, extensions, public`), so `<%` and `word_similarity` resolve from
`extensions` as PostGIS functions do today.

## Reader query shape

One query in `services/database-reader/parcel_search.go`:

1. **Candidates:** `address_attributes` at `t`, filtered on the trigram
   condition first so the GIN index drives it. They're joined to non-voided
   addresses, and through `parcel_attributes.address_id` at `t` to visible
   parcels (rules 1 and 2).
2. **Rank** each candidate (tier and similarity).
3. **Cap:** order by rank and `LIMIT 1000`.
4. **Page:** apply the keyset condition and `LIMIT first + 1`, then join each
   page row's `feature_id`.

The plan checks the query with `EXPLAIN (ANALYZE, BUFFERS)` on the
performance dataset, and confirms that it uses the trigram index.

## Mesh and gateway

`civil.mesh.parcels.v1` gets the same RPC and messages, without validation or
gnostic annotations, as in the other mesh packages. The gateway validates
the request, maps it one-to-one in `services/gateway/parcels.go`, and passes
cursors through unchanged.

## Fixtures

No new fixtures. The existing addresses, `N Fixture Way, Testville ZZ000` on
parcel `n`, cover what the intent asks for once owner names are dropped:

| Need | Fixture |
|---|---|
| Partial match and ranking across tiers | `2 fixture`: parcel 2 at tier 0, then parcel 12 at tier 2 (`12 fixture way` contains it). Voided parcel 20 never appears. |
| Case and punctuation | `2, FIXTURE-way` returns the same order |
| Typo tolerance | `fxture way` returns parcels at tier 3 only |
| Every parcel at one tier | `fixture way`: every visible parcel at tier 1 |
| History | address 5's old line, `5 Old Fixture Way`, until 2023-01-01. Now, `5 Old Fixture` matches address 5's current line only as a tier 3 hit (0.71). |
| Address shared with an invisible parcel | address 2 is also on parcel 21, which has no geometry |
| Voided parcel | parcel 20 and its address 20 |

A voided address is made in the test (committed, since each test gets its own
database copy), as in the address/party tests. The fixture header doesn't
change.

## Tests

| Layer | Coverage |
|---|---|
| Gateway handler tests | An empty query and a 201-character query are rejected without calling the mesh. So are `first` of 0 and 101, and a `legal_as_of` before year 0. An unset `first` reaches the mesh as 25, and cursors pass through unchanged. A fully populated `ParcelSearchHit` round-trips through the mapping. |
| Normalisation | An integration test calls `core.search_normalize` on case, punctuation, runs of spaces, leading and trailing junk, and digits. A query of `--` or `a.b` fails with `InvalidArgument`. |
| Reader integration: matching | `2 fixture` returns parcel 2 then parcel 12 as its first two hits. Parcel 2's hit carries address 2 and `2 Fixture Way, Testville ZZ000`. `2, FIXTURE-way` returns the same hits in the same order. `fxture way` returns hits, all at tier 3. `fixture way` returns all 19 visible parcels with a geometry, in `parcel_id` order within the tier, unless similarity separates them. |
| Reader integration: visibility | No hit is ever parcel 20 or 21. `5 Old Fixture` never returns the old line now: no hit carries `5 Old Fixture Way`, and parcel 5 can appear only at tier 3 with its current address. At 2022, parcel 5 is the first hit and carries the old line. An address voided in the test no longer matches, and its parcel disappears from results. |
| Reader integration: pagination | `fixture way` with `first = 5` walks every visible parcel exactly once through `end_cursor`, in the same order as a single `first = 100` page. A page's `start_cursor` passed as `after` returns that page minus its first hit. An empty page has neither cursor. A cursor reused with another query, or with a query that normalises differently, fails with `InvalidArgument`, but one reused with `12, FIXTURE-way` after `12 fixture way` works. When `legal_as_of` is unset, an address reworded between pages (in the test) doesn't change page 2, because of the snapshot. A malformed or unknown-version cursor fails, and `first = 500` sent directly to the reader is clamped to 100. |
| Reader unit tests: forged cursors | Cursors built in the test with a correctly recomputed filter hash fail with `InvalidArgument` when:<ul><li>`tier` is 4;</li><li>`similarity` is NaN, +Inf, −0.1 or 1.1;</li><li>`last_parcel_id` isn't a UUID;</li><li>`as_of` is unset, or before year 0;</li><li>`as_of` differs from a `legal_as_of` set on the request.</li></ul>A forged cursor with valid values is accepted, and it returns the hits after that position. |
| Reader integration: cap | With the cap lowered through a test-only reader option to 10, `fixture way` returns exactly 10 hits across pages, and the last page has `has_next_page = false`. |
| Migration test | The existing migration suite applies the new migration. A test checks that `pg_trgm` is installed in `extensions` and that the index exists. |
| e2e (`test/e2e`) | One smoke search through the gateway with a bearer token: `3 Fixture Way` returns parcel 3 first. |

## Performance check

The intent's third success criterion. It runs locally, not in CI: a
generator script in `dev/` (synthetic, seeded, deterministic) loads the
dataset sizes in the decision table into a scratch compose database, with
invented street names drawn from fixed word lists. A Go benchmark, or a small
`dev/` tool, runs a fixed set of queries against the reader:

- house-number prefixes;
- street fragments;
- full addresses;
- one-typo street names;
- a broad, cap-hitting query.

It reports p50, p95 and p99. The results, the machine and the dataset sizes go
in the feature PR's description. If the budget isn't met, the PR doesn't
merge until either the query changes or the owner amends the budget here.

## Docs to update

- `api/proto/public/README.md`: `SearchParcelsAddresses` on
  `ParcelsService`. In "Pagination":
  - New paginated RPCs return `repeated <Thing> <things>` with `page_info`
    and no edges. The existing RPCs still have edges until
    [`../pagination-without-edges/intent.md`](../pagination-without-edges/intent.md)
    removes them.
  - Search pages share a snapshot of `legal_as_of`, unlike the other walks.
- `api/proto/public/civil/public/pagination/v1/pagination.proto`: reword the
  `start_cursor` and `end_cursor` comments from "edge" to "result".
- `docs/architecture.md`: note `pg_trgm` among the database's extensions. No
  new env vars, so the config table doesn't change.

## Known limits

- **Abbreviations and units.** `12 Fixture St` and `12 Fixture Street` share
  most trigrams, but not all, so they rank lower than exact text. Unit
  designators (`Apt`, `#`, `Unit`) aren't understood.
- **Accents.** `[:alnum:]` keeps accented letters, but `é` and `e` are
  different trigrams. `unaccent` isn't immutable as shipped, so folding needs
  a wrapper of its own. That's a follow-up if users need it.
- **Owner names and mailing addresses** aren't searched. Only the parcel's
  own address is.
- **The cap** hides hits past 1,000. A broad query in a large jurisdiction
  says "refine your search" rather than paging forever.
- **Ranking between pages** can shift if data is written, with a legal range
  covering the snapshot, while a client is paging.

## Out of scope

- Searching owner names.
- Searching `legacy_id` ([`../parcel-number-search/intent.md`](../parcel-number-search/intent.md)).
- Searching other record types as results.
- Filters (by land use, neighbourhood or area).
- Abbreviation expansion, accent folding, highlighting and spelling
  suggestions.
- Backward pagination and `totalCount`.
- `system_as_of`.
- Any write path.
