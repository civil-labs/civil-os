package main

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/interval"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	publictransfersv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/transfers/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/transfers/v1/transfersv1connect"
	meshpaginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	meshtransfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
	"github.com/civil-labs/civil-os/internal/testutil"
)

func transfersClient(g *gateway) transfersv1connect.RealPropertyTransfersServiceClient {
	return transfersv1connect.NewRealPropertyTransfersServiceClient(http.DefaultClient, g.url,
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+g.token)
				return next(ctx, req)
			}
		})))
}

// meshFirst returns the first field of the single mesh request g received,
// or -1 when it was unset.
func meshFirst(t *testing.T, g *gateway) int32 {
	t.Helper()
	calls := g.mesh.reset(0)
	if len(calls) != 1 {
		t.Fatalf("got %d mesh calls, want 1", len(calls))
	}
	fd := calls[0].req.Descriptor().Fields().ByName("first")
	if !calls[0].req.Has(fd) {
		return -1
	}
	return int32(calls[0].req.Get(fd).Int())
}

func TestTransferPageSizeDefault(t *testing.T) {
	g := startGateway(t, nil)
	c := transfersClient(g)
	ctx := context.Background()
	parcel := []string{testutil.ParcelID(1)}

	g.mesh.reset(0)
	if _, err := c.GetTransfersByParcelId(ctx, connect.NewRequest(&publictransfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel})); err != nil {
		t.Fatal(err)
	}
	if got := meshFirst(t, g); got != 25 {
		t.Errorf("unset first reached the mesh as %d, want 25", got)
	}

	if _, err := c.GetTransfersByFeatureId(ctx, connect.NewRequest(&publictransfersv1.GetTransfersByFeatureIdRequest{FeatureIds: []int64{1001}})); err != nil {
		t.Fatal(err)
	}
	if got := meshFirst(t, g); got != 25 {
		t.Errorf("feature lookup: unset first reached the mesh as %d, want 25", got)
	}

	if _, err := c.GetTransfersByPartyId(ctx, connect.NewRequest(&publictransfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{testutil.PartyID(1)}})); err != nil {
		t.Fatal(err)
	}
	if got := meshFirst(t, g); got != 25 {
		t.Errorf("party lookup: unset first reached the mesh as %d, want 25", got)
	}

	for _, first := range []int32{1, 100} {
		if _, err := c.GetTransfersByParcelId(ctx, connect.NewRequest(&publictransfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel, First: &first})); err != nil {
			t.Fatalf("first = %d: %v", first, err)
		}
		if got := meshFirst(t, g); got != first {
			t.Errorf("first = %d reached the mesh as %d", first, got)
		}
	}
}

func TestTransferValidation(t *testing.T) {
	g := startGateway(t, nil)
	c := transfersClient(g)
	ctx := context.Background()

	ptr := func(v int32) *int32 { return &v }
	parcel := []string{testutil.ParcelID(1)}
	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = testutil.ParcelID(i + 1)
	}
	inverted := &interval.Interval{
		StartTime: timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		EndTime:   timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	emptyCursor, longCursor := "", strings.Repeat("a", 513)

	byParcel := map[string]*publictransfersv1.GetTransfersByParcelIdRequest{
		"no IDs":           {},
		"101 IDs":          {ParcelIds: tooMany},
		"duplicate IDs":    {ParcelIds: []string{testutil.ParcelID(1), testutil.ParcelID(1)}},
		"non-UUID ID":      {ParcelIds: []string{"not-a-uuid"}},
		"first = 0":        {ParcelIds: parcel, First: ptr(0)},
		"first = 101":      {ParcelIds: parcel, First: ptr(101)},
		"inverted range":   {ParcelIds: parcel, TimeRange: inverted},
		"empty cursor":     {ParcelIds: parcel, After: &emptyCursor},
		"oversized cursor": {ParcelIds: parcel, After: &longCursor},
	}
	for name, req := range byParcel {
		g.mesh.reset(0)
		_, err := c.GetTransfersByParcelId(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("by parcel, %s: got %v, want InvalidArgument", name, err)
		}
		if calls := g.mesh.reset(0); len(calls) != 0 {
			t.Errorf("by parcel, %s: invalid request reached the mesh", name)
		}
	}

	byFeature := map[string]*publictransfersv1.GetTransfersByFeatureIdRequest{
		"no IDs":         {},
		"zero ID":        {FeatureIds: []int64{0}},
		"duplicate IDs":  {FeatureIds: []int64{1001, 1001}},
		"first = 101":    {FeatureIds: []int64{1001}, First: ptr(101)},
		"inverted range": {FeatureIds: []int64{1001}, TimeRange: inverted},
	}
	for name, req := range byFeature {
		g.mesh.reset(0)
		_, err := c.GetTransfersByFeatureId(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("by feature, %s: got %v, want InvalidArgument", name, err)
		}
		if calls := g.mesh.reset(0); len(calls) != 0 {
			t.Errorf("by feature, %s: invalid request reached the mesh", name)
		}
	}

	byID := map[string]*publictransfersv1.GetTransfersByIdRequest{
		"no IDs":        {},
		"101 IDs":       {TransferIds: tooMany},
		"duplicate IDs": {TransferIds: []string{testutil.TransferID(1), testutil.TransferID(1)}},
	}
	for name, req := range byID {
		g.mesh.reset(0)
		_, err := c.GetTransfersById(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("by ID, %s: got %v, want InvalidArgument", name, err)
		}
		if calls := g.mesh.reset(0); len(calls) != 0 {
			t.Errorf("by ID, %s: invalid request reached the mesh", name)
		}
	}
}

// checkFullMapping maps a fully populated mesh message and checks every field
// arrives, so a field added to the mesh and missed by a mapper fails.
func checkFullMapping[M, P proto.Message](t *testing.T, mesh M, mapper func(M) P) {
	t.Helper()
	got := mapper(full(mesh))
	if want := asJSON(t, mesh); !reflect.DeepEqual(asJSON(t, got), want) {
		t.Errorf("mapped %s differs:\n got  %v\n want %v", mesh.ProtoReflect().Descriptor().FullName(), asJSON(t, got), want)
	}
	// Every public field must be set too, so a public field with no mesh
	// source shows up.
	unset := []string{}
	var walk func(m protoreflect.Message, path string)
	walk = func(m protoreflect.Message, path string) {
		fields := m.Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			fd := fields.Get(i)
			if !m.Has(fd) {
				unset = append(unset, path+string(fd.Name()))
				continue
			}
			if fd.Message() != nil && !fd.IsMap() && !fd.IsList() && !strings.HasPrefix(string(fd.Message().FullName()), "google.") {
				walk(m.Get(fd).Message(), path+string(fd.Name())+".")
			}
		}
	}
	walk(got.ProtoReflect(), "")
	if len(unset) > 0 {
		t.Errorf("mapped %s leaves fields unset: %v", got.ProtoReflect().Descriptor().FullName(), unset)
	}
}

func TestMapTransfer(t *testing.T) {
	checkFullMapping(t, &meshtransfersv1.RealPropertyTransfer{}, mapTransfer)
	checkFullMapping(t, &meshtransfersv1.RealPropertyTransferEdge{}, mapTransferEdge)
	checkFullMapping(t, &meshpaginationv1.PageInfo{}, mapPageInfo)

	if mapPageInfo(nil) != nil {
		t.Error("nil page info mapped to non-nil")
	}
	if got := mapSlice(([]*meshtransfersv1.TransferShare)(nil), mapTransferShare); got != nil {
		t.Errorf("nil slice mapped to %v", got)
	}
	if got := mapSlice([]*meshtransfersv1.TransferShare{nil, {PartyId: "p"}}, mapTransferShare); len(got) != 1 || got[0].PartyId != "p" {
		t.Errorf("nil element not skipped: %v", got)
	}
	// An untyped code keeps its type unset.
	if got := mapTransferCode(&meshtransfersv1.TransferCode{CodeId: "c"}); got.Type != nil {
		t.Errorf("untyped code got type %v", got.Type)
	}
}
