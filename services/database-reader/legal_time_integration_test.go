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

// F3: comparables read current attributes, addresses, geometry, zoning and
// improvements only. For parcels 5 and 9 the old rows only made the result
// depend on row order; the history added below fails deterministically.
func TestComparablesUseCurrentRows(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	sel := ids(tu.ParcelPrefix, tu.HistoricalAddress, tu.RedrawnParcel, tu.HistoricalParcel)
	eq, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		SelectedParcelIds: sel,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(eq.Msg.Parcels) != 3 {
		t.Fatalf("equity comparables = %v, want 3 parcels", sortedKeys(eq.Msg.Parcels))
	}
	if got := eq.Msg.Parcels[tu.ParcelID(tu.HistoricalAddress)].GetFormattedAddress(); got != "5 Fixture Way, Testville ZZ000" {
		t.Errorf("parcel 5 address = %q, want the current one", got)
	}
	if got := eq.Msg.Parcels[tu.ParcelID(tu.RedrawnParcel)].GetFeatureId(); got != tu.ParcelFeatureID(tu.RedrawnParcel) {
		t.Errorf("parcel 9 feature = %d, want the current %d, not %d", got, tu.ParcelFeatureID(tu.RedrawnParcel), tu.RedrawnParcelEarlierFeature)
	}

	// Improvement totals and zoning ignore rows that are no longer valid. The
	// fixtures have no such history, so add some: improvement 5 was once
	// 9999 m2, and parcel 5 was once zoned C-1 (zoning 2), both before 2020.
	pool := tu.FixtureDatabase(t).Pool(t)
	_, err = pool.Exec(ctx, `
		INSERT INTO core.improvement_attributes (improvement_id, address_id, improvement_type_id, improvement_condition_id,
		       improvement_quality_id, area_sq_m, bathrooms, bedrooms, year_built, effective_year_built, units, legal_valid_range, trace_id)
		SELECT improvement_id, address_id, improvement_type_id, improvement_condition_id, improvement_quality_id,
		       9999, bathrooms, bedrooms, year_built, effective_year_built, units,
		       tstzrange('2015-01-01 00:00:00+00', '2020-01-01 00:00:00+00'), 'test'
		FROM core.improvement_attributes ia JOIN core.improvements i USING (improvement_id)
		WHERE i.public_id = $1`, tu.FixtureID(tu.ImprovementPrefix, 5))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO core.parcel_affordances (public_id, parcel_id, zoning_id, affordance_type_id, source, legal_valid_range, trace_id)
		SELECT $1, pa.parcel_id, z.zoning_id, pa.affordance_type_id, 'test',
		       tstzrange('2015-01-01 00:00:00+00', '2020-01-01 00:00:00+00'), 'test'
		FROM core.parcel_affordances pa JOIN core.parcels p USING (parcel_id), core.zoning z
		WHERE p.public_id = $2 AND z.public_id = $3`,
		tu.FixtureID(tu.ParcelAffordancePrefix, 99), tu.ParcelID(5), tu.FixtureID(tu.ZoningPrefix, 2))
	if err != nil {
		t.Fatal(err)
	}
	hist, err := startReaderOn(t, pool).parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		SelectedParcelIds: ids(tu.ParcelPrefix, 5),
		Criteria: []*parcelsv1.ComparableCriteria{
			{Attribute: parcelsv1.ParcelAttribute_PARCEL_ATTRIBUTE_IMPROVEMENT_AREA_SQ_FT},
			{Attribute: parcelsv1.ParcelAttribute_PARCEL_ATTRIBUTE_ZONING_ID},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range hist.Msg.Parcels[tu.ParcelID(5)].GetAttributes() {
		switch a.GetAttribute() {
		case parcelsv1.ParcelAttribute_PARCEL_ATTRIBUTE_IMPROVEMENT_AREA_SQ_FT:
			if want := 145 * sqftPerSqm; !near(a.GetNumericalValue(), want) {
				t.Errorf("parcel 5 improvement area = %v sq ft, want the current %v", a.GetNumericalValue(), want)
			}
		case parcelsv1.ParcelAttribute_PARCEL_ATTRIBUTE_ZONING_ID:
			if want := tu.FixtureID(tu.ZoningPrefix, 1); a.GetCategoricalValue() != want {
				t.Errorf("parcel 5 zoning = %q, want only the current %s", a.GetCategoricalValue(), want)
			}
		}
	}

	// A comparable whose address is voided has no address.
	voidAddress(t, pool, 5)
	eq, err = startReaderOn(t, pool).parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		SelectedParcelIds: ids(tu.ParcelPrefix, 5),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if p := eq.Msg.Parcels[tu.ParcelID(5)]; p == nil || p.GetAddressId() != "" || p.GetFormattedAddress() != "" {
		t.Errorf("parcel 5 with a voided address = %v, want no address", p)
	}
}
