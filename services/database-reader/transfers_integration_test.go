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

	transfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

func partyID(n int) string       { return tu.FixtureID(tu.PartyPrefix, n) }
func improvementID(n int) string { return tu.FixtureID(tu.ImprovementPrefix, n) }
func codeID(n int) string        { return tu.FixtureID(tu.TransferCodePrefix, n) }

func transfersByID(t *testing.T, r readerClients, ns ...int) []*transfersv1.RealPropertyTransfer {
	t.Helper()
	res, err := r.transfers.GetTransfersById(context.Background(), connect.NewRequest(&transfersv1.GetTransfersByIdRequest{
		TransferIds: ids(tu.TransferPrefix, ns...),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Transfers
}

func oneTransfer(t *testing.T, r readerClients, n int) *transfersv1.RealPropertyTransfer {
	t.Helper()
	got := transfersByID(t, r, n)
	if len(got) != 1 {
		t.Fatalf("transfer %d: got %d transfers", n, len(got))
	}
	return got[0]
}

func byParcel(t *testing.T, r readerClients, req *transfersv1.GetTransfersByParcelIdRequest) *transfersv1.GetTransfersByParcelIdResponse {
	t.Helper()
	res, err := r.transfers.GetTransfersByParcelId(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg
}

func edgeTransferIDs(edges []*transfersv1.RealPropertyTransferEdge) []string {
	out := make([]string, len(edges))
	for i, e := range edges {
		out[i] = e.GetNode().GetTransferId()
	}
	return out
}

func transferIDs(ts []*transfersv1.RealPropertyTransfer) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.GetTransferId()
	}
	return out
}

func shareParties(shares []*transfersv1.TransferShare) []string {
	out := make([]string, len(shares))
	for i, s := range shares {
		out[i] = s.GetPartyId()
	}
	return out
}

func partyIDs(t *transfersv1.RealPropertyTransfer) []string {
	out := make([]string, len(t.GetParties()))
	for i, p := range t.GetParties() {
		out[i] = p.GetPartyId()
	}
	return out
}

func transferParcelIDs(t *transfersv1.RealPropertyTransfer) []string {
	out := make([]string, len(t.GetParcels()))
	for i, p := range t.GetParcels() {
		out[i] = p.GetParcelId()
	}
	return out
}

// checkSharesListed checks that every share points into the transfer's
// parties, and that parties has nothing else.
func checkSharesListed(t *testing.T, tr *transfersv1.RealPropertyTransfer) {
	t.Helper()
	used := map[string]bool{}
	collect := func(shares []*transfersv1.TransferShare) {
		for _, s := range shares {
			used[s.GetPartyId()] = true
		}
	}
	for _, p := range tr.GetParcels() {
		collect(p.GetGrantors())
		collect(p.GetGrantees())
	}
	for _, imp := range tr.GetImprovements() {
		collect(imp.GetGrantors())
		collect(imp.GetGrantees())
	}
	listed := partyIDs(tr)
	if len(listed) != len(used) {
		t.Errorf("transfer %s: parties %v, but shares use %v", tr.GetTransferId(), listed, sortedKeys(used))
	}
	for id := range used {
		if !slices.Contains(listed, id) {
			t.Errorf("transfer %s: share party %s missing from parties", tr.GetTransferId(), id)
		}
	}
}

func TestTransfersByID(t *testing.T) {
	r := startReader(t)

	all := make([]int, tu.TransferCount)
	for i := range all {
		all[i] = i + 1
	}
	got := transfersByID(t, r, all...)
	// Transfer 4 is never visible; the rest come newest first.
	if want := ids(tu.TransferPrefix, 6, 5, 3, 2, 7, 1); !slices.Equal(transferIDs(got), want) {
		t.Errorf("all transfers: got %v, want %v", transferIDs(got), want)
	}
	for _, tr := range got {
		checkSharesListed(t, tr)
	}

	if got := transfersByID(t, r, tu.VoidedOnlyTransfer); len(got) != 0 {
		t.Errorf("voided-only transfer returned: %v", transferIDs(got))
	}
	res, err := r.transfers.GetTransfersById(context.Background(), connect.NewRequest(&transfersv1.GetTransfersByIdRequest{
		TransferIds: []string{tu.MissingID},
	}))
	if err != nil || len(res.Msg.Transfers) != 0 {
		t.Errorf("unknown ID: got %v, %v", res, err)
	}
}

func TestTransferContents(t *testing.T) {
	r := startReader(t)

	t.Run("basic read, codes and recording", func(t *testing.T) {
		tr := oneTransfer(t, r, 1)
		if got := tr.GetTransferTime().AsTime(); !got.Equal(time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("time = %v", got)
		}
		if tr.TransferAmount == nil || *tr.TransferAmount != "250000.0000" {
			t.Errorf("amount = %v", tr.TransferAmount)
		}
		if len(tr.GetRecordings()) != 1 {
			t.Fatalf("recordings = %v", tr.GetRecordings())
		}
		rec := tr.GetRecordings()[0]
		if rec.GetBook() != "FX-B1" || rec.GetPage() != "1" || rec.GetUri() != "https://recordings.example.invalid/FX-1" {
			t.Errorf("recording = %v", rec)
		}
		if len(tr.GetCodes()) != 2 {
			t.Fatalf("codes = %v", tr.GetCodes())
		}
		typed, untyped := tr.GetCodes()[0], tr.GetCodes()[1]
		if typed.GetCodeId() != codeID(1) || typed.GetName() != "Fixture Qualified" ||
			typed.GetType().GetCodeTypeId() != tu.FixtureID(tu.TransferCodeTypePrefix, 1) || typed.GetType().GetName() != "Fixture Qualification" {
			t.Errorf("code 1 = %v", typed)
		}
		if untyped.GetCodeId() != codeID(3) || untyped.Type != nil || untyped.Description != nil {
			t.Errorf("code 3 = %v", untyped)
		}
		if len(tr.GetParcels()) != 1 || tr.GetParcels()[0].GetFeatureId() != tu.ParcelFeatureID(5) {
			t.Fatalf("parcels = %v", tr.GetParcels())
		}
		p := tr.GetParcels()[0]
		if !slices.Equal(shareParties(p.GetGrantors()), []string{partyID(9)}) || !slices.Equal(shareParties(p.GetGrantees()), []string{partyID(4)}) {
			t.Errorf("parcel 5 shares = %v", p)
		}
		if p.GetGrantees()[0].GetTransferredShare() != "1.0000" {
			t.Errorf("share = %q", p.GetGrantees()[0].GetTransferredShare())
		}
		for _, party := range tr.GetParties() {
			if party.Name == nil || party.MailingAddress == nil {
				t.Errorf("party %s has no name or address as of the transfer: %v", party.GetPartyId(), party)
			}
		}
		if got := tr.GetParties()[1]; got.GetPartyId() != partyID(9) || got.GetName() != "Fixture Owner 9" {
			t.Errorf("party 9 = %v", got)
		}
	})

	t.Run("several grantees", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.MultiGranteeTransfer)
		p := tr.GetParcels()[0]
		if !slices.Equal(shareParties(p.GetGrantees()), []string{partyID(2), partyID(3)}) {
			t.Errorf("grantees = %v", shareParties(p.GetGrantees()))
		}
		for _, s := range p.GetGrantees() {
			if s.GetTransferredShare() != "0.5000" {
				t.Errorf("grantee share = %q", s.GetTransferredShare())
			}
		}
		if want := []string{partyID(1), partyID(2), partyID(3)}; !slices.Equal(partyIDs(tr), want) {
			t.Errorf("parties = %v, want %v", partyIDs(tr), want)
		}
	})

	t.Run("several parcels and improvements", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.MultiParcelTransfer)
		if want := []string{tu.ParcelID(6), tu.ParcelID(7)}; !slices.Equal(transferParcelIDs(tr), want) {
			t.Errorf("parcels = %v", transferParcelIDs(tr))
		}
		if len(tr.GetImprovements()) != 2 || tr.GetImprovements()[0].GetImprovementId() != improvementID(6) || tr.GetImprovements()[1].GetImprovementId() != improvementID(7) {
			t.Errorf("improvements = %v", tr.GetImprovements())
		}
		for _, p := range tr.GetParcels() {
			if !slices.Equal(shareParties(p.GetGrantors()), []string{partyID(10)}) {
				t.Errorf("parcel %s grantors = %v", p.GetParcelId(), shareParties(p.GetGrantors()))
			}
		}
		if want := []string{partyID(10), partyID(6), partyID(7)}; !slices.Equal(partyIDs(tr), slices.Sorted(slices.Values(want))) {
			t.Errorf("parties = %v", partyIDs(tr))
		}
	})

	t.Run("voided parcel entry hidden", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.PartlyVoidedTransfer)
		if !slices.Equal(transferParcelIDs(tr), []string{tu.ParcelID(19)}) {
			t.Errorf("parcels = %v, want only parcel 19", transferParcelIDs(tr))
		}
		// Party 10 was only a grantee on voided parcel 20.
		if want := []string{partyID(1), partyID(2), partyID(9)}; !slices.Equal(partyIDs(tr), want) {
			t.Errorf("parties = %v, want %v", partyIDs(tr), want)
		}
	})

	t.Run("several grantors", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.PartlyVoidedTransfer)
		p := tr.GetParcels()[0]
		if !slices.Equal(shareParties(p.GetGrantors()), []string{partyID(1), partyID(2)}) {
			t.Fatalf("parcel 19 grantors = %v, want parties 1 and 2", shareParties(p.GetGrantors()))
		}
		for _, s := range p.GetGrantors() {
			if s.GetTransferredShare() != "0.5000" {
				t.Errorf("grantor %s share = %q, want 0.5000", s.GetPartyId(), s.GetTransferredShare())
			}
		}
		if !slices.Equal(shareParties(p.GetGrantees()), []string{partyID(9)}) {
			t.Errorf("parcel 19 grantees = %v, want party 9", shareParties(p.GetGrantees()))
		}
	})

	t.Run("no amount and no recording", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.NoRecordingTransfer)
		if tr.TransferAmount != nil {
			t.Errorf("amount = %q, want unset", *tr.TransferAmount)
		}
		if len(tr.GetRecordings()) != 0 {
			t.Errorf("recordings = %v, want none", tr.GetRecordings())
		}
	})

	t.Run("missing grantee", func(t *testing.T) {
		tr := oneTransfer(t, r, tu.MissingGranteeTransfer)
		if len(tr.GetParcels()) != 1 {
			t.Fatalf("parcels = %v", tr.GetParcels())
		}
		p := tr.GetParcels()[0]
		if p.GetParcelId() != tu.ParcelID(8) || len(p.GetGrantors()) != 1 || len(p.GetGrantees()) != 0 {
			t.Errorf("parcel 8 = %v, want one grantor and no grantees", p)
		}
	})
}

func TestTransfersByParcelAndFeature(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()

	// Queried by parcel 6 alone, transfer 3 still lists parcel 7 and both improvements.
	res := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.ParcelID(6)}})
	if len(res.Edges) != 1 {
		t.Fatalf("parcel 6: got %v", edgeTransferIDs(res.Edges))
	}
	if tr := res.Edges[0].GetNode(); len(tr.GetParcels()) != 2 || len(tr.GetImprovements()) != 2 {
		t.Errorf("parcel 6: transfer 3 = %v", tr)
	}

	// A transfer covering several requested parcels appears once.
	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.ParcelID(6), tu.ParcelID(7)}})
	if want := ids(tu.TransferPrefix, 3); !slices.Equal(edgeTransferIDs(res.Edges), want) {
		t.Errorf("parcels 6 and 7: got %v", edgeTransferIDs(res.Edges))
	}

	// Newest first.
	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.ParcelID(5)}})
	if want := ids(tu.TransferPrefix, 6, 1); !slices.Equal(edgeTransferIDs(res.Edges), want) {
		t.Errorf("parcel 5: got %v, want %v", edgeTransferIDs(res.Edges), want)
	}

	// The range is half-open: [2022-01-01, end of transfer 6) excludes both ends' outsiders.
	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{
		ParcelIds: []string{tu.ParcelID(5)},
		TimeRange: &interval.Interval{StartTime: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))},
	})
	if want := ids(tu.TransferPrefix, 6); !slices.Equal(edgeTransferIDs(res.Edges), want) {
		t.Errorf("parcel 5 from 2022: got %v, want %v", edgeTransferIDs(res.Edges), want)
	}
	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{
		ParcelIds: []string{tu.ParcelID(5)},
		TimeRange: &interval.Interval{EndTime: timestamppb.New(time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC))},
	})
	if want := ids(tu.TransferPrefix, 1); !slices.Equal(edgeTransferIDs(res.Edges), want) {
		t.Errorf("parcel 5 before transfer 6's time: got %v, want %v", edgeTransferIDs(res.Edges), want)
	}

	// The voided parcel and its feature return nothing.
	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.ParcelID(tu.VoidedParcel)}})
	if len(res.Edges) != 0 {
		t.Errorf("voided parcel: got %v", edgeTransferIDs(res.Edges))
	}
	feat, err := r.transfers.GetTransfersByFeatureId(ctx, connect.NewRequest(&transfersv1.GetTransfersByFeatureIdRequest{
		FeatureIds: []int64{tu.ParcelFeatureID(tu.VoidedParcel)},
	}))
	if err != nil || len(feat.Msg.Edges) != 0 {
		t.Errorf("voided parcel's feature: got %v, %v", feat, err)
	}

	feat, err = r.transfers.GetTransfersByFeatureId(ctx, connect.NewRequest(&transfersv1.GetTransfersByFeatureIdRequest{
		FeatureIds: []int64{tu.ParcelFeatureID(2)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if want := ids(tu.TransferPrefix, tu.MultiGranteeTransfer); !slices.Equal(edgeTransferIDs(feat.Msg.Edges), want) {
		t.Errorf("feature of parcel 2: got %v", edgeTransferIDs(feat.Msg.Edges))
	}

	res = byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: []string{tu.MissingID}})
	if len(res.Edges) != 0 || res.GetPageInfo().GetHasNextPage() {
		t.Errorf("unknown parcel: got %v", res)
	}
}

func TestTransferPagination(t *testing.T) {
	r := startReader(t)
	ctx := context.Background()
	one := int32(1)
	parcel5 := []string{tu.ParcelID(5)}

	page1 := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel5, First: &one})
	if want := ids(tu.TransferPrefix, 6); !slices.Equal(edgeTransferIDs(page1.Edges), want) {
		t.Fatalf("page 1: got %v", edgeTransferIDs(page1.Edges))
	}
	pi := page1.GetPageInfo()
	if !pi.GetHasNextPage() || pi.GetHasPreviousPage() || pi.GetStartCursor() != page1.Edges[0].GetCursor() || pi.GetEndCursor() != page1.Edges[0].GetCursor() {
		t.Errorf("page 1 info = %v", pi)
	}

	page2 := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel5, First: &one, After: pi.EndCursor})
	if want := ids(tu.TransferPrefix, 1); !slices.Equal(edgeTransferIDs(page2.Edges), want) {
		t.Fatalf("page 2: got %v", edgeTransferIDs(page2.Edges))
	}
	if pi := page2.GetPageInfo(); pi.GetHasNextPage() || !pi.GetHasPreviousPage() {
		t.Errorf("page 2 info = %v", pi)
	}

	page3 := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel5, After: page2.GetPageInfo().EndCursor})
	if pi := page3.GetPageInfo(); len(page3.Edges) != 0 || pi.GetHasNextPage() || pi.StartCursor != nil || pi.EndCursor != nil {
		t.Errorf("past the end: got %v", page3)
	}

	// The reader clamps first itself, since the mesh has no validation.
	big := int32(500)
	if res := byParcel(t, r, &transfersv1.GetTransfersByParcelIdRequest{ParcelIds: parcel5, First: &big}); len(res.Edges) != 2 {
		t.Errorf("first = 500: got %v", edgeTransferIDs(res.Edges))
	}

	bad := map[string]*transfersv1.GetTransfersByParcelIdRequest{
		"cursor for another parcel": {ParcelIds: []string{tu.ParcelID(2)}, After: pi.EndCursor},
		"cursor with a time range": {ParcelIds: parcel5, After: pi.EndCursor, TimeRange: &interval.Interval{
			StartTime: timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
		}},
		"malformed cursor": {ParcelIds: parcel5, After: new(string)},
	}
	*bad["malformed cursor"].After = "not-a-cursor"
	for name, req := range bad {
		if _, err := r.transfers.GetTransfersByParcelId(ctx, connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}

	// A parcel cursor isn't valid for the feature lookup, even for the same parcel.
	_, err := r.transfers.GetTransfersByFeatureId(ctx, connect.NewRequest(&transfersv1.GetTransfersByFeatureIdRequest{
		FeatureIds: []int64{tu.ParcelFeatureID(5)}, After: pi.EndCursor,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("parcel cursor on feature lookup: got %v, want InvalidArgument", err)
	}
}

// TestVoidedPartyHidden voids party 3 and checks rule 3: it disappears from
// shares and parties, but the entries stay, even when left with no parties.
func TestVoidedPartyHidden(t *testing.T) {
	pool := tu.FixtureDatabase(t).Pool(t)
	ctx := context.Background()
	// This test has its own database copy, so nothing needs restoring.
	if _, err := pool.Exec(ctx, `UPDATE core.parties SET is_voided = true, voided_at = now() WHERE public_id = $1`, partyID(3)); err != nil {
		t.Fatal(err)
	}
	r := startReaderOn(t, pool)

	tr := oneTransfer(t, r, tu.MultiGranteeTransfer)
	if got := shareParties(tr.GetParcels()[0].GetGrantees()); !slices.Equal(got, []string{partyID(2)}) {
		t.Errorf("transfer 2 grantees = %v, want only party 2", got)
	}
	if want := []string{partyID(1), partyID(2)}; !slices.Equal(partyIDs(tr), want) {
		t.Errorf("transfer 2 parties = %v, want %v", partyIDs(tr), want)
	}

	// Party 3 was transfer 7's only party: parcel 8 stays, with no shares.
	tr = oneTransfer(t, r, tu.MissingGranteeTransfer)
	if len(tr.GetParcels()) != 1 || len(tr.GetParcels()[0].GetGrantors()) != 0 || len(tr.GetParcels()[0].GetGrantees()) != 0 || len(tr.GetParties()) != 0 {
		t.Errorf("transfer 7 = %v, want parcel 8 with no shares and no parties", tr)
	}
}
