package main

import (
	"encoding/base64"
	"math"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
)

const searchTestParcel = "a0000000-0000-4000-8000-000000000002"

func encodeSearchCursorMsg(t *testing.T, m *paginationv1.SearchCursor) string {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestSearchCursorRoundTrip(t *testing.T) {
	asOf := time.Date(2026, 11, 19, 12, 30, 0, 123456000, time.UTC)
	hash := searchFilterHash("2 fixture", nil)
	// The extremes, and a subnormal float32, must survive the round trip
	// exactly: the keyset compares them as real in SQL.
	for _, sim := range []float32{0, 1, 0.8, math.SmallestNonzeroFloat32} {
		key := searchKey{tier: 2, similarity: sim, parcelID: searchTestParcel, asOf: asOf}
		c, err := encodeSearchCursor(key, hash)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeSearchCursor(c, hash, nil)
		if err != nil {
			t.Fatalf("similarity %g: %v", sim, err)
		}
		if got.tier != key.tier || got.similarity != key.similarity || got.parcelID != key.parcelID || !got.asOf.Equal(key.asOf) {
			t.Errorf("round trip = %+v, want %+v", got, key)
		}
	}
}

func TestSearchCursorRejects(t *testing.T) {
	asOf := timestamppb.New(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	setAt := timestamppb.New(time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC))
	hash := searchFilterHash("2 fixture", nil)
	hashSet := searchFilterHash("2 fixture", setAt)

	// valid returns a cursor that passes every check; each case changes one
	// field and keeps the correctly recomputed hash, as a forger would.
	valid := func() *paginationv1.SearchCursor {
		return &paginationv1.SearchCursor{
			Version: 1, Tier: 1, Similarity: 0.8, LastParcelId: searchTestParcel,
			FilterHash: hash, AsOf: asOf,
		}
	}
	with := func(f func(*paginationv1.SearchCursor)) string {
		m := valid()
		f(m)
		return encodeSearchCursorMsg(t, m)
	}
	good := encodeSearchCursorMsg(t, valid())

	for name, tc := range map[string]struct {
		cursor    string
		hash      []byte
		legalAsOf *timestamppb.Timestamp
	}{
		"malformed base64":     {"not base64!", hash, nil},
		"malformed protobuf":   {base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xff}), hash, nil},
		"version 2":            {with(func(m *paginationv1.SearchCursor) { m.Version = 2 }), hash, nil},
		"tier 4":               {with(func(m *paginationv1.SearchCursor) { m.Tier = 4 }), hash, nil},
		"similarity NaN":       {with(func(m *paginationv1.SearchCursor) { m.Similarity = float32(math.NaN()) }), hash, nil},
		"similarity +Inf":      {with(func(m *paginationv1.SearchCursor) { m.Similarity = float32(math.Inf(1)) }), hash, nil},
		"similarity -0.1":      {with(func(m *paginationv1.SearchCursor) { m.Similarity = -0.1 }), hash, nil},
		"similarity 1.1":       {with(func(m *paginationv1.SearchCursor) { m.Similarity = 1.1 }), hash, nil},
		"parcel ID not a UUID": {with(func(m *paginationv1.SearchCursor) { m.LastParcelId = "x' OR 1=1" }), hash, nil},
		"no parcel ID":         {with(func(m *paginationv1.SearchCursor) { m.LastParcelId = "" }), hash, nil},
		"as_of unset":          {with(func(m *paginationv1.SearchCursor) { m.AsOf = nil }), hash, nil},
		"as_of before year 0":  {with(func(m *paginationv1.SearchCursor) { m.AsOf = &timestamppb.Timestamp{Seconds: minLegalTimeSeconds - 1} }), hash, nil},
		"as_of invalid nanos":  {with(func(m *paginationv1.SearchCursor) { m.AsOf = &timestamppb.Timestamp{Seconds: 0, Nanos: -1} }), hash, nil},
		"as_of differs from a set legal_as_of": {
			with(func(m *paginationv1.SearchCursor) { m.FilterHash = hashSet }), hashSet, setAt,
		},
		"another query":       {good, searchFilterHash("12 fixture", nil), nil},
		"legal_as_of now set": {good, hashSet, setAt},
	} {
		if _, err := decodeSearchCursor(tc.cursor, tc.hash, tc.legalAsOf); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}

	// A forged cursor with valid values is accepted: it can only ask for a
	// position a client could reach by paging.
	if _, err := decodeSearchCursor(good, hash, nil); err != nil {
		t.Errorf("valid forged cursor: %v", err)
	}
	matching := with(func(m *paginationv1.SearchCursor) { m.FilterHash = hashSet; m.AsOf = setAt })
	if _, err := decodeSearchCursor(matching, hashSet, setAt); err != nil {
		t.Errorf("cursor whose as_of equals legal_as_of: %v", err)
	}
}

func TestSearchFilterHash(t *testing.T) {
	at := timestamppb.New(time.Unix(0, 0))
	if string(searchFilterHash("q", nil)) == string(searchFilterHash("q", at)) {
		t.Error("unset legal_as_of hashes like the epoch")
	}
	if string(searchFilterHash("12 fixture way", nil)) == string(searchFilterHash("12 fixture", nil)) {
		t.Error("different queries hash the same")
	}
}
