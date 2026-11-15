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
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	dexv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/dex/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/dex/v1/dexv1connect"
	instancev1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/instance/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/instance/v1/instancev1connect"
	parcelsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1/parcelsv1connect"
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
		if len(res.Msg.Edges) != 1 || res.Msg.Edges[0].GetNode().GetTransferId() != tu.TransferID(tu.MultiGranteeTransfer) {
			t.Fatalf("parcel %d transfers = %v", tu.MultiOwnerParcel, res.Msg.Edges)
		}
		tr := res.Msg.Edges[0].GetNode()
		if len(tr.GetParcels()) != 1 || len(tr.GetParcels()[0].GetGrantees()) != 2 || len(tr.GetParties()) != 3 {
			t.Errorf("transfer %d = %v, want 2 grantee shares and 3 parties", tu.MultiGranteeTransfer, tr)
		}
		if pi := res.Msg.GetPageInfo(); pi.GetHasNextPage() || pi.GetEndCursor() != res.Msg.Edges[0].GetCursor() {
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
