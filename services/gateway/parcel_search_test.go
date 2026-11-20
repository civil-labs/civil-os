package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	publicparcelsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1/parcelsv1connect"
	meshparcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
)

// Gateway tests for SearchParcelsAddresses (docs/parcel-search/spec.md,
// "Tests"). The reader's behaviour is tested in the database-reader.

func parcelsClient(g *gateway) parcelsv1connect.ParcelsServiceClient {
	return parcelsv1connect.NewParcelsServiceClient(http.DefaultClient, g.url,
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+g.token)
				return next(ctx, req)
			}
		})))
}

func TestSearchParcelsAddressesValidation(t *testing.T) {
	g := startGateway(t, nil)
	c := parcelsClient(g)
	ctx := context.Background()
	ptr := func(v int32) *int32 { return &v }
	emptyCursor, longCursor := "", strings.Repeat("a", 513)

	for name, req := range map[string]*publicparcelsv1.SearchParcelsAddressesRequest{
		"empty query":          {},
		"201-character query":  {Query: strings.Repeat("a", 201)},
		"first = 0":            {Query: "12 Fixture Way", First: ptr(0)},
		"first = 101":          {Query: "12 Fixture Way", First: ptr(101)},
		"legal_as_of < year 0": {Query: "12 Fixture Way", LegalAsOf: &timestamppb.Timestamp{Seconds: -62135596801}},
		"empty cursor":         {Query: "12 Fixture Way", After: &emptyCursor},
		"oversized cursor":     {Query: "12 Fixture Way", After: &longCursor},
	} {
		g.mesh.reset(0)
		_, err := c.SearchParcelsAddresses(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
		if calls := g.mesh.reset(0); len(calls) != 0 {
			t.Errorf("%s: invalid request reached the mesh", name)
		}
	}

	// The longest valid query passes. The reader, not the gateway, checks
	// that it has 3 letters or digits.
	g.mesh.reset(0)
	if _, err := c.SearchParcelsAddresses(ctx, connect.NewRequest(&publicparcelsv1.SearchParcelsAddressesRequest{Query: strings.Repeat("a", 200)})); err != nil {
		t.Errorf("200-character query: %v", err)
	}
	g.mesh.reset(0)
	if _, err := c.SearchParcelsAddresses(ctx, connect.NewRequest(&publicparcelsv1.SearchParcelsAddressesRequest{Query: "--"})); err != nil {
		t.Errorf("query with no letters reached the gateway's own checks: %v", err)
	}
}

func TestSearchParcelsAddressesForwarding(t *testing.T) {
	g := startGateway(t, nil)
	c := parcelsClient(g)
	ctx := context.Background()

	g.mesh.reset(0)
	if _, err := c.SearchParcelsAddresses(ctx, connect.NewRequest(&publicparcelsv1.SearchParcelsAddressesRequest{Query: "12 Fixture Way"})); err != nil {
		t.Fatal(err)
	}
	if got := meshFirst(t, g); got != 25 {
		t.Errorf("unset first reached the mesh as %d, want 25", got)
	}

	// Cursors are the reader's: the gateway passes them through unchanged.
	cursor := "opaque-cursor_from-the-reader"
	g.mesh.reset(0)
	if _, err := c.SearchParcelsAddresses(ctx, connect.NewRequest(&publicparcelsv1.SearchParcelsAddressesRequest{Query: "12 Fixture Way", After: &cursor})); err != nil {
		t.Fatal(err)
	}
	calls := g.mesh.reset(0)
	if len(calls) != 1 {
		t.Fatalf("got %d mesh calls, want 1", len(calls))
	}
	req := calls[0].req
	if got := req.Get(req.Descriptor().Fields().ByName("after")).String(); got != cursor {
		t.Errorf("after reached the mesh as %q, want %q", got, cursor)
	}
	if got := req.Get(req.Descriptor().Fields().ByName("query")).String(); got != "12 Fixture Way" {
		t.Errorf("query reached the mesh as %q, want it unchanged", got)
	}
}

func TestMapParcelSearchHit(t *testing.T) {
	checkFullMapping(t, &meshparcelsv1.ParcelSearchHit{}, mapParcelSearchHit)
}
