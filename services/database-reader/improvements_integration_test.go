//go:build integration

package main

import (
	"context"
	"fmt"
	"math/big"
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
// "Tests"; plan.md, 1.5 and 2.3).

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

func improvementsByID(t *testing.T, r readerClients, when *timestamppb.Timestamp, valuation *string, ids ...string) map[string]*improvementsv1.Improvement {
	t.Helper()
	res, err := r.improvements.GetImprovementsById(context.Background(), connect.NewRequest(&improvementsv1.GetImprovementsByIdRequest{
		ImprovementIds: ids, LegalAsOf: when, ValuationId: valuation,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Improvements
}

func oneImprovement(t *testing.T, r readerClients, n int, when *timestamppb.Timestamp) *improvementsv1.Improvement {
	t.Helper()
	m := improvementsByID(t, r, when, nil, tu.ImprovementID(n))[tu.ImprovementID(n)]
	if m == nil {
		t.Fatalf("improvement %d missing", n)
	}
	return m
}

func shares(m *improvementsv1.Improvement) []string {
	var out []string
	for _, s := range m.GetParties() {
		out = append(out, s.GetPartyId()+"="+s.GetOwnershipShare())
	}
	return out
}

func share(party int, value string) string { return tu.PartyID(party) + "=" + value }

func links(m *improvementsv1.Improvement) []string {
	var out []string
	for _, l := range m.GetParcels() {
		primary := "secondary"
		if l.GetIsPrimary() {
			primary = "primary"
		}
		out = append(out, fmt.Sprintf("%s/%d/%s", l.GetParcelId(), l.GetFeatureId(), primary))
	}
	return out
}

func link(parcel int, primary bool) string {
	p := "secondary"
	if primary {
		p = "primary"
	}
	return fmt.Sprintf("%s/%d/%s", tu.ParcelID(parcel), tu.ParcelFeatureID(parcel), p)
}

func TestImprovementsByID(t *testing.T) {
	r := startReader(t)

	t.Run("improvement 1 has its fields", func(t *testing.T) {
		m := oneImprovement(t, r, 1, nil)
		if m.GetTypeId() != tu.FixtureID(tu.ImprovementTypePrefix, 1) ||
			m.GetConditionId() != tu.FixtureID(tu.ImprovementConditionPrefix, 1) ||
			m.GetQualityId() != tu.ImprovementQualityID(1) ||
			m.GetAddressId() != tu.AddressID(1) || m.GetFormattedAddress() != "1 Fixture Way, Testville ZZ000" ||
			!near(m.GetAreaSqFt(), tu.ImprovementAreaSqM(1)*sqftPerSqm) ||
			m.GetFullBathrooms() != 2 || m.GetHalfBathrooms() != 1 || m.GetBathrooms() != 2.5 ||
			m.GetBedrooms() != 3 || m.GetUnits() != 1 || m.GetYearBuilt() != 1981 || m.GetEffectiveYearBuilt() != 1991 {
			t.Errorf("improvement 1 = %v", m)
		}
		if m.Properties != nil || m.MarketValue != nil || m.AssessedValue != nil {
			t.Errorf("improvement 1: properties or values set without data or a valuation: %v", m)
		}
		if got, want := links(m), []string{link(1, true)}; !slices.Equal(got, want) {
			t.Errorf("improvement 1 parcels = %v, want %v", got, want)
		}
		if len(m.GetParties()) != 0 {
			t.Errorf("improvement 1 parties = %v, want none recorded", shares(m))
		}
	})

	t.Run("secondary improvement with its own owner", func(t *testing.T) {
		m := oneImprovement(t, r, tu.SecondaryImprovement, nil)
		if got, want := links(m), []string{link(tu.SecondaryImpParcel, false)}; !slices.Equal(got, want) {
			t.Errorf("improvement 16 parcels = %v, want %v", got, want)
		}
		// Voided party 11's share is dropped; parcel 3's owner is party 3.
		if got, want := shares(m), []string{share(5, "0.5000")}; !slices.Equal(got, want) {
			t.Errorf("improvement 16 parties = %v, want %v", got, want)
		}
	})

	t.Run("shares", func(t *testing.T) {
		if got, want := shares(oneImprovement(t, r, 2, nil)), []string{share(2, "0.5000"), share(3, "0.5000")}; !slices.Equal(got, want) {
			t.Errorf("improvement 2 parties = %v, want %v", got, want)
		}
		if got, want := shares(oneImprovement(t, r, tu.OwnerChangeImprovement, nil)), []string{share(6, "1.0000")}; !slices.Equal(got, want) {
			t.Errorf("improvement 6 parties now = %v, want %v", got, want)
		}
		if got, want := shares(oneImprovement(t, r, tu.OwnerChangeImprovement, at(before))), []string{share(10, "1.0000")}; !slices.Equal(got, want) {
			t.Errorf("improvement 6 parties in 2022 = %v, want %v", got, want)
		}
	})

	t.Run("attribute history", func(t *testing.T) {
		old := oneImprovement(t, r, tu.HistoricalImprovement, at(before))
		if old.GetConditionId() != tu.FixtureID(tu.ImprovementConditionPrefix, 3) ||
			!near(old.GetAreaSqFt(), 100*sqftPerSqm) || old.GetFullBathrooms() != 1 || old.HalfBathrooms != nil ||
			old.GetBathrooms() != 1 || old.GetBedrooms() != 2 || old.GetEffectiveYearBuilt() != 1984 {
			t.Errorf("improvement 4 in 2022 = %v", old)
		}
		now := oneImprovement(t, r, tu.HistoricalImprovement, nil)
		if now.GetConditionId() != tu.FixtureID(tu.ImprovementConditionPrefix, 1) ||
			!near(now.GetAreaSqFt(), tu.ImprovementAreaSqM(4)*sqftPerSqm) || now.GetFullBathrooms() != 2 ||
			now.GetBedrooms() != 3 || now.GetEffectiveYearBuilt() != 1994 {
			t.Errorf("improvement 4 now = %v", now)
		}
	})

	t.Run("moving improvement and unset fields", func(t *testing.T) {
		now := oneImprovement(t, r, tu.MovingImprovement, nil)
		if got, want := links(now), []string{link(tu.MovingImprovementParcel, false)}; !slices.Equal(got, want) {
			t.Errorf("improvement 18 parcels now = %v, want %v", got, want)
		}
		if got, want := links(oneImprovement(t, r, tu.MovingImprovement, at(before))), []string{link(tu.MovingImprovementEarlierParcel, false)}; !slices.Equal(got, want) {
			t.Errorf("improvement 18 parcels in 2022 = %v, want %v", got, want)
		}
		if now.ConditionId != nil || now.QualityId != nil || now.FullBathrooms != nil || now.HalfBathrooms != nil ||
			now.Bathrooms != nil || now.Bedrooms != nil || now.Units != nil || now.EffectiveYearBuilt != nil {
			t.Errorf("improvement 18 has fields set that the fixture leaves null: %v", now)
		}
		if now.GetYearBuilt() != 2001 || !near(now.GetAreaSqFt(), tu.ImprovementAreaSqM(tu.MovingImprovement)*sqftPerSqm) {
			t.Errorf("improvement 18 = %v", now)
		}
	})

	t.Run("hidden and unknown IDs", func(t *testing.T) {
		got := improvementsByID(t, r, nil, nil, tu.ImprovementID(1), tu.ImprovementID(17), tu.MissingID)
		if keys := sortedKeys(got); !slices.Equal(keys, []string{tu.ImprovementID(1)}) {
			t.Errorf("improvements = %v, want only improvement 1", keys)
		}
	})

	t.Run("valuation", func(t *testing.T) {
		val := tu.FixtureID(tu.ValuationPrefix, 1)
		got := improvementsByID(t, r, nil, &val, tu.ImprovementID(1), tu.ImprovementID(tu.MovingImprovement))
		if m := got[tu.ImprovementID(1)]; m.GetMarketValue() != "82000.0000" || m.GetAssessedValue() != "73800.0000" {
			t.Errorf("improvement 1 values = %q, %q", m.GetMarketValue(), m.GetAssessedValue())
		}
		if m := got[tu.ImprovementID(tu.MovingImprovement)]; m == nil || m.MarketValue != nil || m.AssessedValue != nil {
			t.Errorf("improvement 18 with a valuation = %v, want no values", m)
		}
		unknown := tu.MissingID
		if m := improvementsByID(t, r, nil, &unknown, tu.ImprovementID(1))[tu.ImprovementID(1)]; m == nil || m.MarketValue != nil {
			t.Errorf("improvement 1 with an unknown valuation = %v, want no values", m)
		}
	})
}

// TestImprovementsByIDVisibility voids records in its own database copy.
func TestImprovementsByIDVisibility(t *testing.T) {
	pool := tu.FixtureDatabase(t).Pool(t)
	ctx := context.Background()
	for _, q := range []struct{ sql, id string }{
		{`UPDATE core.improvements SET is_voided = true, voided_at = now() WHERE public_id = $1`, tu.ImprovementID(2)},
		{`UPDATE core.improvement_conditions SET is_voided = true, voided_at = now() WHERE public_id = $1`, tu.FixtureID(tu.ImprovementConditionPrefix, 3)},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.id); err != nil {
			t.Fatal(err)
		}
	}
	voidAddress(t, pool, 3)
	r := startReaderOn(t, pool)

	if got := improvementsByID(t, r, nil, nil, tu.ImprovementID(2)); len(got) != 0 {
		t.Errorf("voided improvement 2 = %v, want absent", got)
	}
	if m := oneImprovement(t, r, 3, nil); m.AddressId != nil || m.FormattedAddress != nil {
		t.Errorf("improvement 3 at voided address 3 = %v, want both address fields unset", m)
	}
	// Condition IDs are set only when GetImprovementConditions returns them now.
	if m := oneImprovement(t, r, tu.HistoricalImprovement, at(before)); m.ConditionId != nil {
		t.Errorf("improvement 4 in 2022 with voided condition 3 = %v, want condition_id unset", m)
	}
}

// TestImprovementsAgreeWithSummary: summing GetImprovementsById over a
// parcel summary's improvement_ids gives the summary's totals, and the
// improvement whose link to *that* parcel is primary is the summary's
// primary improvement. Written per parcel, so it holds when an improvement
// sits on several parcels.
func TestImprovementsAgreeWithSummary(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	val := tu.FixtureID(tu.ValuationPrefix, 1)

	type check struct {
		parcel int
		when   *timestamppb.Timestamp
	}
	var checks []check
	for n := 1; n <= tu.ImprovedParcels; n++ {
		checks = append(checks, check{n, nil})
	}
	// Parcel 4 holds historical improvement 4; parcels 11 and 12 the moving one.
	for _, n := range []int{4, tu.MovingImprovementEarlierParcel, tu.MovingImprovementParcel} {
		checks = append(checks, check{n, at(before)})
	}

	for _, c := range checks {
		res, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(c.parcel)}, LegalAsOf: c.when, ValuationId: &val,
		}))
		if err != nil {
			t.Fatal(err)
		}
		s := res.Msg.Parcels[tu.ParcelID(c.parcel)].GetImprovementSummary()
		imps := improvementsByID(t, r, c.when, &val, s.GetImprovementIds()...)
		if len(imps) != len(s.GetImprovementIds()) {
			t.Errorf("parcel %d (%v): summary lists %d improvements, read returned %d", c.parcel, c.when, len(s.GetImprovementIds()), len(imps))
			continue
		}

		var area, baths float64
		var full, half, beds, units int32
		market, assessed := new(big.Rat), new(big.Rat)
		var primaries []string
		for _, m := range imps {
			area += m.GetAreaSqFt()
			baths += m.GetBathrooms()
			full += m.GetFullBathrooms()
			half += m.GetHalfBathrooms()
			beds += m.GetBedrooms()
			units += m.GetUnits()
			addDecimal(t, market, m.MarketValue)
			addDecimal(t, assessed, m.AssessedValue)
			for _, l := range m.GetParcels() {
				if l.GetParcelId() == tu.ParcelID(c.parcel) && l.GetIsPrimary() {
					primaries = append(primaries, m.GetImprovementId())
				}
			}
		}
		if !near(area, s.GetTotalAreaSqFt()) || baths != s.GetTotalBathrooms() || full != s.GetTotalFullBathrooms() ||
			half != s.GetTotalHalfBathrooms() || beds != s.GetTotalBedrooms() || units != s.GetTotalUnits() {
			t.Errorf("parcel %d (%v): sums area %v, baths %v (%d full, %d half), beds %d, units %d; summary %v",
				c.parcel, c.when, area, baths, full, half, beds, units, s)
		}
		if !slices.Equal(primaries, []string{s.GetPrimaryImprovementId()}) {
			t.Errorf("parcel %d (%v): primary links %v, summary primary %s", c.parcel, c.when, primaries, s.GetPrimaryImprovementId())
		}
		wantMarket, wantAssessed := new(big.Rat), new(big.Rat)
		addDecimal(t, wantMarket, s.TotalMarketImprovementValue)
		addDecimal(t, wantAssessed, s.TotalAssessedImprovementValue)
		if market.Cmp(wantMarket) != 0 || assessed.Cmp(wantAssessed) != 0 {
			t.Errorf("parcel %d (%v): values sum to %s / %s, summary %s / %s", c.parcel, c.when,
				market.FloatString(4), assessed.FloatString(4), wantMarket.FloatString(4), wantAssessed.FloatString(4))
		}
	}
}

// addDecimal adds a decimal string to sum; an unset value adds nothing.
func addDecimal(t *testing.T, sum *big.Rat, v *string) {
	t.Helper()
	if v == nil {
		return
	}
	x, ok := new(big.Rat).SetString(*v)
	if !ok {
		t.Fatalf("not a decimal: %q", *v)
	}
	sum.Add(sum, x)
}
