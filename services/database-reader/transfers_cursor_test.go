package main

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/interval"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	transfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
)

func TestTransferPageSize(t *testing.T) {
	ptr := func(v int32) *int32 { return &v }
	for _, c := range []struct {
		first *int32
		want  int
	}{
		{nil, 25}, {ptr(1), 1}, {ptr(50), 50}, {ptr(100), 100},
		{ptr(500), 100}, {ptr(0), 1}, {ptr(-3), 1},
	} {
		if got := transferPageSize(c.first); got != c.want {
			t.Errorf("transferPageSize(%v) = %d, want %d", c.first, got, c.want)
		}
	}
}

func TestTransferFilterHash(t *testing.T) {
	jan := &interval.Interval{StartTime: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))}
	base := transferFilterHash("GetTransfersByParcelId", []string{"a", "b"}, jan)

	if got := transferFilterHash("GetTransfersByParcelId", []string{"b", "a", "a"}, jan); !bytes.Equal(got, base) {
		t.Error("ID order or duplicates changed the hash")
	}
	for name, other := range map[string][]byte{
		"different RPC":   transferFilterHash("GetTransfersByFeatureId", []string{"a", "b"}, jan),
		"different IDs":   transferFilterHash("GetTransfersByParcelId", []string{"a"}, jan),
		"no range":        transferFilterHash("GetTransfersByParcelId", []string{"a", "b"}, nil),
		"end, not start":  transferFilterHash("GetTransfersByParcelId", []string{"a", "b"}, &interval.Interval{EndTime: jan.StartTime}),
		"joined ID split": transferFilterHash("GetTransfersByParcelId", []string{"a,b"}, jan),
	} {
		if bytes.Equal(other, base) {
			t.Errorf("%s: same hash", name)
		}
	}
}

func TestTransferCursorRoundTrip(t *testing.T) {
	hash := transferFilterHash("GetTransfersByParcelId", []string{"a"}, nil)
	key := transferKey{time: time.Date(2023, 9, 1, 12, 30, 0, 123, time.UTC), id: "90000000-0000-4000-8000-000000000003"}

	s, err := encodeTransferCursor(key, hash)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTransferCursor(s, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !got.time.Equal(key.time) || got.id != key.id {
		t.Errorf("round trip = %+v, want %+v", got, key)
	}
}

func TestTransferCursorRejected(t *testing.T) {
	hash := transferFilterHash("GetTransfersByParcelId", []string{"a"}, nil)
	encode := func(c *transfersv1.TransferCursor) string {
		b, err := proto.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	valid := func() *transfersv1.TransferCursor {
		return &transfersv1.TransferCursor{
			Version:      transferCursorVersion,
			TransferTime: timestamppb.New(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)),
			TransferId:   "x",
			FilterHash:   hash,
		}
	}
	other := transferFilterHash("GetTransfersByParcelId", []string{"b"}, nil)

	cases := map[string]string{
		"not base64":     "%%%",
		"padded base64":  base64.URLEncoding.EncodeToString([]byte("ab")),
		"not a protobuf": base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xff, 0xff}),
		"empty":          "",
		"version 2":      encode(func() *transfersv1.TransferCursor { c := valid(); c.Version = 2; return c }()),
		"other filter":   encode(func() *transfersv1.TransferCursor { c := valid(); c.FilterHash = other; return c }()),
		"no time":        encode(func() *transfersv1.TransferCursor { c := valid(); c.TransferTime = nil; return c }()),
		"no transfer ID": encode(func() *transfersv1.TransferCursor { c := valid(); c.TransferId = ""; return c }()),
		"invalid time":   encode(func() *transfersv1.TransferCursor { c := valid(); c.TransferTime.Nanos = -1; return c }()),
	}
	for name, s := range cases {
		if _, err := decodeTransferCursor(s, hash); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
	if _, err := decodeTransferCursor(encode(valid()), hash); err != nil {
		t.Errorf("valid cursor rejected: %v", err)
	}
}
