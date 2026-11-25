//go:build integration

package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	improvementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for the improvement read API (docs/improvement-read-api/spec.md,
// "Tests"; plan.md, 1.5).

// before is a legal time before every improvement fixture change.
var before = time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)

func TestImprovementQualities(t *testing.T) {
	r := startReader(t)
	res, err := r.improvements.GetImprovementQualities(context.Background(), connect.NewRequest(&improvementsv1.GetImprovementQualitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := res.Msg.ImprovementQualities
	want := map[string]struct {
		name     string
		modifier float64
	}{
		tu.ImprovementQualityID(1):                {"Fixture Standard", 1.0},
		tu.ImprovementQualityID(tu.UnusedQuality): {"Fixture Premium", 1.2},
	}
	if len(got) != len(want) {
		t.Fatalf("qualities = %v, want %d entries", got, len(want))
	}
	for id, w := range want {
		q := got[id]
		if q.GetId() != id || q.GetName() != w.name || !near(q.GetQualityAdjustmentModifier(), w.modifier) {
			t.Errorf("quality %s = %v, want %q with modifier %v", id, q, w.name, w.modifier)
		}
	}
}

func TestImprovementQualitiesHideVoided(t *testing.T) {
	pool := tu.FixtureDatabase(t).Pool(t)
	// This test has its own database copy, so nothing needs restoring.
	if _, err := pool.Exec(context.Background(),
		`UPDATE core.improvement_qualities SET is_voided = true, voided_at = now() WHERE public_id = $1`,
		tu.ImprovementQualityID(tu.UnusedQuality)); err != nil {
		t.Fatal(err)
	}
	res, err := startReaderOn(t, pool).improvements.GetImprovementQualities(context.Background(),
		connect.NewRequest(&improvementsv1.GetImprovementQualitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(res.Msg.ImprovementQualities); !slices.Equal(got, []string{tu.ImprovementQualityID(1)}) {
		t.Errorf("qualities = %v, want only quality 1", got)
	}
}

// TestImprovementFixtures pins the new improvement fixtures through the
// reads that exist already, so a fixture change shows up before the new
// reads depend on it.
func TestImprovementFixtures(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()

	t.Run("moving improvement at its address", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			when       *timestamppb.Timestamp
			shedParcel int
		}{
			{"now", nil, tu.MovingImprovementParcel},
			{"2022", at(before), tu.MovingImprovementEarlierParcel},
		} {
			res, err := r.improvements.GetImprovementsByAddressId(ctx, connect.NewRequest(&improvementsv1.GetImprovementsByAddressIdRequest{
				AddressId: tu.AddressID(tu.MovingImprovementParcel), LegalAsOf: tc.when,
			}))
			if err != nil {
				t.Fatal(err)
			}
			got := map[string][]string{}
			for _, imp := range res.Msg.Improvements {
				got[imp.GetImprovementId()] = imp.GetParcelIds()
			}
			want := map[string][]string{
				tu.ImprovementID(tu.MovingImprovementParcel): {tu.ParcelID(tu.MovingImprovementParcel)},
				tu.ImprovementID(tu.MovingImprovement):       {tu.ParcelID(tc.shedParcel)},
			}
			if len(got) != len(want) {
				t.Errorf("%s: improvements at address 12 = %v, want %v", tc.name, got, want)
				continue
			}
			for id, parcels := range want {
				if !slices.Equal(got[id], parcels) {
					t.Errorf("%s: improvement %s parcels = %v, want %v", tc.name, id, got[id], parcels)
				}
			}
		}
	})

	t.Run("parcel summaries", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			when      *timestamppb.Timestamp
			parcel    int
			imps      []int // improvement numbers, primary first
			areaSqM   float64
			checkArea bool
		}{
			{"parcel 12 now", nil, 12, []int{12, tu.MovingImprovement}, 0, false},
			{"parcel 11 now", nil, 11, []int{11}, 0, false},
			{"parcel 11 in 2022", at(before), 11, []int{11, tu.MovingImprovement}, 0, false},
			{"parcel 12 in 2022", at(before), 12, []int{12}, 0, false},
			{"parcel 4 now", nil, 4, []int{4}, tu.ImprovementAreaSqM(4), true},
			{"parcel 4 in 2022", at(before), 4, []int{4}, 100, true},
		} {
			res, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
				ParcelIds: []string{tu.ParcelID(tc.parcel)}, LegalAsOf: tc.when,
			}))
			if err != nil {
				t.Fatal(err)
			}
			s := res.Msg.Parcels[tu.ParcelID(tc.parcel)].GetImprovementSummary()
			var want []string
			for _, n := range tc.imps {
				want = append(want, tu.ImprovementID(n))
			}
			got := slices.Clone(s.GetImprovementIds())
			slices.Sort(got)
			wantSorted := slices.Clone(want)
			slices.Sort(wantSorted)
			if !slices.Equal(got, wantSorted) || s.GetPrimaryImprovementId() != want[0] {
				t.Errorf("%s: improvements %v, primary %s; want %v, primary %s",
					tc.name, got, s.GetPrimaryImprovementId(), wantSorted, want[0])
			}
			if tc.checkArea && !near(s.GetTotalAreaSqFt(), tc.areaSqM*sqftPerSqm) {
				t.Errorf("%s: total area %v sq ft, want %v m2", tc.name, s.GetTotalAreaSqFt(), tc.areaSqM)
			}
		}
	})
}
