//go:build integration

package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	addressesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/addresses/v1"
	improvementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	partiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for the address and party read API
// (docs/address-party-read-api/spec.md, "Tests").

func at(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func addressesByID(t *testing.T, r readerClients, when *timestamppb.Timestamp, ids ...string) map[string]*addressesv1.Address {
	t.Helper()
	res, err := r.addresses.GetAddressesById(context.Background(), connect.NewRequest(&addressesv1.GetAddressesByIdRequest{
		AddressIds: ids, LegalAsOf: when,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Addresses
}

func partiesByID(t *testing.T, r readerClients, when *timestamppb.Timestamp, ids ...string) map[string]*partiesv1.Party {
	t.Helper()
	res, err := r.parties.GetPartiesById(context.Background(), connect.NewRequest(&partiesv1.GetPartiesByIdRequest{
		PartyIds: ids, LegalAsOf: when,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Parties
}

func parcelsAtAddress(t *testing.T, r readerClients, n int, when *timestamppb.Timestamp) []*parcelsv1.AddressParcel {
	t.Helper()
	res, err := r.parcels.GetParcelsByAddressId(context.Background(), connect.NewRequest(&parcelsv1.GetParcelsByAddressIdRequest{
		AddressId: tu.AddressID(n), LegalAsOf: when,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Parcels
}

func improvementsAtAddress(t *testing.T, r readerClients, n int) []*improvementsv1.AddressImprovement {
	t.Helper()
	res, err := r.improvements.GetImprovementsByAddressId(context.Background(), connect.NewRequest(&improvementsv1.GetImprovementsByAddressIdRequest{
		AddressId: tu.AddressID(n),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Improvements
}

func partiesAtAddress(t *testing.T, r readerClients, n int, when *timestamppb.Timestamp) []string {
	t.Helper()
	res, err := r.parties.GetPartiesByAddressId(context.Background(), connect.NewRequest(&partiesv1.GetPartiesByAddressIdRequest{
		AddressId: tu.AddressID(n), LegalAsOf: when,
	}))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range res.Msg.Parties {
		out = append(out, p.GetPartyId())
	}
	return out
}

func partyParcels(t *testing.T, r readerClients, n int) []*parcelsv1.PartyParcel {
	t.Helper()
	res, err := r.parcels.GetParcelsByPartyId(context.Background(), connect.NewRequest(&parcelsv1.GetParcelsByPartyIdRequest{
		PartyId: tu.PartyID(n),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Parcels
}

func TestGetAddressesById(t *testing.T) {
	r := startReader(t)

	a := addressesByID(t, r, nil, tu.AddressID(3))[tu.AddressID(3)]
	if a == nil {
		t.Fatal("address 3 missing")
	}
	if a.GetFormattedAddress() != "3 Fixture Way, Testville ZZ000" || a.GetAddressLine_1() != "3 Fixture Way" ||
		a.AddressLine_2 != nil || a.GetLocality() != "Testville" || a.Sublocality != nil || a.GetPostalCode() != "ZZ000" {
		t.Errorf("address 3 = %v", a)
	}
	if a.GetCountry().GetCode() != "ZZ" || a.GetCountry().GetName() != "Testland" ||
		a.GetAdministrativeArea().GetCode() != "ZZ-FX" || a.GetAdministrativeArea().GetName() != "Fixture Province" {
		t.Errorf("address 3 country %v, area %v", a.GetCountry(), a.GetAdministrativeArea())
	}
	// The centre of parcel 3's square (column 2, row 0 of the grid).
	if c := a.GetCoordinates(); c == nil || !near(c.GetLongitude(), -139.999) || !near(c.GetLatitude(), -39.9998) {
		t.Errorf("address 3 coordinates = %v", c)
	}

	h := tu.AddressID(tu.HistoricalAddress)
	if got := addressesByID(t, r, nil, h)[h]; got.GetAddressLine_1() != "5 Fixture Way" || got.GetCoordinates() == nil {
		t.Errorf("address 5 now = %v", got)
	}
	if got := addressesByID(t, r, at(beforeHistory), h)[h]; got.GetAddressLine_1() != "5 Old Fixture Way" ||
		got.GetFormattedAddress() != "5 Old Fixture Way, Testville ZZ000" || got.Coordinates != nil {
		t.Errorf("address 5 at 2022 = %v, want the old line and no coordinates", got)
	}

	if got := addressesByID(t, r, nil, tu.MissingID); len(got) != 0 {
		t.Errorf("unknown ID returned %v", got)
	}
	if got := addressesByID(t, r, at(time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)), tu.AddressID(3)); len(got) != 0 {
		t.Errorf("before any attributes: got %v", got)
	}

	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 3)
	if got := addressesByID(t, startReaderOn(t, pool), nil, tu.AddressID(3), tu.AddressID(4)); len(got) != 1 || got[tu.AddressID(4)] == nil {
		t.Errorf("with address 3 voided: got %v, want only address 4", sortedKeys(got))
	}
}

func TestAddressReverseLookups(t *testing.T) {
	r := startReader(t)

	p := parcelsAtAddress(t, r, 3, nil)
	if len(p) != 1 || p[0].GetParcelId() != tu.ParcelID(3) || p[0].GetFeatureId() != tu.ParcelFeatureID(3) {
		t.Errorf("parcels at address 3 = %v", p)
	}
	// Parcel 21 also uses address 2, but has no geometry (rule 3).
	if p := parcelsAtAddress(t, r, 2, nil); len(p) != 1 || p[0].GetParcelId() != tu.ParcelID(2) {
		t.Errorf("parcels at address 2 = %v, want parcel 2 only", p)
	}

	imps := improvementsAtAddress(t, r, 3)
	want := []string{tu.FixtureID(tu.ImprovementPrefix, 3), tu.FixtureID(tu.ImprovementPrefix, 16)}
	var got []string
	for _, i := range imps {
		got = append(got, i.GetImprovementId())
		if !reflect.DeepEqual(i.GetParcelIds(), []string{tu.ParcelID(3)}) {
			t.Errorf("improvement %s parcels = %v, want parcel 3", i.GetImprovementId(), i.GetParcelIds())
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("improvements at address 3 = %v, want %v", got, want)
	}
	// Improvement 17 is only on voided parcel 20 (rule 4).
	if imps := improvementsAtAddress(t, r, 20); len(imps) != 0 {
		t.Errorf("improvements at address 20 = %v, want none", imps)
	}

	if got := partiesAtAddress(t, r, 3, nil); !reflect.DeepEqual(got, []string{tu.PartyID(3)}) {
		t.Errorf("parties at address 3 = %v", got)
	}
	if got := partiesAtAddress(t, r, 11, nil); len(got) != 0 {
		t.Errorf("parties at address 11 = %v, want none (party 11 is voided)", got)
	}
	e := tu.HistoricalPartyEarlierAddress
	if got := partiesAtAddress(t, r, e, at(beforeHistory)); !reflect.DeepEqual(got, []string{tu.PartyID(tu.HistoricalParty)}) {
		t.Errorf("parties at address %d in 2022 = %v, want party 8", e, got)
	}
	if got := partiesAtAddress(t, r, e, nil); len(got) != 0 {
		t.Errorf("parties at address %d now = %v, want none", e, got)
	}

	// A voided address has no references.
	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 3)
	rv := startReaderOn(t, pool)
	if p := parcelsAtAddress(t, rv, 3, nil); len(p) != 0 {
		t.Errorf("parcels at voided address 3 = %v", p)
	}
	if got := partiesAtAddress(t, rv, 3, nil); len(got) != 0 {
		t.Errorf("parties at voided address 3 = %v", got)
	}
}

func TestGetPartiesById(t *testing.T) {
	r := startReader(t)

	p := partiesByID(t, r, nil, tu.PartyID(3))[tu.PartyID(3)]
	if p.GetName() != "Fixture Owner 3" || p.GetMailingAddressId() != tu.AddressID(3) ||
		p.GetMailingAddress() != "3 Fixture Way, Testville ZZ000" {
		t.Errorf("party 3 = %v", p)
	}

	h := tu.PartyID(tu.HistoricalParty)
	if p := partiesByID(t, r, nil, h)[h]; p.GetName() != "Fixture Owner 8" || p.GetMailingAddressId() != tu.AddressID(8) {
		t.Errorf("party 8 now = %v", p)
	}
	if p := partiesByID(t, r, at(beforeHistory), h)[h]; p.GetName() != "Fixture Owner 8 Formerly" ||
		p.GetMailingAddressId() != tu.AddressID(tu.HistoricalPartyEarlierAddress) ||
		p.GetMailingAddress() != "18 Fixture Way, Testville ZZ000" {
		t.Errorf("party 8 at 2022 = %v", p)
	}

	if got := partiesByID(t, r, nil, tu.PartyID(tu.VoidedParty), tu.MissingID); len(got) != 0 {
		t.Errorf("voided and unknown parties returned %v", sortedKeys(got))
	}

	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 3)
	p = partiesByID(t, startReaderOn(t, pool), nil, tu.PartyID(3))[tu.PartyID(3)]
	if p.GetName() != "Fixture Owner 3" || p.MailingAddressId != nil || p.MailingAddress != nil {
		t.Errorf("party 3 with a voided mailing address = %v, want the name only", p)
	}
}

func TestGetParcelsByPartyId(t *testing.T) {
	r := startReader(t)
	type holding struct {
		parcel int
		share  string
	}
	check := func(party int, want []holding) {
		t.Helper()
		got := partyParcels(t, r, party)
		var g []holding
		for _, pp := range got {
			n := 0
			for i := 1; i <= tu.ParcelCount; i++ {
				if tu.ParcelID(i) == pp.GetParcelId() {
					n = i
				}
			}
			if pp.GetFeatureId() != tu.ParcelFeatureID(n) {
				t.Errorf("party %d parcel %d feature = %d", party, n, pp.GetFeatureId())
			}
			g = append(g, holding{n, pp.GetOwnershipShare()})
		}
		if !reflect.DeepEqual(g, want) {
			t.Errorf("party %d parcels = %v, want %v", party, g, want)
		}
	}
	check(3, []holding{{2, "0.5000"}, {3, "1.0000"}, {13, "1.0000"}})
	check(2, []holding{{2, "0.5000"}, {12, "1.0000"}}) // not parcel 21 (no geometry)
	check(10, []holding{{10, "1.0000"}})               // not voided parcel 20
	check(tu.VoidedParty, nil)
}

func TestAddressPartyPagination(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	page := func(req *parcelsv1.GetParcelsByPartyIdRequest) (*parcelsv1.GetParcelsByPartyIdResponse, error) {
		res, err := r.parcels.GetParcelsByPartyId(ctx, connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	one := int32(1)

	var walked []string
	var after *string
	for i, want := range []struct{ next, prev bool }{{true, false}, {true, true}, {false, true}} {
		res, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), First: &one, After: after})
		if err != nil {
			t.Fatalf("page %d: %v", i+1, err)
		}
		pi := res.GetPageInfo()
		if len(res.GetParcels()) != 1 || pi.GetHasNextPage() != want.next || pi.GetHasPreviousPage() != want.prev ||
			pi.GetStartCursor() == "" || pi.GetEndCursor() != pi.GetStartCursor() {
			t.Fatalf("page %d = %v", i+1, res)
		}
		walked = append(walked, res.GetParcels()[0].GetParcelId())
		after = pi.EndCursor
	}
	if want := ids(tu.ParcelPrefix, 2, 3, 13); !reflect.DeepEqual(walked, want) {
		t.Errorf("walked %v, want %v", walked, want)
	}
	past, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), After: after})
	if err != nil {
		t.Fatal(err)
	}
	if pi := past.GetPageInfo(); len(past.GetParcels()) != 0 || pi.GetHasNextPage() || !pi.GetHasPreviousPage() ||
		pi.StartCursor != nil || pi.EndCursor != nil {
		t.Errorf("past the end = %v", past)
	}

	first, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), First: &one})
	if err != nil {
		t.Fatal(err)
	}
	cursor := first.GetPageInfo().EndCursor
	for name, call := range map[string]func() error{
		"another party": func() error {
			_, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(2), After: cursor})
			return err
		},
		"another legal_as_of": func() error {
			_, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), LegalAsOf: at(beforeHistory), After: cursor})
			return err
		},
		"another RPC": func() error {
			_, err := r.parcels.GetParcelsByAddressId(ctx, connect.NewRequest(&parcelsv1.GetParcelsByAddressIdRequest{
				AddressId: tu.PartyID(3), After: cursor,
			}))
			return err
		},
		"malformed cursor": func() error {
			bad := "%%%"
			_, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), After: &bad})
			return err
		},
		"non-UUID ID": func() error {
			_, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: "not-a-uuid"})
			return err
		},
	} {
		if err := call(); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}

	// The mesh has no validation, so the reader clamps first itself.
	big := int32(500)
	res, err := page(&parcelsv1.GetParcelsByPartyIdRequest{PartyId: tu.PartyID(3), First: &big})
	if err != nil || len(res.GetParcels()) != 3 || res.GetPageInfo().GetHasNextPage() {
		t.Errorf("first = 500: %v, %v", res, err)
	}
}

// walkPages reads every page of a lookup with first = 1, checking each page's
// PageInfo, and returns the IDs in the order they came. read returns one
// page's IDs and PageInfo for the given after.
func walkPages(t *testing.T, name string, read func(first *int32, after *string) ([]string, *paginationv1.PageInfo)) []string {
	t.Helper()
	one := int32(1)
	var walked []string
	var after *string
	for i := 0; ; i++ {
		got, pi := read(&one, after)
		if len(got) != 1 || pi.GetHasPreviousPage() != (i > 0) ||
			pi.GetStartCursor() == "" || pi.GetEndCursor() != pi.GetStartCursor() {
			t.Fatalf("%s page %d = %v, %v", name, i+1, got, pi)
		}
		walked = append(walked, got...)
		if !pi.GetHasNextPage() {
			break
		}
		if i > 10 {
			t.Fatalf("%s: more pages than results", name)
		}
		after = pi.EndCursor
	}
	big := int32(100)
	if all, pi := read(&big, nil); !reflect.DeepEqual(walked, all) || pi.GetHasNextPage() {
		t.Errorf("%s: walked %v, one page of 100 gave %v", name, walked, all)
	}
	return walked
}

// TestAddressLookupPagination walks every page of the three address lookups
// (docs/pagination-without-edges/plan.md, step 1.2). Parcel 4 and party 4 are
// moved to address 3, so each lookup has two results.
func TestAddressLookupPagination(t *testing.T) {
	ctx := context.Background()
	pool := tu.FixtureDatabase(t).Pool(t)
	moveParcelToAddress(t, pool, 4, 3)
	movePartyToAddress(t, pool, 4, 3)
	r := startReaderOn(t, pool)
	addr := tu.AddressID(3)

	parcels := walkPages(t, "parcels", func(first *int32, after *string) ([]string, *paginationv1.PageInfo) {
		res, err := r.parcels.GetParcelsByAddressId(ctx, connect.NewRequest(&parcelsv1.GetParcelsByAddressIdRequest{
			AddressId: addr, First: first, After: after,
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, p := range res.Msg.Parcels {
			out = append(out, p.GetParcelId())
		}
		return out, res.Msg.PageInfo
	})
	if want := ids(tu.ParcelPrefix, 3, 4); !reflect.DeepEqual(parcels, want) {
		t.Errorf("parcels at address 3 = %v, want %v", parcels, want)
	}

	imps := walkPages(t, "improvements", func(first *int32, after *string) ([]string, *paginationv1.PageInfo) {
		res, err := r.improvements.GetImprovementsByAddressId(ctx, connect.NewRequest(&improvementsv1.GetImprovementsByAddressIdRequest{
			AddressId: addr, First: first, After: after,
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, i := range res.Msg.Improvements {
			out = append(out, i.GetImprovementId())
		}
		return out, res.Msg.PageInfo
	})
	if want := ids(tu.ImprovementPrefix, 3, 16); !reflect.DeepEqual(imps, want) {
		t.Errorf("improvements at address 3 = %v, want %v", imps, want)
	}

	parties := walkPages(t, "parties", func(first *int32, after *string) ([]string, *paginationv1.PageInfo) {
		res, err := r.parties.GetPartiesByAddressId(ctx, connect.NewRequest(&partiesv1.GetPartiesByAddressIdRequest{
			AddressId: addr, First: first, After: after,
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, p := range res.Msg.Parties {
			out = append(out, p.GetPartyId())
		}
		return out, res.Msg.PageInfo
	})
	if want := ids(tu.PartyPrefix, 3, 4); !reflect.DeepEqual(parties, want) {
		t.Errorf("parties at address 3 = %v, want %v", parties, want)
	}
}
