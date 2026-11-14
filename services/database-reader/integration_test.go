//go:build integration

package main

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	improvementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1/improvementsv1connect"
	landusesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/landuses/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/landuses/v1/landusesv1connect"
	neighborhoodsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1/neighborhoodsv1connect"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1/parcelsv1connect"
	valuationsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/valuations/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/valuations/v1/valuationsv1connect"
	zoningv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/zoning/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/zoning/v1/zoningv1connect"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

const (
	sqftPerSqm = 10.7639
	ftPerM     = 3.28084
)

type readerClients struct {
	parcels       parcelsv1connect.ParcelReadServiceClient
	improvements  improvementsv1connect.ImprovementReadServiceClient
	landUses      landusesv1connect.LandUseReadServiceClient
	zoning        zoningv1connect.ZoningReadServiceClient
	valuations    valuationsv1connect.ValuationReadServiceClient
	neighborhoods neighborhoodsv1connect.NeighborhoodReadServiceClient
}

func startReader(t *testing.T) readerClients {
	t.Helper()
	pool := tu.FixtureDatabase(t).Pool(t)
	srv := httptest.NewServer(newHandler(pool, slog.New(slog.DiscardHandler)))
	t.Cleanup(srv.Close)
	c := http.DefaultClient
	return readerClients{
		parcels:       parcelsv1connect.NewParcelReadServiceClient(c, srv.URL),
		improvements:  improvementsv1connect.NewImprovementReadServiceClient(c, srv.URL),
		landUses:      landusesv1connect.NewLandUseReadServiceClient(c, srv.URL),
		zoning:        zoningv1connect.NewZoningReadServiceClient(c, srv.URL),
		valuations:    valuationsv1connect.NewValuationReadServiceClient(c, srv.URL),
		neighborhoods: neighborhoodsv1connect.NewNeighborhoodReadServiceClient(c, srv.URL),
	}
}

// near compares with a relative tolerance; several mesh fields are float32.
func near[F float32 | float64](a F, b float64) bool {
	return math.Abs(float64(a)-b) <= 1e-5*math.Max(1, math.Abs(b))
}

func ids(prefix string, ns ...int) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = tu.FixtureID(prefix, n)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestReferenceData(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()

	lu, err := r.landUses.GetLandUses(ctx, connect.NewRequest(&landusesv1.GetLandUsesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := lu.Msg.LandUses[tu.FixtureID(tu.LandUsePrefix, 2)]; got.GetCode() != "C200" || got.GetLandUseTypeId() != tu.FixtureID(tu.LandUseTypePrefix, 2) || got.GetLandUseTypeName() != "Commercial" {
		t.Errorf("land use 2 = %v", got)
	}
	if len(lu.Msg.LandUses) != 3 {
		t.Errorf("got %d land uses, want 3", len(lu.Msg.LandUses))
	}

	lut, err := r.landUses.GetLandUseTypes(ctx, connect.NewRequest(&landusesv1.GetLandUseTypesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(lut.Msg.LandUseTypes) != 3 || lut.Msg.LandUseTypes[tu.FixtureID(tu.LandUseTypePrefix, 3)].GetCode() != "VAC" {
		t.Errorf("land use types = %v", lut.Msg.LandUseTypes)
	}

	it, err := r.improvements.GetImprovementTypes(ctx, connect.NewRequest(&improvementsv1.GetImprovementTypesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Msg.ImprovementTypes) != 2 || it.Msg.ImprovementTypes[tu.FixtureID(tu.ImprovementTypePrefix, 1)].GetCode() != "SFD" {
		t.Errorf("improvement types = %v", it.Msg.ImprovementTypes)
	}

	ic, err := r.improvements.GetImprovementConditions(ctx, connect.NewRequest(&improvementsv1.GetImprovementConditionsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := ic.Msg.ImprovementConditions[tu.FixtureID(tu.ImprovementConditionPrefix, 3)]; len(ic.Msg.ImprovementConditions) != 3 || got.GetName() != "Poor" || !near(got.GetDepreciationModifier(), 0.6) {
		t.Errorf("improvement conditions = %v", ic.Msg.ImprovementConditions)
	}

	z, err := r.zoning.GetZoning(ctx, connect.NewRequest(&zoningv1.GetZoningRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	r1 := z.Msg.Zoning[tu.FixtureID(tu.ZoningPrefix, 1)]
	if len(z.Msg.Zoning) != 2 || r1.GetCode() != "R-1" || !near(r1.GetMaxHeightFt(), 10*ftPerM) || !near(r1.GetMinLotSizeSqFt(), 300*sqftPerSqm) {
		t.Errorf("zoning = %v", z.Msg.Zoning)
	}

	v, err := r.valuations.GetValuations(ctx, connect.NewRequest(&valuationsv1.GetValuationsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Msg.Valuations[tu.FixtureID(tu.ValuationPrefix, 1)]; len(v.Msg.Valuations) != 1 || !got.GetValuationTimestamp().AsTime().Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("valuations = %v", v.Msg.Valuations)
	}

	nd, err := r.neighborhoods.GetNeighborhoodDefinitions(ctx, connect.NewRequest(&neighborhoodsv1.GetNeighborhoodDefinitionsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(nd.Msg.Neighborhoods) != 1 || nd.Msg.Neighborhoods[tu.FixtureID(tu.NeighborhoodDefPrefix, 1)].GetName() != "Fixture districts" {
		t.Errorf("neighborhood definitions = %v", nd.Msg.Neighborhoods)
	}

	n, err := r.neighborhoods.GetNeighborhoods(ctx, connect.NewRequest(&neighborhoodsv1.GetNeighborhoodsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Msg.Neighborhoods) != 2 || n.Msg.Neighborhoods[tu.FixtureID(tu.NeighborhoodPrefix, 2)].GetName() != "South Fixture" {
		t.Errorf("neighborhoods = %v", n.Msg.Neighborhoods)
	}
}

// parcel describes what the fixture says about parcel n.
type parcel struct{ n int }

func (p parcel) id() string        { return tu.ParcelID(p.n) }
func (p parcel) feature() int64    { return tu.ParcelFeatureID(p.n) }
func (p parcel) improved() bool    { return p.n <= tu.ImprovedParcels }
func (p parcel) areaSqft() float64 { return tu.ParcelLandAreaSqM(p.n) * sqftPerSqm }
func (p parcel) frontageFt() float64 {
	return (15 + 0.5*float64(p.n)) * ftPerM
}
func (p parcel) depthFt() float64 { return float64(25+p.n) * ftPerM }
func (p parcel) landUse() string  { return tu.FixtureID(tu.LandUsePrefix, tu.ParcelLandUse(p.n)) }
func (p parcel) zoning() string {
	if p.n <= 12 {
		return tu.FixtureID(tu.ZoningPrefix, 1)
	}
	return tu.FixtureID(tu.ZoningPrefix, 2)
}
func (p parcel) impAreaSqft() float64 {
	a := float64(120 + 5*p.n)
	if p.n == tu.SecondaryImpParcel {
		a += 30
	}
	return a * sqftPerSqm
}
func (p parcel) units() int32 {
	if p.n == tu.SecondaryImpParcel {
		return 2
	}
	return 1
}
func (p parcel) condition() string {
	return tu.FixtureID(tu.ImprovementConditionPrefix, (p.n-1)%3+1)
}
func (p parcel) impType() string {
	if p.n <= 12 {
		return tu.FixtureID(tu.ImprovementTypePrefix, 1)
	}
	return tu.FixtureID(tu.ImprovementTypePrefix, 2)
}

// queried are active parcels with and without improvements. The voided
// parcel and an unknown ID are asked for too and must never come back:
// voided parcels are hidden from every read.
var queried = []parcel{{1}, {3}, {13}, {17}}

func parcelIDs() []string {
	out := []string{tu.ParcelID(tu.VoidedParcel), tu.MissingID}
	for _, p := range queried {
		out = append(out, p.id())
	}
	return out
}

func featureIDs() []int64 {
	out := []int64{tu.ParcelFeatureID(tu.VoidedParcel), 999999}
	for _, p := range queried {
		out = append(out, p.feature())
	}
	return out
}

// checkValues compares a by-parcel or by-feature result against want for the
// parcels in queried (only improved ones when improvedOnly), and requires that
// nothing else came back.
func checkValues[K comparable, V any](t *testing.T, name string, got map[K]V, key func(parcel) K, want func(parcel) V, improvedOnly bool, eq func(V, V) bool) {
	t.Helper()
	expected := map[K]V{}
	for _, p := range queried {
		if improvedOnly && !p.improved() {
			continue
		}
		expected[key(p)] = want(p)
	}
	for k, w := range expected {
		g, ok := got[k]
		if !ok {
			t.Errorf("%s: missing %v", name, k)
		} else if !eq(g, w) {
			t.Errorf("%s[%v] = %v, want %v", name, k, g, w)
		}
	}
	for k := range got {
		if _, ok := expected[k]; !ok {
			t.Errorf("%s: unexpected key %v (voided, unknown or unimproved parcel)", name, k)
		}
	}
}

func eqF(a, b float64) bool { return near(a, b) }
func eqI(a, b int32) bool   { return a == b }
func eqS(a, b string) bool  { return a == b }

func TestAttributesByParcelID(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	c := r.parcels
	ids := parcelIDs()
	pid := func(p parcel) string { return p.id() }

	la, err := c.GetLandAreaSqftByParcelId(ctx, connect.NewRequest(&parcelsv1.GetLandAreaSqftByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "LandAreaSqft", la.Msg.Values, pid, parcel.areaSqft, false, eqF)

	fr, err := c.GetFrontageFtByParcelId(ctx, connect.NewRequest(&parcelsv1.GetFrontageFtByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "FrontageFt", fr.Msg.Values, pid, parcel.frontageFt, false, eqF)

	de, err := c.GetDepthFtByParcelId(ctx, connect.NewRequest(&parcelsv1.GetDepthFtByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "DepthFt", de.Msg.Values, pid, parcel.depthFt, false, eqF)

	lu, err := c.GetLandUseIdByParcelId(ctx, connect.NewRequest(&parcelsv1.GetLandUseIdByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "LandUseId", lu.Msg.Values, pid, parcel.landUse, false, eqS)

	zo, err := c.GetZoningIdByParcelId(ctx, connect.NewRequest(&parcelsv1.GetZoningIdByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ZoningId", zo.Msg.Values, pid, parcel.zoning, false, eqS)

	ia, err := c.GetImprovementAreaSqftByParcelId(ctx, connect.NewRequest(&parcelsv1.GetImprovementAreaSqftByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ImprovementAreaSqft", ia.Msg.Values, pid, parcel.impAreaSqft, true, eqF)

	be, err := c.GetBedroomsByParcelId(ctx, connect.NewRequest(&parcelsv1.GetBedroomsByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Bedrooms", be.Msg.Values, pid, func(parcel) int32 { return 3 }, true, eqI)

	ba, err := c.GetBathroomsByParcelId(ctx, connect.NewRequest(&parcelsv1.GetBathroomsByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Bathrooms", ba.Msg.Values, pid, func(parcel) int32 { return 2 }, true, eqI)

	un, err := c.GetUnitsByParcelId(ctx, connect.NewRequest(&parcelsv1.GetUnitsByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Units", un.Msg.Values, pid, parcel.units, true, eqI)

	yb, err := c.GetPrimaryImprovementYearBuiltByParcelId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementYearBuiltByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "YearBuilt", yb.Msg.Values, pid, func(p parcel) int32 { return int32(1980 + p.n) }, true, eqI)

	ey, err := c.GetPrimaryImprovementEffectiveYearBuiltByParcelId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementEffectiveYearBuiltByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "EffectiveYearBuilt", ey.Msg.Values, pid, func(p parcel) int32 { return int32(1990 + p.n) }, true, eqI)

	co, err := c.GetPrimaryImprovementConditionIdByParcelId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementConditionIdByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ConditionId", co.Msg.Values, pid, parcel.condition, true, eqS)

	ty, err := c.GetPrimaryImprovementTypeIdByParcelId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementTypeIdByParcelIdRequest{ParcelIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "TypeId", ty.Msg.Values, pid, parcel.impType, true, eqS)
}

func TestAttributesByFeatureID(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	c := r.parcels
	ids := featureIDs()
	fid := func(p parcel) int64 { return p.feature() }

	la, err := c.GetLandAreaSqftByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetLandAreaSqftByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "LandAreaSqft", la.Msg.Values, fid, parcel.areaSqft, false, eqF)

	fr, err := c.GetFrontageFtByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetFrontageFtByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "FrontageFt", fr.Msg.Values, fid, parcel.frontageFt, false, eqF)

	de, err := c.GetDepthFtByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetDepthFtByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "DepthFt", de.Msg.Values, fid, parcel.depthFt, false, eqF)

	lu, err := c.GetLandUseIdByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetLandUseIdByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "LandUseId", lu.Msg.Values, fid, parcel.landUse, false, eqS)

	zo, err := c.GetZoningIdByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetZoningIdByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ZoningId", zo.Msg.Values, fid, parcel.zoning, false, eqS)

	ia, err := c.GetImprovementAreaSqftByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetImprovementAreaSqftByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ImprovementAreaSqft", ia.Msg.Values, fid, parcel.impAreaSqft, true, eqF)

	be, err := c.GetBedroomsByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetBedroomsByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Bedrooms", be.Msg.Values, fid, func(parcel) int32 { return 3 }, true, eqI)

	ba, err := c.GetBathroomsByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetBathroomsByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Bathrooms", ba.Msg.Values, fid, func(parcel) int32 { return 2 }, true, eqI)

	un, err := c.GetUnitsByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetUnitsByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "Units", un.Msg.Values, fid, parcel.units, true, eqI)

	yb, err := c.GetPrimaryImprovementYearBuiltByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementYearBuiltByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "YearBuilt", yb.Msg.Values, fid, func(p parcel) int32 { return int32(1980 + p.n) }, true, eqI)

	ey, err := c.GetPrimaryImprovementEffectiveYearBuiltByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementEffectiveYearBuiltByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "EffectiveYearBuilt", ey.Msg.Values, fid, func(p parcel) int32 { return int32(1990 + p.n) }, true, eqI)

	co, err := c.GetPrimaryImprovementConditionIdByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementConditionIdByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ConditionId", co.Msg.Values, fid, parcel.condition, true, eqS)

	ty, err := c.GetPrimaryImprovementTypeIdByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetPrimaryImprovementTypeIdByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "TypeId", ty.Msg.Values, fid, parcel.impType, true, eqS)

	pi, err := c.GetParcelIdsByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetParcelIdsByFeatureIdRequest{FeatureIds: ids}))
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, "ParcelIds", pi.Msg.ParcelIds, fid, parcel.id, false, eqS)
}

func TestLegalAsOf(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	id := tu.ParcelID(tu.HistoricalParcel)
	for _, tc := range []struct {
		at   time.Time
		want float64
		ok   bool
	}{
		{time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC), 350 * sqftPerSqm, true},
		{time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), tu.ParcelLandAreaSqM(1) * sqftPerSqm, true},
		{time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC), 0, false}, // before any record
	} {
		res, err := r.parcels.GetLandAreaSqftByParcelId(ctx, connect.NewRequest(&parcelsv1.GetLandAreaSqftByParcelIdRequest{
			ParcelIds: []string{id}, LegalAsOf: timestamppb.New(tc.at),
		}))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := res.Msg.Values[id]
		if ok != tc.ok || (ok && !near(got, tc.want)) {
			t.Errorf("as of %s: got %v (present %v), want %v (present %v)", tc.at.Format("2006-01-02"), got, ok, tc.want, tc.ok)
		}
	}
}

func TestParcelsWithImprovementSummary(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	val, nd := tu.FixtureID(tu.ValuationPrefix, 1), tu.FixtureID(tu.NeighborhoodDefPrefix, 1)

	byID, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
		ParcelIds: parcelIDs(), ValuationId: &val, NeighborhoodDefinitionId: &nd,
	}))
	if err != nil {
		t.Fatal(err)
	}
	byFeature, err := r.parcels.GetParcelsWithImprovementSummaryByFeatureId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByFeatureIdRequest{
		FeatureIds: featureIDs(), ValuationId: &val, NeighborhoodDefinitionId: &nd,
	}))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{}
	for _, p := range queried {
		want = append(want, p.id())
	}
	sort.Strings(want)
	for name, got := range map[string]map[string]*parcelsv1.ParcelWithImprovementSummary{"by parcel": byID.Msg.Parcels, "by feature": byFeature.Msg.Parcels} {
		if keys := sortedKeys(got); len(keys) != len(want) || keys[0] != want[0] || keys[len(keys)-1] != want[len(want)-1] {
			t.Errorf("%s: got parcels %v, want %v", name, keys, want)
		}
	}

	p3 := byID.Msg.Parcels[tu.ParcelID(3)]
	d, s := p3.GetParcelDetails(), p3.GetImprovementSummary()
	if d.GetFeatureId() != tu.ParcelFeatureID(3) || d.GetAddressId() != tu.FixtureID(tu.AddressPrefix, 3) ||
		d.GetFormattedAddress() != "3 Fixture Way, Testville ZZ000" || d.GetLandUseId() != tu.FixtureID(tu.LandUsePrefix, 3) ||
		!near(d.GetLandAreaSqFt(), tu.ParcelLandAreaSqM(3)*sqftPerSqm) || d.GetPrimaryOwnerName() != "Fixture Owner 3" ||
		d.GetNeighborhoodId() != tu.FixtureID(tu.NeighborhoodPrefix, 1) {
		t.Errorf("parcel 3 details = %v", d)
	}
	if len(d.GetZoningIds()) != 1 || d.GetZoningIds()[0] != tu.FixtureID(tu.ZoningPrefix, 1) {
		t.Errorf("parcel 3 zoning = %v", d.GetZoningIds())
	}
	if d.GetMarketLandValue() == "" {
		t.Error("parcel 3 market land value missing with a valuation ID")
	}
	if len(s.GetImprovementIds()) != 2 || s.GetPrimaryImprovementId() != tu.FixtureID(tu.ImprovementPrefix, 3) ||
		s.GetTotalUnits() != 2 || !near(s.GetTotalAreaSqFt(), parcel{3}.impAreaSqft()) || s.GetPrimaryYearBuilt() != 1983 {
		t.Errorf("parcel 3 improvement summary = %v", s)
	}

	p2, err := r.parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, connect.NewRequest(&parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
		ParcelIds: []string{tu.ParcelID(tu.MultiOwnerParcel)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if parties := p2.Msg.Parcels[tu.ParcelID(tu.MultiOwnerParcel)].GetParcelDetails().GetPartyIds(); len(parties) != 2 {
		t.Errorf("parcel 2 party IDs = %v, want two owners", parties)
	}

	p17 := byID.Msg.Parcels[tu.ParcelID(17)]
	if s := p17.GetImprovementSummary(); len(s.GetImprovementIds()) != 0 || s.GetPrimaryImprovementId() != "" {
		t.Errorf("unimproved parcel 17 summary = %v", s)
	}
}

func TestEstimatedExtent(t *testing.T) {
	r := startReader(t)
	res, err := r.parcels.GetEstimatedParcelsExtentWGS84(context.Background(), connect.NewRequest(&parcelsv1.GetEstimatedParcelsExtentWGS84Request{}))
	if err != nil {
		t.Fatal(err)
	}
	m := res.Msg
	// Estimated extents may be padded; they must at least cover the grid.
	if m.GetMinX() > -140 || m.GetMaxX() < -139.9984 || m.GetMinY() > -40 || m.GetMaxY() < -39.998 || m.GetMinX() < -141 || m.GetMaxY() > -39 {
		t.Errorf("extent = %v", m)
	}
}

func TestEquityComparables(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()

	_, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no polygon or IDs: got %v, want InvalidArgument", err)
	}

	sel, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		SelectedParcelIds: ids(tu.ParcelPrefix, 4, 5),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(sel.Msg.Parcels); len(keys) != 2 {
		t.Errorf("selected parcels: got %v", keys)
	}

	// The polygon covers the first grid row: parcels 1-4.
	poly, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		WktPolygon: "POLYGON((-140.0001 -40.0001, -139.9983 -40.0001, -139.9983 -39.9997, -140.0001 -39.9997, -140.0001 -40.0001))",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(poly.Msg.Parcels); len(keys) != 4 {
		t.Errorf("polygon: got %d parcels %v, want parcels 1-4", len(keys), keys)
	}
}

// TestVoidedParcelHidden checks the voided parcel still has current rows
// (so the other tests' absence checks prove the filter, not missing data) and
// that the comparables RPCs drop it too.
func TestVoidedParcelHidden(t *testing.T) {
	pool := tu.FixtureDatabase(t).Pool(t)
	ctx := context.Background()
	var rows int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM core.parcel_attributes pa JOIN core.parcels p USING (parcel_id)
		WHERE p.public_id = $1 AND p.is_voided AND pa.legal_valid_range @> now()`, tu.ParcelID(tu.VoidedParcel)).Scan(&rows)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("fixture: voided parcel has %d current attribute rows, want 1", rows)
	}

	r := startReader(t)
	voided := tu.ParcelID(tu.VoidedParcel)
	eq, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		SelectedParcelIds: []string{voided, tu.ParcelID(19)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := eq.Msg.Parcels[voided]; ok || len(eq.Msg.Parcels) != 1 {
		t.Errorf("equity comparables returned %v, want only parcel 19", sortedKeys(eq.Msg.Parcels))
	}

	// The last grid row holds parcels 17-20; 20 is voided.
	poly, err := r.parcels.GetEquityComparables(ctx, connect.NewRequest(&parcelsv1.GetEquityComparablesRequest{
		WktPolygon: "POLYGON((-140.0001 -39.9983, -139.9983 -39.9983, -139.9983 -39.9981, -140.0001 -39.9981, -140.0001 -39.9983))",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := poly.Msg.Parcels[voided]; ok || len(poly.Msg.Parcels) != 3 {
		t.Errorf("polygon comparables returned %v, want parcels 17-19", sortedKeys(poly.Msg.Parcels))
	}
}
