//go:build integration

package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/interval"
	"google.golang.org/protobuf/types/known/timestamppb"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	transfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// Tests for parcel extents and transfers by party
// (docs/address-party-read-api/spec.md, "Tests").

func extents(t *testing.T, r readerClients, when *timestamppb.Timestamp, ns ...int) map[string]*parcelsv1.ExtentWGS84 {
	t.Helper()
	res, err := r.parcels.GetParcelExtentsWGS84ByParcelId(context.Background(), connect.NewRequest(&parcelsv1.GetParcelExtentsWGS84ByParcelIdRequest{
		ParcelIds: ids(tu.ParcelPrefix, ns...), LegalAsOf: when,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Extents
}

// fixtureSquare is parcel n's square in the fixture grid: 4 columns of
// 0.0004 degrees from (-140, -40).
func fixtureSquare(n int) [4]float64 {
	x := -140.0 + float64((n-1)%4)*0.0004
	y := -40.0 + float64((n-1)/4)*0.0004
	return [4]float64{x, y, x + 0.0004, y + 0.0004}
}

func checkBox(t *testing.T, what string, e *parcelsv1.ExtentWGS84, want [4]float64) {
	t.Helper()
	got := [4]float64{e.GetMinX(), e.GetMinY(), e.GetMaxX(), e.GetMaxY()}
	for i := range got {
		if !near(got[i], want[i]) {
			t.Errorf("%s = %v, want %v", what, got, want)
			return
		}
	}
}

func TestParcelExtents(t *testing.T) {
	r := startReader(t)

	all := make([]int, tu.ParcelCount)
	for i := range all {
		all[i] = i + 1
	}
	got := extents(t, r, nil, all...)
	if len(got) != tu.ActiveParcelCount-1 {
		t.Errorf("got %d extents, want %d", len(got), tu.ActiveParcelCount-1)
	}
	for _, n := range []int{20, tu.NoGeometryParcel} { // voided, no geometry
		if got[tu.ParcelID(n)] != nil {
			t.Errorf("parcel %d has an extent", n)
		}
	}
	checkBox(t, "parcel 1", got[tu.ParcelID(1)], fixtureSquare(1))
	checkBox(t, "parcel 9 now", got[tu.ParcelID(tu.RedrawnParcel)], fixtureSquare(tu.RedrawnParcel))

	west := fixtureSquare(tu.RedrawnParcel)
	west[2] = west[0] + 0.0002
	checkBox(t, "parcel 9 at 2022", extents(t, r, at(beforeHistory), tu.RedrawnParcel)[tu.ParcelID(tu.RedrawnParcel)], west)

	// Every box contains the parcel's current geometry.
	pool := tu.FixtureDatabase(t).Pool(t)
	for id, e := range got {
		var ok bool
		err := pool.QueryRow(context.Background(), `
			SELECT extensions.ST_Contains(extensions.ST_MakeEnvelope($2, $3, $4, $5, 4326), g.geom_web)
			FROM geo.parcel_geometry g JOIN core.parcels p USING (parcel_id)
			WHERE p.public_id = $1 AND g.legal_valid_range @> now()`,
			id, e.GetMinX(), e.GetMinY(), e.GetMaxX(), e.GetMaxY()).Scan(&ok)
		if err != nil || !ok {
			t.Errorf("parcel %s: box %v doesn't contain its geometry (%v)", id, e, err)
		}
	}
}

func byParty(t *testing.T, r readerClients, req *transfersv1.GetTransfersByPartyIdRequest) *transfersv1.GetTransfersByPartyIdResponse {
	t.Helper()
	res, err := r.transfers.GetTransfersByPartyId(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg
}

func TestTransfersByParty(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()

	p1 := byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{partyID(1)}})
	if want := ids(tu.TransferPrefix, 5, 2); !slices.Equal(edgeTransferIDs(p1.Edges), want) {
		t.Fatalf("party 1 = %v, want transfers 5 then 2", edgeTransferIDs(p1.Edges))
	}
	// Transfer 5's parcel 20 entry is voided, so party 10 is only on transfer 3.
	if got := edgeTransferIDs(byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{partyID(10)}}).Edges); !slices.Equal(got, ids(tu.TransferPrefix, 3)) {
		t.Errorf("party 10 = %v, want transfer 3 only", got)
	}
	if got := byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{partyID(tu.VoidedParty)}}).Edges; len(got) != 0 {
		t.Errorf("voided party 11 = %v, want nothing", edgeTransferIDs(got))
	}

	// The transfer is built as the other lookups build it.
	if got, want := p1.Edges[1].GetNode(), oneTransfer(t, r, 2); got.String() != want.String() {
		t.Errorf("transfer 2 by party differs from by ID:\n %v\n %v", got, want)
	}

	// A time range starting after transfer 2 leaves only transfer 5.
	after2 := p1.Edges[1].GetNode().GetTransferTime().AsTime().Add(time.Second)
	ranged := byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{
		PartyIds:  []string{partyID(1)},
		TimeRange: &interval.Interval{StartTime: timestamppb.New(after2)},
	})
	if got := edgeTransferIDs(ranged.Edges); !slices.Equal(got, ids(tu.TransferPrefix, 5)) {
		t.Errorf("party 1 after transfer 2 = %v, want transfer 5", got)
	}

	// Pagination.
	one := int32(1)
	page1 := byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{partyID(1)}, First: &one})
	if pi := page1.GetPageInfo(); len(page1.Edges) != 1 || !pi.GetHasNextPage() || pi.GetHasPreviousPage() {
		t.Fatalf("page 1 = %v", page1)
	}
	page2 := byParty(t, r, &transfersv1.GetTransfersByPartyIdRequest{PartyIds: []string{partyID(1)}, First: &one, After: page1.GetPageInfo().EndCursor})
	if pi := page2.GetPageInfo(); !slices.Equal(edgeTransferIDs(page2.Edges), ids(tu.TransferPrefix, 2)) || pi.GetHasNextPage() || !pi.GetHasPreviousPage() {
		t.Errorf("page 2 = %v", page2)
	}

	// A cursor from the parcel lookup fails here, and the other way round.
	byParcelPage := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.ParcelID(5)}, First: &one})
	if byParcelPage.GetPageInfo().EndCursor == nil {
		t.Fatal("parcel 5 has no transfers")
	}
	_, err := r.transfers.GetTransfersByPartyId(ctx, connect.NewRequest(&transfersv1.GetTransfersByPartyIdRequest{
		PartyIds: []string{partyID(1)}, After: byParcelPage.GetPageInfo().EndCursor,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("parcel cursor on party lookup: got %v, want InvalidArgument", err)
	}
	_, err = r.transfers.GetTransfersByPartyId(ctx, connect.NewRequest(&transfersv1.GetTransfersByPartyIdRequest{
		PartyIds: []string{partyID(2)}, After: page1.GetPageInfo().EndCursor,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("cursor for another party: got %v, want InvalidArgument", err)
	}
}
