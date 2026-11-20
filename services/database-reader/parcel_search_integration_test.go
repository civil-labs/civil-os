//go:build integration

package main

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for SearchParcelsAddresses (docs/parcel-search/spec.md, "Tests").

func searchReq(query string, first int32, after *string, legalAsOf *timestamppb.Timestamp) *parcelsv1.SearchParcelsAddressesRequest {
	req := &parcelsv1.SearchParcelsAddressesRequest{Query: query, After: after, LegalAsOf: legalAsOf}
	if first != 0 {
		req.First = &first
	}
	return req
}

func search(t *testing.T, r readerClients, req *parcelsv1.SearchParcelsAddressesRequest) *parcelsv1.SearchParcelsAddressesResponse {
	t.Helper()
	res, err := r.parcels.SearchParcelsAddresses(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("search %q: %v", req.Query, err)
	}
	return res.Msg
}

func searchErr(r readerClients, req *parcelsv1.SearchParcelsAddressesRequest) error {
	_, err := r.parcels.SearchParcelsAddresses(context.Background(), connect.NewRequest(req))
	return err
}

func hitIDs(hits []*parcelsv1.ParcelSearchHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.GetParcelId()
	}
	return out
}

// walk pages through a search with page size first and returns every hit.
func walk(t *testing.T, r readerClients, query string, first int32, legalAsOf *timestamppb.Timestamp) ([]*parcelsv1.ParcelSearchHit, []int) {
	t.Helper()
	var all []*parcelsv1.ParcelSearchHit
	var sizes []int
	var after *string
	for range 100 {
		page := search(t, r, searchReq(query, first, after, legalAsOf))
		all = append(all, page.Hits...)
		sizes = append(sizes, len(page.Hits))
		if !page.PageInfo.GetHasNextPage() {
			return all, sizes
		}
		after = page.PageInfo.EndCursor
	}
	t.Fatal("walk didn't end")
	return nil, nil
}

// decodeTestCursor reads a cursor's fields, which clients can't do, to check
// tiers and the snapshot.
func decodeTestCursor(t *testing.T, s string) *paginationv1.SearchCursor {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	var c paginationv1.SearchCursor
	if err := proto.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func TestSearchMatching(t *testing.T) {
	r := startReader(t)

	two := search(t, r, searchReq("2 fixture", 100, nil, nil))
	if got := hitIDs(two.Hits); len(got) < 2 || got[0] != tu.ParcelID(2) || got[1] != tu.ParcelID(12) {
		t.Fatalf("2 fixture = %v, want parcel 2 then 12 first", got)
	}
	h := two.Hits[0]
	if h.GetAddressId() != tu.AddressID(2) || h.GetFormattedAddress() != "2 Fixture Way, Testville ZZ000" || h.GetFeatureId() != tu.ParcelFeatureID(2) {
		t.Errorf("parcel 2's hit = %v", h)
	}
	if c := decodeTestCursor(t, two.PageInfo.GetStartCursor()); c.GetTier() != 0 {
		t.Errorf("parcel 2's tier = %d, want 0 (prefix)", c.GetTier())
	}

	punct := search(t, r, searchReq("2, FIXTURE-way", 100, nil, nil))
	twoWay := search(t, r, searchReq("2 fixture way", 100, nil, nil))
	if !slices.Equal(hitIDs(punct.Hits), hitIDs(twoWay.Hits)) {
		t.Errorf("2, FIXTURE-way = %v, want the same as 2 fixture way: %v", hitIDs(punct.Hits), hitIDs(twoWay.Hits))
	}

	typo := search(t, r, searchReq("fxture way", 100, nil, nil))
	if len(typo.Hits) == 0 {
		t.Fatal("fxture way returned nothing")
	}
	if first, last := decodeTestCursor(t, typo.PageInfo.GetStartCursor()), decodeTestCursor(t, typo.PageInfo.GetEndCursor()); first.GetTier() != 3 || last.GetTier() != 3 {
		t.Errorf("fxture way tiers run %d..%d, want 3 only", first.GetTier(), last.GetTier())
	}

	var want []string
	for n := 1; n <= 19; n++ {
		want = append(want, tu.ParcelID(n))
	}
	if got := hitIDs(search(t, r, searchReq("fixture way", 100, nil, nil)).Hits); !slices.Equal(got, want) {
		t.Errorf("fixture way = %v, want parcels 1..19 in order", got)
	}
}

func TestSearchVisibility(t *testing.T) {
	r := startReader(t)

	for _, q := range []string{"fixture way", "20 fixture way", "21 fixture", "2 fixture way", "testville"} {
		for _, id := range hitIDs(search(t, r, searchReq(q, 100, nil, nil)).Hits) {
			if id == tu.ParcelID(tu.VoidedParcel) || id == tu.ParcelID(tu.NoGeometryParcel) {
				t.Errorf("%q returned invisible parcel %s", q, id)
			}
		}
	}

	// Address 5's old line is history now. Its current line is still a weak
	// (tier 3) match, but no hit carries the old text
	// (docs/parcel-search/plan.md, step 2.5).
	now := search(t, r, searchReq("5 Old Fixture", 100, nil, nil))
	if len(now.Hits) != 1 || now.Hits[0].GetParcelId() != tu.ParcelID(tu.HistoricalAddress) ||
		now.Hits[0].GetFormattedAddress() != "5 Fixture Way, Testville ZZ000" {
		t.Errorf("5 Old Fixture now = %v, want only parcel 5 with its current line", now.Hits)
	} else if c := decodeTestCursor(t, now.PageInfo.GetStartCursor()); c.GetTier() != 3 {
		t.Errorf("parcel 5 now ranks at tier %d, want 3", c.GetTier())
	}
	before := at(tu.HistoryChange.Add(-24 * time.Hour))
	then := search(t, r, searchReq("5 Old Fixture", 100, nil, before))
	if len(then.Hits) == 0 || then.Hits[0].GetParcelId() != tu.ParcelID(tu.HistoricalAddress) ||
		then.Hits[0].GetFormattedAddress() != "5 Old Fixture Way, Testville ZZ000" {
		t.Errorf("5 Old Fixture before %s = %v, want parcel 5 first with the old line", tu.HistoryChange, then.Hits)
	}

	// Voiding address 3 removes parcel 3 from results.
	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 3)
	rv := startReaderOn(t, pool)
	if got := hitIDs(search(t, rv, searchReq("fixture way", 100, nil, nil)).Hits); slices.Contains(got, tu.ParcelID(3)) || len(got) != 18 {
		t.Errorf("after voiding address 3, fixture way = %v, want parcels 1..19 without 3", got)
	}
}

// Parcels that share an address are ordered by parcel_id within the rank,
// and pagination doesn't skip or repeat one across that tie. Parcel 21 shares
// address 2 with parcel 2. It has no geometry in the fixtures, so it gets one
// here.
func TestSearchSharedAddress(t *testing.T) {
	pool := tu.FixtureDatabase(t).Pool(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO geo.parcel_geometry (parcel_id, feature_id, geom_web, legal_valid_range, trace_id)
		SELECT p.parcel_id, 1021,
		       extensions.ST_Multi(extensions.ST_MakeEnvelope(-139.99, -39.99, -139.9896, -39.9896, 4326)),
		       tstzrange('2020-01-01 00:00:00+00', NULL), 'test'
		FROM core.parcels p WHERE p.public_id = $1`, tu.ParcelID(tu.NoGeometryParcel))
	if err != nil {
		t.Fatal(err)
	}
	r := startReaderOn(t, pool)

	single := search(t, r, searchReq("2 fixture way", 100, nil, nil))
	want := []string{tu.ParcelID(2), tu.ParcelID(tu.NoGeometryParcel)}
	if got := hitIDs(single.Hits); len(got) < 2 || !slices.Equal(got[:2], want) {
		t.Fatalf("2 fixture way = %v, want parcels 2 and 21 first, in that order", got)
	}
	for _, h := range single.Hits[:2] {
		if h.GetAddressId() != tu.AddressID(2) {
			t.Errorf("hit %s carries address %s, want address 2", h.GetParcelId(), h.GetAddressId())
		}
	}
	walked, _ := walk(t, r, "2 fixture way", 1, nil)
	if !slices.Equal(hitIDs(walked), hitIDs(single.Hits)) {
		t.Errorf("first=1 walk = %v, want %v", hitIDs(walked), hitIDs(single.Hits))
	}
}

func TestSearchPagination(t *testing.T) {
	r := startReader(t)

	single := search(t, r, searchReq("fixture way", 100, nil, nil))
	walked, _ := walk(t, r, "fixture way", 5, nil)
	if !slices.Equal(hitIDs(walked), hitIDs(single.Hits)) {
		t.Errorf("first=5 walk = %v, want %v", hitIDs(walked), hitIDs(single.Hits))
	}

	page := search(t, r, searchReq("fixture way", 5, nil, nil))
	if page.PageInfo.GetHasPreviousPage() || !page.PageInfo.GetHasNextPage() {
		t.Errorf("first page info = %v", page.PageInfo)
	}
	fromStart := search(t, r, searchReq("fixture way", 4, page.PageInfo.StartCursor, nil))
	if !slices.Equal(hitIDs(fromStart.Hits), hitIDs(page.Hits[1:])) {
		t.Errorf("after start_cursor = %v, want %v", hitIDs(fromStart.Hits), hitIDs(page.Hits[1:]))
	}
	if !fromStart.PageInfo.GetHasPreviousPage() {
		t.Error("a page read with after has has_previous_page = false")
	}

	empty := search(t, r, searchReq("maple way", 0, nil, nil))
	if len(empty.Hits) != 0 || empty.PageInfo.StartCursor != nil || empty.PageInfo.EndCursor != nil || empty.PageInfo.GetHasNextPage() {
		t.Errorf("maple way = %v, %v; want an empty page with no cursors", empty.Hits, empty.PageInfo)
	}

	// Cursors belong to the normalised query.
	twelve := search(t, r, searchReq("12 fixture way", 1, nil, nil))
	if err := searchErr(r, searchReq("2 fixture", 1, twelve.PageInfo.EndCursor, nil)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("cursor reused with another query: %v, want InvalidArgument", err)
	}
	if err := searchErr(r, searchReq("12, FIXTURE-way", 1, twelve.PageInfo.EndCursor, nil)); err != nil {
		t.Errorf("cursor reused with a query that normalises the same: %v", err)
	}
	if err := searchErr(r, searchReq("12 fixture way", 1, twelve.PageInfo.EndCursor, at(time.Now()))); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("cursor reused with legal_as_of set: %v, want InvalidArgument", err)
	}

	bad := []string{"not base64!", base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xff})}
	c := decodeTestCursor(t, twelve.PageInfo.GetEndCursor())
	c.Version = 2
	b, _ := proto.Marshal(c)
	bad = append(bad, base64.RawURLEncoding.EncodeToString(b))
	for _, cur := range bad {
		if err := searchErr(r, searchReq("12 fixture way", 1, &cur, nil)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("cursor %q: %v, want InvalidArgument", cur, err)
		}
	}

	// The reader clamps first, as the gateway would reject it.
	if got := search(t, r, searchReq("fixture way", 500, nil, nil)); len(got.Hits) != 19 || got.PageInfo.GetHasNextPage() {
		t.Errorf("first=500 returned %d hits, has_next_page %v", len(got.Hits), got.PageInfo.GetHasNextPage())
	}
	if got := search(t, r, searchReq("fixture way", -3, nil, nil)); len(got.Hits) != 1 {
		t.Errorf("first=-3 returned %d hits, want 1", len(got.Hits))
	}

	// With legal_as_of set, every cursor carries it and the walk still works.
	when := at(tu.HistoryChange.Add(-24 * time.Hour))
	if got, _ := walk(t, r, "fixture way", 4, when); len(got) != 19 {
		t.Errorf("walk at %s returned %d hits, want 19", tu.HistoryChange, len(got))
	}
}

// When legal_as_of is unset, every page resolves at the first page's time,
// so a legal change recorded between pages doesn't move a hit.
func TestSearchSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := tu.FixtureDatabase(t).Pool(t)
	r := startReaderOn(t, pool)

	page1 := search(t, r, searchReq("fixture way", 5, nil, nil))
	asOf := decodeTestCursor(t, page1.PageInfo.GetEndCursor()).GetAsOf().AsTime()
	if time.Since(asOf) > time.Minute || time.Since(asOf) < 0 {
		t.Fatalf("snapshot %s isn't now", asOf)
	}

	// Address 7 is reworded from a time after the snapshot, as a new legal
	// fact would be: its current row ends then, and a new row starts.
	time.Sleep(2 * time.Millisecond)
	_, err := pool.Exec(ctx, `
		WITH closed AS (
			UPDATE core.address_attributes aa
			SET legal_valid_range = tstzrange(lower(aa.legal_valid_range), now())
			FROM core.addresses a
			WHERE a.address_id = aa.address_id AND a.public_id = $1 AND aa.legal_valid_range @> now()
			RETURNING aa.*
		)
		INSERT INTO core.address_attributes (address_id, country_id, administrative_area_id, locality, postal_code,
		       address_line_1, formatted_address, legal_valid_range, trace_id)
		SELECT address_id, country_id, administrative_area_id, locality, postal_code,
		       '7 Renamed Road', '7 Renamed Road, Testville ZZ000', tstzrange(now(), NULL), 'test'
		FROM closed`, tu.AddressID(7))
	if err != nil {
		t.Fatal(err)
	}
	if got := hitIDs(search(t, r, searchReq("fixture way", 100, nil, nil)).Hits); slices.Contains(got, tu.ParcelID(7)) {
		t.Fatal("the rewording didn't take effect: a new search still finds parcel 7")
	}

	page2 := search(t, r, searchReq("fixture way", 5, page1.PageInfo.EndCursor, nil))
	if !slices.Contains(hitIDs(page2.Hits), tu.ParcelID(7)) {
		t.Errorf("page 2 = %v, want parcel 7 from the snapshot", hitIDs(page2.Hits))
	}
	if got := decodeTestCursor(t, page2.PageInfo.GetEndCursor()).GetAsOf().AsTime(); !got.Equal(asOf) {
		t.Errorf("page 2's snapshot = %s, want %s", got, asOf)
	}
}

func TestSearchCap(t *testing.T) {
	r := startReaderOn(t, tu.FixtureDatabase(t).Pool(t), withSearchCap(10))
	hits, sizes := walk(t, r, "fixture way", 4, nil)
	if !slices.Equal(sizes, []int{4, 4, 2}) {
		t.Errorf("page sizes = %v, want [4 4 2]", sizes)
	}
	if len(hits) != 10 {
		t.Errorf("got %d hits, want the cap of 10", len(hits))
	}
}

func TestSearchQueryLength(t *testing.T) {
	r := startReader(t)
	for _, q := range []string{"--", "a.b", "ab", " a  b ", "\x00\x00\x00", strings.Repeat("a", 201)} {
		if err := searchErr(r, searchReq(q, 0, nil, nil)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("query %q: %v, want InvalidArgument", q, err)
		}
	}
	for _, q := range []string{"abc", "a.b.c", strings.Repeat("a", 200), "12\x00fixture"} {
		if err := searchErr(r, searchReq(q, 0, nil, nil)); err != nil {
			t.Errorf("query %q: %v", q, err)
		}
	}
}
