//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1/parcelsv1connect"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func startWriter(t *testing.T) (parcelsv1connect.ParcelWriteServiceClient, *pgxpool.Pool) {
	t.Helper()
	pool := tu.FixtureDatabase(t).Pool(t)
	srv := httptest.NewServer(newHandler(pool, slog.New(slog.DiscardHandler)))
	t.Cleanup(srv.Close)
	return parcelsv1connect.NewParcelWriteServiceClient(http.DefaultClient, srv.URL), pool
}

// current is a parcel's attributes row valid now, read the way the reader
// reads it (stored metres converted back to feet).
type current struct {
	addressID, landUseID          string
	areaSqft, frontageFt, depthFt float64
	properties                    map[string]any
	traceID                       string
}

func readCurrent(t *testing.T, pool *pgxpool.Pool, n int) current {
	t.Helper()
	var c current
	var props []byte
	err := pool.QueryRow(context.Background(), `
		SELECT a.public_id::text, lu.public_id::text,
		       pa.land_area_sq_m * 10.7639, pa.frontage_m * 3.28084, pa.depth_m * 3.28084,
		       pa.properties, pa.trace_id
		FROM core.parcel_attributes pa
		JOIN core.parcels p ON p.parcel_id = pa.parcel_id
		JOIN core.addresses a ON a.address_id = pa.address_id
		JOIN core.land_uses lu ON lu.land_use_id = pa.land_use_id
		WHERE p.public_id = $1 AND pa.legal_valid_range @> now()`, tu.ParcelID(n)).
		Scan(&c.addressID, &c.landUseID, &c.areaSqft, &c.frontageFt, &c.depthFt, &props, &c.traceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(props, &c.properties); err != nil {
		t.Fatal(err)
	}
	return c
}

func historyCount(t *testing.T, pool *pgxpool.Pool, n int) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM core.parcel_attributes_history h
		JOIN core.parcels p ON p.parcel_id = h.parcel_id WHERE p.public_id = $1`, tu.ParcelID(n)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }

func update(t *testing.T, c parcelsv1connect.ParcelWriteServiceClient, msg *parcelsv1.UpdateParcelRequest) error {
	t.Helper()
	req := connect.NewRequest(msg)
	req.Header().Set("traceparent", traceparent)
	_, err := c.UpdateParcel(context.Background(), req)
	return err
}

func TestUpdateParcelFull(t *testing.T) {
	c, pool := startWriter(t)
	const n = 5
	before := readCurrent(t, pool, n)
	historyBefore := historyCount(t, pool, n)

	err := update(t, c, &parcelsv1.UpdateParcelRequest{
		ParcelId:     tu.ParcelID(n),
		AddressId:    ptr(tu.FixtureID(tu.AddressPrefix, tu.UnattachedAddress)),
		LandUseId:    ptr(tu.FixtureID(tu.LandUsePrefix, 3)),
		LandAreaSqFt: ptr(6000.5),
		FrontageFt:   ptr(52.25),
		DepthFt:      ptr(101.0),
		Properties:   ptr(`{"replaced": true}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	got := readCurrent(t, pool, n)
	if got.addressID != tu.FixtureID(tu.AddressPrefix, tu.UnattachedAddress) || got.landUseID != tu.FixtureID(tu.LandUsePrefix, 3) {
		t.Errorf("address/land use = %s / %s", got.addressID, got.landUseID)
	}
	// Units round-trip exactly through the reader's conversion.
	if !near(got.areaSqft, 6000.5) || !near(got.frontageFt, 52.25) || !near(got.depthFt, 101) {
		t.Errorf("area %v, frontage %v, depth %v; want 6000.5, 52.25, 101", got.areaSqft, got.frontageFt, got.depthFt)
	}
	// properties is replaced, not merged.
	if len(got.properties) != 1 || got.properties["replaced"] != true {
		t.Errorf("properties = %v, want only {replaced: true}", got.properties)
	}
	if got.traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id = %q, want the traceparent's trace ID", got.traceID)
	}

	// The trigger recorded the previous version, with its own trace ID.
	if h := historyCount(t, pool, n); h != historyBefore+1 {
		t.Fatalf("history rows %d, want %d", h, historyBefore+1)
	}
	var oldArea float64
	var oldTrace string
	err = pool.QueryRow(context.Background(), `
		SELECT h.land_area_sq_m * 10.7639, h.trace_id FROM core.parcel_attributes_history h
		JOIN core.parcels p ON p.parcel_id = h.parcel_id
		WHERE p.public_id = $1 ORDER BY h.parcel_attribute_history_id DESC LIMIT 1`, tu.ParcelID(n)).Scan(&oldArea, &oldTrace)
	if err != nil {
		t.Fatal(err)
	}
	if !near(oldArea, before.areaSqft) || oldTrace != before.traceID {
		t.Errorf("history row area %v trace %q, want %v %q", oldArea, oldTrace, before.areaSqft, before.traceID)
	}
}

func TestUpdateParcelPartial(t *testing.T) {
	c, pool := startWriter(t)
	const n = 6
	before := readCurrent(t, pool, n)

	if err := update(t, c, &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(n), FrontageFt: ptr(77.0)}); err != nil {
		t.Fatal(err)
	}
	got := readCurrent(t, pool, n)
	if !near(got.frontageFt, 77) {
		t.Errorf("frontage %v, want 77", got.frontageFt)
	}
	if got.addressID != before.addressID || got.landUseID != before.landUseID || !near(got.areaSqft, before.areaSqft) ||
		!near(got.depthFt, before.depthFt) || len(got.properties) != len(before.properties) {
		t.Errorf("untouched fields changed:\n before %+v\n after  %+v", before, got)
	}
}

func TestUpdateParcelWithoutTraceparent(t *testing.T) {
	c, pool := startWriter(t)
	_, err := c.UpdateParcel(context.Background(), connect.NewRequest(&parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(7), DepthFt: ptr(10.0)}))
	if err != nil {
		t.Fatal(err)
	}
	if got := readCurrent(t, pool, 7).traceID; len(got) != 32 || got == "fixture" {
		t.Errorf("trace_id = %q, want a generated 32-character ID", got)
	}
}

func TestUpdateParcelErrors(t *testing.T) {
	c, pool := startWriter(t)

	random := make([]byte, 8000)
	_, _ = rand.Read(random)
	oversized := `{"blob":"` + hex.EncodeToString(random) + `"}` // incompressible, well over 5000 bytes stored

	tests := []struct {
		name string
		req  *parcelsv1.UpdateParcelRequest
		want connect.Code
	}{
		{"unknown parcel", &parcelsv1.UpdateParcelRequest{ParcelId: tu.MissingID, DepthFt: ptr(1.0)}, connect.CodeNotFound},
		{"voided parcel", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(tu.VoidedParcel), DepthFt: ptr(1.0)}, connect.CodeNotFound},
		{"unknown address", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(8), AddressId: ptr(tu.MissingID)}, connect.CodeInvalidArgument},
		{"unknown land use", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(8), LandUseId: ptr(tu.MissingID)}, connect.CodeInvalidArgument},
		{"negative frontage", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(8), FrontageFt: ptr(-3.0)}, connect.CodeInvalidArgument},
		{"properties not an object", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(8), Properties: ptr(`[1]`)}, connect.CodeInvalidArgument},
		{"oversized properties", &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(8), Properties: &oversized}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := historyCount(t, pool, 8)
			err := update(t, c, tt.req)
			if code := connect.CodeOf(err); code != tt.want {
				t.Errorf("got %v (%v), want %v", code, err, tt.want)
			}
			if after := historyCount(t, pool, 8); after != before {
				t.Errorf("a rejected update wrote %d history rows", after-before)
			}
		})
	}
}

func TestUpdateParcelNoCurrentRecord(t *testing.T) {
	c, pool := startWriter(t)
	const n = 9
	// End parcel 9's only attributes record in the past.
	_, err := pool.Exec(context.Background(), `
		UPDATE core.parcel_attributes SET legal_valid_range = tstzrange('2020-01-01', '2021-01-01')
		WHERE parcel_id = (SELECT parcel_id FROM core.parcels WHERE public_id = $1)`, tu.ParcelID(n))
	if err != nil {
		t.Fatal(err)
	}
	err = update(t, c, &parcelsv1.UpdateParcelRequest{ParcelId: tu.ParcelID(n), DepthFt: ptr(1.0)})
	if code := connect.CodeOf(err); code != connect.CodeFailedPrecondition {
		t.Errorf("got %v (%v), want FailedPrecondition", code, err)
	}
}
