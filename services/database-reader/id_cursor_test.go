package main

import (
	"encoding/base64"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
)

func TestIDCursor(t *testing.T) {
	const (
		party  = "30000000-0000-4000-8000-000000000003"
		other  = "30000000-0000-4000-8000-000000000004"
		parcel = "a0000000-0000-4000-8000-000000000002"
	)
	at := timestamppb.New(time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC))
	hash := idFilterHash("GetParcelsByPartyId", party, at)

	c, err := encodeIDCursor(parcel, hash)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := decodeIDCursor(c, hash); err != nil || got != parcel {
		t.Fatalf("round trip = %q, %v", got, err)
	}

	encode := func(m *paginationv1.IdCursor) string {
		b, err := proto.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	later := timestamppb.New(time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC))
	for name, tc := range map[string]struct {
		cursor string
		hash   []byte
	}{
		"malformed base64":   {"not base64!", hash},
		"malformed protobuf": {base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xff}), hash},
		"version 2":          {encode(&paginationv1.IdCursor{Version: 2, LastId: parcel, FilterHash: hash}), hash},
		"no last ID":         {encode(&paginationv1.IdCursor{Version: 1, FilterHash: hash}), hash},
		"last ID not a UUID": {encode(&paginationv1.IdCursor{Version: 1, LastId: "x' OR 1=1", FilterHash: hash}), hash},
		"different RPC":      {c, idFilterHash("GetParcelsByAddressId", party, at)},
		"different ID":       {c, idFilterHash("GetParcelsByPartyId", other, at)},
		"different time":     {c, idFilterHash("GetParcelsByPartyId", party, later)},
		"time unset":         {c, idFilterHash("GetParcelsByPartyId", party, nil)},
	} {
		if _, err := decodeIDCursor(tc.cursor, tc.hash); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
}

func TestIDFilterHashUnsetDiffersFromEpoch(t *testing.T) {
	// legal_as_of unset ("now") must not hash like any explicit time.
	unset := idFilterHash("GetPartiesByAddressId", "a", nil)
	epoch := idFilterHash("GetPartiesByAddressId", "a", timestamppb.New(time.Unix(0, 0)))
	if string(unset) == string(epoch) {
		t.Error("unset legal_as_of hashes like the epoch")
	}
}
