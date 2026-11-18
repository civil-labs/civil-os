//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for docs/neighborhoods-api-fixes/spec.md.

// beforeNeighborhoodChange is a legal time when parcel 10 is still South
// under definition 1, and parcel 6 is still assigned under definition 2.
var beforeNeighborhoodChange = time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)

// parcelNeighborhood reads fixture parcel n under definition def (0 for none)
// at the legal time at (nil for now), by parcel ID and by feature ID, and
// returns the neighborhood ID each read gives.
func parcelNeighborhood(t *testing.T, r readerClients, n, def int, at *time.Time) (byID, byFeature string) {
	t.Helper()
	ctx := context.Background()
	var defID *string
	if def != 0 {
		id := tu.NeighborhoodDefinitionID(def)
		defID = &id
	}
	var legalAsOf *timestamppb.Timestamp
	if at != nil {
		legalAsOf = timestamppb.New(*at)
	}

	idRes, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
		ParcelIds: []string{tu.ParcelID(n)}, NeighborhoodDefinitionId: defID, LegalAsOf: legalAsOf,
	}))
	if err != nil {
		t.Fatal(err)
	}
	featureRes, err := r.parcels.GetParcelsWithImprovementSummaryByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByFeatureIdRequest{
		FeatureIds: []int64{tu.ParcelFeatureID(n)}, NeighborhoodDefinitionId: defID, LegalAsOf: legalAsOf,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(idRes.Msg.Parcels) != 1 || len(featureRes.Msg.Parcels) != 1 {
		t.Fatalf("parcel %d: got %d parcels by ID and %d by feature, want 1 each", n, len(idRes.Msg.Parcels), len(featureRes.Msg.Parcels))
	}
	return idRes.Msg.Parcels[tu.ParcelID(n)].GetParcelDetails().GetNeighborhoodId(),
		featureRes.Msg.Parcels[tu.ParcelID(n)].GetParcelDetails().GetNeighborhoodId()
}

func TestParcelNeighborhood(t *testing.T) {
	r := startReader(t)
	legal, nonLegal := tu.LegalNeighborhoodDefinition, tu.NonLegalNeighborhoodDefinition
	before := beforeNeighborhoodChange

	for _, tc := range []struct {
		name     string
		parcel   int
		def      int
		at       *time.Time
		wantNbhd int // 0 for unset
	}{
		{"parcel 3 under the legal definition", 3, legal, nil, tu.NorthNeighborhood},
		{"parcel 3 under the non-legal definition (null range)", 3, nonLegal, nil, tu.MarketAreaNeighborhood},
		{"parcel 3 with no definition", 3, 0, nil, 0},
		{"parcel 7 shares North under the non-legal definition", tu.SharedNeighborhoodParcel, nonLegal, nil, tu.NorthNeighborhood},
		{"parcel 6's dated non-legal row has ended", tu.DatedNonLegalParcel, nonLegal, nil, 0},
		{"parcel 6's dated non-legal row in 2022", tu.DatedNonLegalParcel, nonLegal, &before, tu.MarketAreaNeighborhood},
		{"parcel 10 under the legal definition now", tu.NeighborhoodHistoryParcel, legal, nil, tu.NorthNeighborhood},
		{"parcel 10 under the legal definition in 2022", tu.NeighborhoodHistoryParcel, legal, &before, tu.SouthNeighborhood},
		{"parcel 12 has no non-legal assignment", 12, nonLegal, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := ""
			if tc.wantNbhd != 0 {
				want = tu.NeighborhoodID(tc.wantNbhd)
			}
			byID, byFeature := parcelNeighborhood(t, r, tc.parcel, tc.def, tc.at)
			if byID != want || byFeature != want {
				t.Errorf("neighborhood = %q by parcel ID, %q by feature ID, want %q", byID, byFeature, want)
			}
		})
	}
}
