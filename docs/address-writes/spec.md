# Spec: address write APIs

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-27 |
| **Status** | Draft. Every decision below was made by the owner in the brainstorm of 2026-11-26/27. Accepted when merged. |
| **Intent** | None. This spec was written without one. |

This spec adds the first write APIs for a bitemporal record: create, correct,
change, retire and void an address. It is also the **template for later
write APIs** (parcels, improvements, parties, …), so the general rules are
stated once in [The write pattern](#the-write-pattern) and then applied to
addresses. The order of the work goes in `plan.md`.

## Background

- `core.addresses` holds an address's identity: `public_id`, `legacy_id`,
  `is_voided` and `voided_at`. It has no legal time.
- `core.address_attributes` holds the facts, each row valid over a
  `legal_valid_range` (`[)`, an open upper bound meaning "until further
  notice"). An exclusion constraint stops two rows overlapping in legal time
  for one address. `formatted_address` is `NOT NULL`; every other column is
  nullable.
- History triggers on both tables copy the old row, with its
  `system_valid_range`, into the `*_history` twin on every `UPDATE` or
  `DELETE`. System time costs the writer nothing extra.
- `parcel_attributes`, `improvement_attributes` and `party_attributes`
  reference `addresses` (`ON DELETE RESTRICT`), each with an index on
  `address_id`.
- The only write today is `ParcelsService.UpdateParcel`. It corrects the row
  valid now in place and never changes legal time
  ([`../monorepo-migration/spec.md`](../monorepo-migration/spec.md) §5.3, C1):
  a system-time correction under a name that doesn't say so.

## Decisions

| Question | Decision |
|---|---|
| Separate system-time and legal-time writes | **Decided.** A correction ("we recorded it wrong") and a change ("the world changed on date T") are different RPCs. No write API is called `Update`. |
| Deleting | **Decided.** Deletion is voiding, a system-time statement that the address never existed. Ending an address in legal time is a separate RPC, `RetireAddress`, in v1. |
| Void is permanent | **Decided.** No `UnvoidAddress` in v1. It can be added later if needed. |
| Voiding or retiring a referenced address | **Decided.** Rejected while a non-voided parcel, improvement or party refers to it (see each RPC for the legal times checked). |
| Partial writes | **Decided.** A `google.protobuf.FieldMask` ([AIP-134](https://google.aip.dev/134)). Fields named in the mask and left unset are cleared. |
| Responses | **Decided.** Writes return the resulting record, so clients don't need a second read. |
| Country and administrative area | **Decided.** Referenced by `public_id`, not code. RPCs that list countries and administrative areas are a separate intent. |
| `formatted_address` | **Decided.** Optional on writes. When it isn't supplied, the writer builds it from the parts in the USPS Publication 28 style ([Formatted address](#formatted-address)). |
| Placement | **Decided.** Public `AddressesService`; mesh `AddressesWriteService` on the database-writer. |
| Authorization | **Decided.** Out of scope. Authorization through OPA is its own future intent, and this spec doesn't depend on it. Writes pass the same OIDC check as reads. |
| `UpdateParcel` | **Decided.** It becomes `CorrectParcel` when the parcel write APIs follow this template. Not part of this spec. |
| Concurrency | **Decided.** An optional per-row `etag` ([Concurrency](#concurrency)). |
| Retries | **Decided.** AIP-155 request IDs are their own feature, [`../request-ids/intent.md`](../request-ids/intent.md). This spec doesn't depend on it ([Retries](#retries)). |
| A change dated in a gap | **Decided.** `ChangeAddress` reopens the address ([ChangeAddress](#changeaddress)). |
| Case in a built formatted address | **Decided.** The parts keep their case, not Pub 28's capitals ([Formatted address](#formatted-address)). |

## The write pattern

These rules apply to addresses here and to every later write API.

### Verbs

|  | System time: "we recorded it wrong" | Legal time: "the world changed" |
|---|---|---|
| **Add** | n/a. A record we forgot is a create with a past `effective_from`. | `Create<Thing>(effective_from)` |
| **Modify** | `Correct<Thing>(legal_as_of)`: rewrite one legal row in place. | `Change<Thing>(effective_from)`: a new legal row from T. |
| **Remove** | `Void<Thing>`: it never existed. | `Retire<Thing>(effective_to)`: it stopped existing at T. |

- A **correction** never moves a legal boundary. The history trigger keeps
  the old version.
- A **change** never rewrites a fact already recorded for another legal
  time. It splits the row that contains T.
- **Voiding** never touches the facts. Reads hide voided records, and every
  write treats them as not found.

### Request and response shape

- **Fields** go in a flat write message, `<Thing>Fields`, with every field
  `optional` and references as `public_id`s. It's separate from the read
  message because reads nest references (`Country { code, name }`) and
  writes name them by ID.
- **`update_mask`** lists the `<Thing>Fields` paths to write, using their
  proto field names. It is required and non-empty on `Correct` and `Change`.
  An unknown path is `InvalidArgument`. A path whose field is unset clears it,
  except where a field has a documented default (here, `formatted_address`).
  `Create` takes no mask: unset fields are null.
- **Responses** return a `<Thing>Version`: the read message as of the
  written legal row, that row's `legal_valid_from` and `legal_valid_to`
  (unset when open), and that row's new `etag`. A client editing
  bitemporal data needs the row's bounds, which the read API doesn't expose.
  `Void` returns an empty message, because nothing readable remains.
- **Legal times** take the same lower bound as the read API's `legal_as_of`
  (after 0001-01-01). Future times are allowed: a change can be scheduled.
- **`trace_id`** comes from `traceparent` or is generated, as in
  `UpdateParcel`.
- **Every write runs in one transaction** and first locks the identity row
  (`SELECT … FOR UPDATE`), so writes to one record are serialized.

### Concurrency

**Decided: an optional per-row `etag`, checked when sent** ([AIP-154](https://google.aip.dev/154)).
It asserts that the legal row the caller read hasn't changed since.

- **The token** is an opaque string that encodes two values. The first is
  the row's `legal_valid_from`, which locates it: the exclusion constraint
  makes it unique for the record. The second is the row's
  `system_updated_at`, which versions it. The record's ID is already in the
  request. The token isn't signed. A forged one can only fail the caller's
  own write, or skip a check the caller could skip by leaving it out. The
  reader and writer share its encoding in one internal Go package.
- **Reads** return the etag of the row they read (`Address.etag`). Writes
  return the etag of the row they wrote (`AddressVersion.etag`). A read with
  no row at its legal time returns nothing, so there's no etag.
- **The check.** When a write carries an etag, the writer first locks the
  identity row, then finds the record's attribute row starting at the
  token's `legal_valid_from`. If no such row exists, or its
  `system_updated_at` differs, the write fails with `Aborted` and changes
  nothing. Any edit to that row changes its `system_updated_at`, because
  the history trigger sets it to the transaction time: a correction, a
  `Change` that splits it, or a `Retire` that ends it. So the legal range
  itself needn't be compared.
- **Without an etag**, the last write wins. Scripts may leave it out.
- **Per RPC:**
  - `Correct`, `Change` and `Retire` check the row the token names. That's
    usually the row they act on. When it isn't, for example a `Change` at a
    T in another row, the check is still "what I read is current".
  - A `Change` that reopens an address in a gap usually has no etag, since
    there was nothing to read. If it sends one, it's checked the same way.
  - `Void` acts on the whole record. It ignores the token's row and checks
    only that the address isn't voided, which `NotFound` already covers.
- **What it gains:** an edit to the 2019 row doesn't make an editor's 2024
  etag stale.
- **What it doesn't catch:** a change to a row the caller didn't read but
  the write depends on. For example, `Change` copies unmasked fields from
  the row containing T, which may not be the row the caller read. The checks
  each RPC makes on the server (rows after a retirement, references) still
  apply.
- **Same-microsecond writes.** `system_updated_at` is the transaction's
  `now()`. Two writes to one row in the same microsecond would leave the
  same version. Writes to a record are serialized by the identity-row lock,
  so this is accepted.

Alternatives considered:

| Approach | How it works | Why not |
|---|---|---|
| Last write wins | Nothing is checked. | A form loaded before someone else's edit silently undoes it. The field mask narrows this to the fields both people sent, but doesn't remove it. The etag is optional, so scripts still get this behaviour. |
| Record-level etag | A hash over every row of the record. | An edit to any legal row makes every editor's token stale, even for rows they didn't touch. |
| Version counter | An integer `version` column, incremented on every write. | The same guarantee, plus a schema change on every table. `system_updated_at` already versions each row. |
| Expected values | The request carries the old value of each masked field, and the write fails if any differs. | Every request doubles in size, and every API needs per-field comparison code. |
| Pessimistic locks | A client checks a record out, edits it and releases it. | Needs lock leases, expiry and cleanup in a stateless API. A browser tab that closes leaves a record locked. |
| Database locks alone | `SELECT … FOR UPDATE` in the write. | Already done. It stops two concurrent requests interleaving, but not a read, a pause for editing, and then a write. |

### Retries

**Decided: request IDs ([AIP-155](https://google.aip.dev/155)) are their own
feature**, in [`../request-ids/intent.md`](../request-ids/intent.md). It will
add a `request_id` to every write request. This spec doesn't depend on it,
and `CreateAddressRequest` takes no client-chosen `address_id`.

Until then:

- **`Create` isn't safe to retry.** If its response is lost, a retry makes a
  second address. `legacy_id`, which is unique, guards imports.
- **The other writes are.** A repeated `Correct` writes the same values. A
  repeated `Change` finds T at the start of a row and fails. A repeated
  `Retire` or `Void` finds the work done and fails (see each RPC). The
  caller can treat that failure as success.

## Address RPCs

Added to `civil.public.addresses.v1.AddressesService`:

```proto
rpc CreateAddress(CreateAddressRequest) returns (AddressVersion) {}
rpc CorrectAddress(CorrectAddressRequest) returns (AddressVersion) {}
rpc ChangeAddress(ChangeAddressRequest) returns (AddressVersion) {}
rpc RetireAddress(RetireAddressRequest) returns (AddressVersion) {}
rpc VoidAddress(VoidAddressRequest) returns (VoidAddressResponse) {}
```

### Messages

```proto
// The writable fields of an address.
message AddressFields {
  // Built from the other fields when unset (see the spec's formatted address rules).
  optional string formatted_address = 1;
  optional string address_line_1 = 2;
  optional string address_line_2 = 3;
  optional string address_line_3 = 4;
  optional string locality = 5;
  optional string sublocality = 6;
  optional string postal_code = 7;
  optional string administrative_area_id = 8; // UUID
  optional string country_id = 9;             // UUID
  optional google.type.LatLng coordinates = 10;
}

message CreateAddressRequest {
  AddressFields fields = 1;
  google.protobuf.Timestamp effective_from = 2; // required
  optional string legacy_id = 3;
}

message CorrectAddressRequest {
  string address_id = 1;
  AddressFields fields = 2;
  google.protobuf.FieldMask update_mask = 3;    // required, non-empty
  optional google.protobuf.Timestamp legal_as_of = 4; // the row to correct; defaults to now
  optional string etag = 5;
}

message ChangeAddressRequest {
  string address_id = 1;
  AddressFields fields = 2;
  google.protobuf.FieldMask update_mask = 3;    // required, non-empty
  google.protobuf.Timestamp effective_from = 4; // required
  optional string etag = 5;
}

message RetireAddressRequest {
  string address_id = 1;
  google.protobuf.Timestamp effective_to = 2;   // required
  optional string etag = 3;
}

message VoidAddressRequest {
  string address_id = 1;
  optional string etag = 2;
}

message VoidAddressResponse {}

// An address as of one legal row, returned by writes.
message AddressVersion {
  Address address = 1;
  google.protobuf.Timestamp legal_valid_from = 2;
  optional google.protobuf.Timestamp legal_valid_to = 3; // unset when open
  string etag = 4;
}
```

Validation (protovalidate): UUIDs on every ID; legal times bounded as in
the reads; `coordinates` with latitude in [-90, 90] and longitude in
[-180, 180]; no string longer than 1,000 characters. Strings are stored as
sent, not trimmed. An `AddressFields` string set to `""` is stored as `""`, not
null. To clear a field, name it in the mask and leave it unset.

Read-side additions, all additive:

- `Address.etag = 12`.
- `Country.id = 3` and `AdministrativeArea.id = 3`, their `public_id`s, so
  a client can send back the reference it read.

### CreateAddress

Inserts an `addresses` row, with a `public_id` the database generates, and
one `address_attributes` row valid over `[effective_from, ∞)`, then returns
it.

- `country_id` and `administrative_area_id`, when set, must exist, or it
  fails with `InvalidArgument`.
- A `legacy_id` already in use, voided or not, is `AlreadyExists`.

### CorrectAddress

Rewrites, in place, the masked fields of the attribute row containing
`legal_as_of` (default now). `legal_valid_range` is not changed. Returns that
row.

- No row at `legal_as_of` (before the address began, after it retired, or
  in a gap): `FailedPrecondition`.
- Correcting a row's legal bounds, for example an effective date entered
  wrongly, is out of scope (see [Out of scope](#out-of-scope)).

### ChangeAddress

Records that the masked fields took new values at T = `effective_from`:

1. Find the row R whose range `[lo, hi)` contains T.
2. If `lo = T`, fail with `FailedPrecondition`: nothing would be split, and
   rewriting R is a correction. The message names `CorrectAddress`.
3. Otherwise, set R's range to `[lo, T)`, and insert a row over `[T, hi)`
   with R's values overwritten by the masked fields.

Rows after `hi` are untouched, so a change dated in the past doesn't erase
later changes, and a change scheduled in the future ends where the next one
begins. Returns the new row.

If no row contains T, because the address is retired or T falls between
rows, the change **reopens** the address. It inserts a row from T to the
start of the next row, or open if there is none. No row is being split, so
unmasked fields are null, and the formatted address is built if it isn't
sent. T before the address's first row works the same way and extends the
address back in legal time.

### RetireAddress

Records that the address stopped existing at T = `effective_to`:

1. Find the row R whose range `[lo, hi)` contains T. None:
   `FailedPrecondition` (already retired, or not yet begun).
2. `lo = T`: `FailedPrecondition`, since R would be empty.
3. Any row starting at or after `hi`: `FailedPrecondition`, naming the
   first. Retiring would have to delete recorded facts. The caller retires
   later, or corrects those rows first.
4. A non-voided parcel, improvement or party whose attribute row refers to
   the address over any legal time at or after T: `FailedPrecondition`,
   naming up to 10 of them. They would point to an address that doesn't
   exist.
5. Otherwise, set R's range to `[lo, T)`. Returns R.

Reads at or after T then leave the address out, as they do today for an
address with no row.

### VoidAddress

Sets `is_voided = true` and `voided_at = now()`. The attribute rows stay as
they are.

- An unknown or already voided address: `NotFound`.
- A non-voided parcel, improvement or party whose attribute row refers to
  the address at **any** legal time: `FailedPrecondition`, naming up to 10.
  A voided address never existed, so nothing may ever have been at it.

### Formatted address

When a write leaves `formatted_address` unset, the writer builds it:

- **On create**, whenever it isn't supplied.
- **On correct and change**, when the mask names `formatted_address`
  without a value, or names any other field that goes into it without also
  naming `formatted_address`. An edit to `address_line_1` then can't leave a
  stale formatted address. A custom formatted address is replaced when its
  parts change, unless the same request sends a new one.

The rule, after USPS Publication 28's single-line form:

```
{line 1} {line 2} {line 3}, {locality} {administrative area code} {postal code}
```

- Empty or unset parts are left out, with their separating space or comma.
  The comma appears only when there is something on both sides of it.
- Whitespace inside each part is collapsed to single spaces.
- `sublocality` and the country are left out. Pub 28 has no place for
  either, and every address in a deployment is in its jurisdiction.
- Parts are not standardized: `Street` isn't abbreviated to `ST`, and the
  address isn't checked against USPS data.
- Each part keeps the case it was stored in. Pub 28's capitals aren't
  applied: they read worse in the apps, and search already normalizes case.
- An address with nothing to build from is `InvalidArgument`.

The synthetic fixtures' `5 Fixture Way, Testville ZZ000` fits the rule
(with no administrative area code).

### Errors

| Code | When |
|---|---|
| `InvalidArgument` | Validation fails; the mask is missing, empty or names an unknown path; a referenced country or administrative area doesn't exist; no formatted address can be built. |
| `NotFound` | The address is unknown or voided (every RPC but `Create`). |
| `AlreadyExists` | `Create` with a `legacy_id` in use. |
| `Aborted` | The row the `etag` names is gone or has changed. |
| `FailedPrecondition` | No row at the legal time, T at the start of a row, rows after a retirement, or the address is referenced. The message says which. |
| `Unavailable` | The gateway has no writer configured (`CIVIL_OS_GATEWAY_WRITER_HOST` unset), as for `UpdateParcel`. |
| `Internal` | Anything else. The cause is logged, not returned. |

## Mesh, writer and gateway

- **Mesh:** `civil.mesh.addresses.v1.AddressesWriteService` with the same
  five RPCs and messages. `TestMeshServiceNames` accepts the name as it is.
- **Writer:** a new `addresses.go` with an `AddressServer`, registered in
  `main.go` alongside `ParcelServer`. It shares `traceIDFromTraceparent`
  and the check-violation mapping. The formatter is a pure function with
  its own unit tests.
- **Gateway:** `AddressServer` gains a writer client. The five handlers
  check the writer is configured, forward `traceparent` and map the
  messages. The reader's `GetAddressesById` gains `etag` and the two `id`s.
- **Schema:** no change. Every check uses existing columns and indexes.

## Documentation

- `api/proto/public/README.md`: the five RPCs in the services table, and a
  short "Writes" section on the verbs, the field mask and the `etag`.
- `docs/architecture.md`: the writer serves `ParcelsWriteService` and
  `AddressesWriteService`. "Corrections, not new facts" becomes a short
  account of correct, change, retire and void.
- `CLAUDE.md`, Rules: "Writes follow the pattern in
  `docs/address-writes/spec.md`: no `Update` verbs, a field mask, and
  version responses."

## Release

`feat: add address write APIs`. Every public change is additive, so
`buf breaking` passes and it's a minor release of the public API.

## Acceptance criteria

Writer integration tests (testcontainers PostGIS), each checking the
history rows the trigger wrote:

1. Create writes an identity row and one open row, and returns the ID, the
   bounds and an etag. A repeated `legacy_id` is `AlreadyExists`.
2. Correct rewrites only the masked fields of the row at `legal_as_of`. A
   masked, unset field is cleared. Bounds are unchanged. One history row is
   written.
3. Change on fixture address 5 (rows `[2020, 2023)` and `[2023, ∞)`) at
   2021 gives `[2020, 2021)`, `[2021, 2023)` and `[2023, ∞)`. The new row
   copies unmasked fields from `[2020, 2023)`. Change at exactly 2023 is
   `FailedPrecondition`. After retiring the address at 2025, a change at
   2026 reopens it with a row `[2026, ∞)` whose unmasked fields are null.
4. Retire ends the containing row, and a read after T leaves the address
   out. Retiring before a later row, or while a parcel refers to the address
   after T, is `FailedPrecondition`.
5. Void hides the address from every read. Voiding a referenced address is
   `FailedPrecondition`. Voiding twice is `NotFound`.
6. An etag read from the `[2023, ∞)` row is `Aborted` after that row is
   corrected, split or ended, and nothing changes. It still succeeds after an
   edit to the `[2020, 2023)` row. No etag succeeds. `Void` ignores the
   token's row.
7. The formatter's unit tests cover every combination of empty parts, the
   rebuild-on-mask rules and a custom value surviving an unrelated edit.
8. Gateway handler tests map each RPC, return `Unavailable` without a
   writer and forward `traceparent`.
9. The e2e smoke tests create, change, correct, retire and void an address
   through the gateway, and read it back after each step.

## Out of scope

- Authorization (a future OPA intent).
- RPCs that list countries and administrative areas (a separate intent).
- `UnvoidAddress`.
- Correcting a row's legal bounds (an effective date entered wrongly).
  A later `CorrectAddressEffectiveDates`, or a range on `CorrectAddress`.
- Reading every legal version of an address (`GetAddressVersions`), which
  an editing UI will want for a timeline.
- Address standardization or validation against USPS data.
- Batch writes.
- Request IDs for safe retries ([`../request-ids/intent.md`](../request-ids/intent.md)).
- Renaming `UpdateParcel` to `CorrectParcel`.
