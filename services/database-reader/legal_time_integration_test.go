//go:build integration

package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for the fixes F1-F4 in docs/address-party-read-api/spec.md: existing
// reads resolve addresses, owners and geometry at the legal time, and hide
// voided parties and addresses.

// beforeHistory is a legal time when party 8, address 5 and parcel 9 still
// have their earlier versions.
var beforeHistory = time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)

func parcelDetails(t *testing.T, r readerClients, at *time.Time, ns ...int) map[string]*parcelsv1.ParcelDetails {
	t.Helper()
	req := &parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{ParcelIds: ids(tu.ParcelPrefix, ns...)}
	if at != nil {
		req.LegalAsOf = timestamppb.New(*at)
	}
	res, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*parcelsv1.ParcelDetails{}
	for id, p := range res.Msg.Parcels {
		out[id] = p.GetParcelDetails()
	}
	return out
}

// voidAddress voids fixture address n in this test's own database copy.
func voidAddress(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE core.addresses SET is_voided = true, voided_at = now() WHERE public_id = $1`, tu.AddressID(n))
	if err != nil {
		t.Fatal(err)
	}
}

// F1: a parcel's address is resolved at the legal time and hidden when voided.
func TestParcelAddressAtLegalTime(t *testing.T) {
	r := startReader(t)
	n := tu.HistoricalAddress // parcel 5 uses address 5
	for _, tc := range []struct {
		name string
		at   *time.Time
		want string
	}{
		{"now", nil, "5 Fixture Way, Testville ZZ000"},
		{"before the change", &beforeHistory, "5 Old Fixture Way, Testville ZZ000"},
	} {
		d := parcelDetails(t, r, tc.at, n)[tu.ParcelID(n)]
		if d.GetFormattedAddress() != tc.want || d.GetAddressId() != tu.AddressID(n) {
			t.Errorf("%s: parcel %d address = %q (%s), want %q", tc.name, n, d.GetFormattedAddress(), d.GetAddressId(), tc.want)
		}
	}

	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 3)
	d := parcelDetails(t, startReaderOn(t, pool), nil, 3)[tu.ParcelID(3)]
	if d == nil {
		t.Fatal("parcel 3 missing after voiding its address")
	}
	if d.AddressId != nil || d.FormattedAddress != nil {
		t.Errorf("parcel 3 with a voided address: address_id %v, formatted_address %v, want both unset", d.AddressId, d.FormattedAddress)
	}
}

// F2: owners exclude voided parties, appear once each, and are resolved at
// the legal time.
func TestParcelOwnersAtLegalTime(t *testing.T) {
	r := startReader(t)

	d := parcelDetails(t, r, nil, tu.VoidedPartyParcel)[tu.ParcelID(tu.VoidedPartyParcel)]
	if want := []string{tu.PartyID(4)}; !reflect.DeepEqual(d.GetPartyIds(), want) {
		t.Errorf("parcel %d party IDs = %v, want %v (voided party %d hidden)", tu.VoidedPartyParcel, d.GetPartyIds(), want, tu.VoidedParty)
	}
	if d.GetPrimaryOwnerName() != "Fixture Owner 4" {
		t.Errorf("parcel %d primary owner = %q", tu.VoidedPartyParcel, d.GetPrimaryOwnerName())
	}

	for _, tc := range []struct {
		name        string
		at          *time.Time
		owner, addr string
	}{
		{"now", nil, "Fixture Owner 8", "8 Fixture Way, Testville ZZ000"},
		{"before the change", &beforeHistory, "Fixture Owner 8 Formerly", "18 Fixture Way, Testville ZZ000"},
	} {
		got := parcelDetails(t, r, tc.at, 8, 18)
		for _, n := range []int{8, 18} {
			d := got[tu.ParcelID(n)]
			if want := []string{tu.PartyID(tu.HistoricalParty)}; !reflect.DeepEqual(d.GetPartyIds(), want) {
				t.Errorf("%s: parcel %d party IDs = %v, want %v once", tc.name, n, d.GetPartyIds(), want)
			}
			if d.GetPrimaryOwnerName() != tc.owner || d.GetPrimaryOwnerAddress() != tc.addr {
				t.Errorf("%s: parcel %d primary owner = %q at %q, want %q at %q",
					tc.name, n, d.GetPrimaryOwnerName(), d.GetPrimaryOwnerAddress(), tc.owner, tc.addr)
			}
		}
	}

	// A primary owner whose mailing address is voided keeps its name.
	pool := tu.FixtureDatabase(t).Pool(t)
	voidAddress(t, pool, 6) // party 6's mailing address
	d = parcelDetails(t, startReaderOn(t, pool), nil, 6)[tu.ParcelID(6)]
	if d.GetPrimaryOwnerName() != "Fixture Owner 6" || d.PrimaryOwnerAddress != nil {
		t.Errorf("parcel 6 with a voided owner address: owner %q, address %v; want the name and no address",
			d.GetPrimaryOwnerName(), d.PrimaryOwnerAddress)
	}
}
