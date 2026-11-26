//go:build e2e

// Package e2e runs smoke tests against a running compose stack:
//
//	mise run test:e2e  // starts or rebuilds the stack, then runs these tests
//
// or, without mise: docker compose up --build --wait, then
// go test -tags e2e ./test/e2e/...
//
// CIVIL_OS_E2E_GATEWAY_URL and CIVIL_OS_E2E_ISSUER_URL override the defaults
// (http://localhost:8080 and http://localhost:5556/dex).
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	addressesv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/addresses/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/addresses/v1/addressesv1connect"
	dexv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/dex/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/dex/v1/dexv1connect"
	improvementsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1/improvementsv1connect"
	instancev1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/instance/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/instance/v1/instancev1connect"
	neighborhoodsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/neighborhoods/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/neighborhoods/v1/neighborhoodsv1connect"
	parcelsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1/parcelsv1connect"
	partiesv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parties/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/parties/v1/partiesv1connect"
	transfersv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/transfers/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/transfers/v1/transfersv1connect"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Throwaway credentials from dev/dex.yaml.
const (
	clientID     = "civil-os-dev"
	clientSecret = "civil-os-dev-secret"
	username     = "dev@example.com"
	password     = "civil-os-dev-password"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	gatewayURL = env("CIVIL_OS_E2E_GATEWAY_URL", "http://localhost:8080")
	issuerURL  = env("CIVIL_OS_E2E_ISSUER_URL", "http://localhost:5556/dex")
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

// waitHealthy polls the gateway's /health until it answers 200.
func waitHealthy(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		res, err := httpClient.Get(gatewayURL + "/health")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("gateway at %s not healthy (last error %v); is `docker compose up --wait` running?", gatewayURL, err)
		}
		time.Sleep(time.Second)
	}
}

// login gets an ID token from Dex with the password grant.
func login(t *testing.T) string {
	t.Helper()
	form := url.Values{
		"grant_type":    {"password"},
		"username":      {username},
		"password":      {password},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {"openid email profile"},
	}
	res, err := httpClient.PostForm(issuerURL+"/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("token request: %d %s", res.StatusCode, body)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		t.Fatalf("no id_token in %s (%v)", body, err)
	}
	return tok.IDToken
}

func authed[T any](token string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}

func TestSmoke(t *testing.T) {
	waitHealthy(t)
	token := login(t)
	ctx := context.Background()
	parcels := parcelsv1connect.NewParcelsServiceClient(httpClient, gatewayURL)

	t.Run("unauthenticated read is rejected", func(t *testing.T) {
		_, err := parcels.GetFrontageFtByParcelId(ctx, connect.NewRequest(&parcelsv1.GetFrontageFtByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(1)},
		}))
		if code := connect.CodeOf(err); code != connect.CodeUnauthenticated {
			t.Errorf("got %v (%v), want Unauthenticated", code, err)
		}
	})

	t.Run("search parcels by address", func(t *testing.T) {
		res, err := parcels.SearchParcelsAddresses(ctx, authed(token, &parcelsv1.SearchParcelsAddressesRequest{
			Query: "3 Fixture Way",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Msg.Hits) == 0 || res.Msg.Hits[0].GetParcelId() != tu.ParcelID(3) {
			t.Fatalf("3 Fixture Way = %v, want parcel 3 first", res.Msg.Hits)
		}
		if res.Msg.PageInfo.GetEndCursor() == "" {
			t.Error("a non-empty page has no end_cursor")
		}
	})

	t.Run("read, update, read back", func(t *testing.T) {
		const n = 2
		id := tu.ParcelID(n)

		res, err := parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, authed(token, &parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
			ParcelIds: []string{id},
		}))
		if err != nil {
			t.Fatal(err)
		}
		details := res.Msg.Parcels[id].GetParcelDetails()
		if details.GetFeatureId() != tu.ParcelFeatureID(n) || details.GetFormattedAddress() == "" {
			t.Fatalf("parcel %d = %v", n, details)
		}

		// A different value on every run, so the stack can be reused.
		want := 40 + float64(time.Now().UnixNano()%100000)/1000
		if math.Abs(details.GetFrontageFt()-want) < 1e-6 {
			want += 1
		}
		_, err = parcels.UpdateParcel(ctx, authed(token, &parcelsv1.UpdateParcelRequest{
			ParcelId:   id,
			FrontageFt: &want,
		}))
		if err != nil {
			t.Fatalf("UpdateParcel: %v", err)
		}

		back, err := parcels.GetFrontageFtByParcelId(ctx, authed(token, &parcelsv1.GetFrontageFtByParcelIdRequest{
			ParcelIds: []string{id},
		}))
		if err != nil {
			t.Fatal(err)
		}
		if got := back.Msg.Values[id]; math.Abs(got-want) > 1e-6 {
			t.Errorf("frontage read back %v, want %v", got, want)
		}
	})

	t.Run("bathrooms", func(t *testing.T) {
		// The half-bath parcel has 2 full and 1 half bath; parcel 17 has no
		// improvements. Each public response must also satisfy its own
		// protovalidate rules, which the gateway doesn't check on responses
		// (docs/full-half-bathrooms/spec.md).
		half, none := tu.HalfBathParcel, 17
		ids := []string{tu.ParcelID(half), tu.ParcelID(none)}
		features := []int64{tu.ParcelFeatureID(half), tu.ParcelFeatureID(none)}

		sum, err := parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, authed(token, &parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{ParcelIds: ids}))
		if err != nil {
			t.Fatal(err)
		}
		byParcel, err := parcels.GetBathroomsByParcelId(ctx, authed(token, &parcelsv1.GetBathroomsByParcelIdRequest{ParcelIds: ids}))
		if err != nil {
			t.Fatal(err)
		}
		byFeature, err := parcels.GetBathroomsByFeatureId(ctx, authed(token, &parcelsv1.GetBathroomsByFeatureIdRequest{FeatureIds: features}))
		if err != nil {
			t.Fatal(err)
		}
		for name, msg := range map[string]proto.Message{"summary": sum.Msg, "by parcel": byParcel.Msg, "by feature": byFeature.Msg} {
			if err := protovalidate.Validate(msg); err != nil {
				t.Errorf("%s response breaks its rules: %v", name, err)
			}
		}

		if s := sum.Msg.Parcels[tu.ParcelID(half)].GetImprovementSummary(); s.GetTotalFullBathrooms() != 2 || s.GetTotalHalfBathrooms() != 1 || s.GetTotalBathrooms() != 2.5 {
			t.Errorf("parcel %d summary bathrooms = %d full, %d half, %v total; want 2, 1, 2.5",
				half, s.GetTotalFullBathrooms(), s.GetTotalHalfBathrooms(), s.GetTotalBathrooms())
		}
		if s := sum.Msg.Parcels[tu.ParcelID(none)].GetImprovementSummary(); s.GetTotalFullBathrooms() != 0 || s.GetTotalHalfBathrooms() != 0 || s.GetTotalBathrooms() != 0 {
			t.Errorf("unimproved parcel %d summary bathrooms = %v, want all 0", none, s)
		}
		if got := byParcel.Msg.Values[tu.ParcelID(half)]; got != 2.5 {
			t.Errorf("GetBathroomsByParcelId[%d] = %v, want 2.5", half, got)
		}
		if got := byFeature.Msg.Values[tu.ParcelFeatureID(half)]; got != 2.5 {
			t.Errorf("GetBathroomsByFeatureId[%d] = %v, want 2.5", half, got)
		}
	})

	t.Run("update of unknown parcel is NotFound", func(t *testing.T) {
		v := 10.0
		_, err := parcels.UpdateParcel(ctx, authed(token, &parcelsv1.UpdateParcelRequest{ParcelId: tu.MissingID, FrontageFt: &v}))
		if code := connect.CodeOf(err); code != connect.CodeNotFound {
			t.Errorf("got %v (%v), want NotFound", code, err)
		}
	})

	t.Run("tile", func(t *testing.T) {
		tile := tu.FixtureTile
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/tiles/get_parcel_tiles/%d/%d/%d", gatewayURL, tile.Z, tile.X, tile.Y), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK || len(body) == 0 {
			t.Errorf("tile: status %d, %d bytes; want 200 and a non-empty MVT", res.StatusCode, len(body))
		}
		if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "protobuf") {
			t.Errorf("tile Content-Type %q", ct)
		}
	})

	t.Run("tile JSON", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, gatewayURL+"/tiles/get_parcel_tiles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var tj struct {
			Tiles []string `json:"tiles"`
		}
		if err := json.NewDecoder(res.Body).Decode(&tj); err != nil || res.StatusCode != http.StatusOK || len(tj.Tiles) == 0 {
			t.Errorf("TileJSON: status %d, tiles %v, err %v", res.StatusCode, tj.Tiles, err)
		}
	})

	t.Run("real property transfers", func(t *testing.T) {
		transfers := transfersv1connect.NewRealPropertyTransfersServiceClient(httpClient, gatewayURL)
		res, err := transfers.GetTransfersByParcelId(ctx, authed(token, &transfersv1.GetTransfersByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(tu.MultiOwnerParcel)},
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Msg.Transfers) != 1 || res.Msg.Transfers[0].GetTransferId() != tu.TransferID(tu.MultiGranteeTransfer) {
			t.Fatalf("parcel %d transfers = %v", tu.MultiOwnerParcel, res.Msg.Transfers)
		}
		tr := res.Msg.Transfers[0]
		if len(tr.GetParcels()) != 1 || len(tr.GetParcels()[0].GetGrantees()) != 2 || len(tr.GetParties()) != 3 {
			t.Errorf("transfer %d = %v, want 2 grantee shares and 3 parties", tu.MultiGranteeTransfer, tr)
		}
		if pi := res.Msg.GetPageInfo(); pi.GetHasNextPage() || pi.GetEndCursor() == "" || pi.GetEndCursor() != pi.GetStartCursor() {
			t.Errorf("page info = %v", pi)
		}

		tooBig := int32(101)
		_, err = transfers.GetTransfersByParcelId(ctx, authed(token, &transfersv1.GetTransfersByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(1)}, First: &tooBig,
		}))
		if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
			t.Errorf("first = 101: got %v (%v), want InvalidArgument", code, err)
		}
	})

	t.Run("addresses and parties", func(t *testing.T) {
		// Parcel 3 -> its owner, party 3 -> the owner's parcels and mailing address.
		parties := partiesv1connect.NewPartiesServiceClient(httpClient, gatewayURL)
		parcels := parcelsv1connect.NewParcelsServiceClient(httpClient, gatewayURL)
		pr, err := parties.GetPartiesById(ctx, authed(token, &partiesv1.GetPartiesByIdRequest{
			PartyIds: []string{tu.PartyID(3)},
		}))
		if err != nil {
			t.Fatal(err)
		}
		owner := pr.Msg.GetParties()[tu.PartyID(3)]
		if owner.GetName() != "Fixture Owner 3" || owner.GetMailingAddressId() != tu.AddressID(3) {
			t.Fatalf("party 3 = %v", owner)
		}

		one := int32(1)
		pp, err := parcels.GetParcelsByPartyId(ctx, authed(token, &parcelsv1.GetParcelsByPartyIdRequest{
			PartyId: tu.PartyID(3), First: &one,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(pp.Msg.Parcels) != 1 || !pp.Msg.GetPageInfo().GetHasNextPage() {
			t.Fatalf("party 3 parcels, first page = %v", pp.Msg)
		}
		next, err := parcels.GetParcelsByPartyId(ctx, authed(token, &parcelsv1.GetParcelsByPartyIdRequest{
			PartyId: tu.PartyID(3), After: pp.Msg.GetPageInfo().EndCursor,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(next.Msg.Parcels) != 2 || next.Msg.GetPageInfo().GetHasNextPage() {
			t.Errorf("party 3 parcels after the first = %v, want the other 2", next.Msg)
		}

		addresses := addressesv1connect.NewAddressesServiceClient(httpClient, gatewayURL)
		ar, err := addresses.GetAddressesById(ctx, authed(token, &addressesv1.GetAddressesByIdRequest{
			AddressIds: []string{owner.GetMailingAddressId()},
		}))
		if err != nil {
			t.Fatal(err)
		}
		if a := ar.Msg.GetAddresses()[tu.AddressID(3)]; a.GetCountry().GetCode() != "ZZ" || a.GetCoordinates() == nil {
			t.Errorf("address 3 = %v", a)
		}
		ap, err := parcels.GetParcelsByAddressId(ctx, authed(token, &parcelsv1.GetParcelsByAddressIdRequest{
			AddressId: tu.AddressID(3),
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(ap.Msg.Parcels) != 1 || ap.Msg.Parcels[0].GetParcelId() != tu.ParcelID(3) {
			t.Errorf("parcels at address 3 = %v", ap.Msg)
		}

		_, err = parcels.GetParcelsByPartyId(ctx, authed(token, &parcelsv1.GetParcelsByPartyIdRequest{PartyId: "not-a-uuid"}))
		if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
			t.Errorf("non-UUID party: got %v (%v), want InvalidArgument", code, err)
		}
	})

	t.Run("parcel extents", func(t *testing.T) {
		parcels := parcelsv1connect.NewParcelsServiceClient(httpClient, gatewayURL)
		res, err := parcels.GetParcelExtentsWGS84ByParcelId(ctx, authed(token, &parcelsv1.GetParcelExtentsWGS84ByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(1)},
		}))
		if err != nil {
			t.Fatal(err)
		}
		// Parcel 1 is the 0.0004-degree square at (-140, -40).
		e := res.Msg.GetExtents()[tu.ParcelID(1)]
		if e == nil || e.GetMinX() != -140 || e.GetMinY() != -40 || e.GetMaxX() <= e.GetMinX() || e.GetMaxY() <= e.GetMinY() {
			t.Errorf("parcel 1 extent = %v", e)
		}
	})

	t.Run("transfers by party", func(t *testing.T) {
		transfers := transfersv1connect.NewRealPropertyTransfersServiceClient(httpClient, gatewayURL)
		res, err := transfers.GetTransfersByPartyId(ctx, authed(token, &transfersv1.GetTransfersByPartyIdRequest{
			PartyIds: []string{tu.PartyID(1)},
		}))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, tr := range res.Msg.Transfers {
			got = append(got, tr.GetTransferId())
		}
		if want := []string{tu.TransferID(5), tu.TransferID(2)}; !slices.Equal(got, want) {
			t.Errorf("party 1 transfers = %v, want %v", got, want)
		}
	})

	t.Run("improvement qualities", func(t *testing.T) {
		improvements := improvementsv1connect.NewImprovementsServiceClient(httpClient, gatewayURL)
		res, err := improvements.GetImprovementQualities(ctx, authed(token, &improvementsv1.GetImprovementQualitiesRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		q := res.Msg.GetImprovementQualities()
		if len(q) != 2 || q[tu.ImprovementQualityID(1)].GetName() != "Fixture Standard" {
			t.Errorf("improvement qualities = %v", q)
		}
	})

	t.Run("improvements by ID", func(t *testing.T) {
		// Improvement 1 and the secondary building on parcel 3, which has its
		// own owner. The response must satisfy its own protovalidate rules.
		improvements := improvementsv1connect.NewImprovementsServiceClient(httpClient, gatewayURL)
		val := tu.FixtureID(tu.ValuationPrefix, 1)
		res, err := improvements.GetImprovementsById(ctx, authed(token, &improvementsv1.GetImprovementsByIdRequest{
			ImprovementIds: []string{tu.ImprovementID(1), tu.ImprovementID(tu.SecondaryImprovement)},
			ValuationId:    &val,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if err := protovalidate.Validate(res.Msg); err != nil {
			t.Errorf("response breaks its rules: %v", err)
		}
		got := res.Msg.GetImprovements()
		if m := got[tu.ImprovementID(1)]; m.GetBathrooms() != 2.5 || m.GetMarketValue() != "82000.0000" ||
			len(m.GetParcels()) != 1 || m.GetParcels()[0].GetParcelId() != tu.ParcelID(1) {
			t.Errorf("improvement 1 = %v", m)
		}
		if m := got[tu.ImprovementID(tu.SecondaryImprovement)]; len(m.GetParties()) != 1 || m.GetParties()[0].GetPartyId() != tu.PartyID(5) {
			t.Errorf("improvement 16 = %v, want party 5 only", m)
		}
	})

	t.Run("neighborhoods", func(t *testing.T) {
		// Over Connect GET: the RPCs are NO_SIDE_EFFECTS.
		nbhds := neighborhoodsv1connect.NewNeighborhoodsServiceClient(httpClient, gatewayURL, connect.WithHTTPGet())
		defs, err := nbhds.GetNeighborhoodDefinitions(ctx, authed(token, &neighborhoodsv1.GetNeighborhoodDefinitionsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if d := defs.Msg.NeighborhoodDefinitions[tu.NeighborhoodDefinitionID(tu.LegalNeighborhoodDefinition)]; !d.GetIsLegal() {
			t.Errorf("legal definition = %v, want is_legal", d)
		}

		nonLegal := tu.NeighborhoodDefinitionID(tu.NonLegalNeighborhoodDefinition)
		res, err := nbhds.GetNeighborhoods(ctx, authed(token, &neighborhoodsv1.GetNeighborhoodsRequest{
			NeighborhoodDefinitionId: &nonLegal,
		}))
		if err != nil {
			t.Fatal(err)
		}
		got := slices.Sorted(maps.Keys(res.Msg.Neighborhoods))
		want := []string{tu.NeighborhoodID(tu.NorthNeighborhood), tu.NeighborhoodID(tu.MarketAreaNeighborhood)}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("neighborhoods for the non-legal definition = %v, want %v", got, want)
		}

		p, err := parcels.GetParcelsWithImprovementSummaryByParcelId(ctx, authed(token, &parcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
			ParcelIds: []string{tu.ParcelID(3)}, NeighborhoodDefinitionId: &nonLegal,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Msg.Parcels[tu.ParcelID(3)].GetParcelDetails().GetNeighborhoodId(); got != tu.NeighborhoodID(tu.MarketAreaNeighborhood) {
			t.Errorf("parcel 3 neighborhood under the non-legal definition = %q", got)
		}
	})

	t.Run("instance metadata", func(t *testing.T) {
		inst := instancev1connect.NewInstanceServiceClient(httpClient, gatewayURL)
		res, err := inst.GetInstanceMetadata(ctx, connect.NewRequest(&instancev1.GetInstanceMetadataRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if res.Msg.GetAuthIssuerUrl() != issuerURL {
			t.Errorf("auth issuer %q, want %q", res.Msg.GetAuthIssuerUrl(), issuerURL)
		}
		if res.Msg.GetMetadata().GetFields()["name"].GetStringValue() == "" {
			t.Errorf("metadata = %v", res.Msg.GetMetadata())
		}
	})

	t.Run("dex admin API", func(t *testing.T) {
		dex := dexv1connect.NewDexServiceClient(httpClient, gatewayURL)
		res, err := dex.ListClients(ctx, authed(token, &dexv1.ListClientsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range res.Msg.GetClients() {
			if c.GetId() == clientID {
				return
			}
		}
		t.Errorf("ListClients = %v, want %s", res.Msg.GetClients(), clientID)
	})
}
