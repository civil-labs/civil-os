package main

import (
	"bytes"
	"encoding/base64"
	"math"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
)

// Cursors for SearchParcelsAddresses (docs/parcel-search/spec.md,
// Pagination). Unlike the other cursors, a search cursor carries values that
// reach SQL as more than an ID, so decoding checks every field: the filter
// hash is a plain hash of public inputs and a client can forge it.

// searchCursorVersion is the only search cursor format this reader accepts.
const searchCursorVersion = 1

// maxSearchTier is the last ranking tier: 3, similar but not containing.
const maxSearchTier = 3

// minLegalTimeSeconds is the earliest legal time the public API accepts
// (legal_as_of must be after it: about 1 January of year 0). A cursor's as_of
// stands in for legal_as_of, so it gets the same bound.
const minLegalTimeSeconds = -62135596800

// searchKey is a hit's position in the ranked order (tier ASC, similarity
// DESC, address, parcel public_id ASC), plus the legal time the walk resolves
// at. The address orders by its internal key, which the reader resolves from
// addressID, so the internal key never leaves the database.
type searchKey struct {
	tier       int
	similarity float32
	addressID  string
	parcelID   string
	asOf       time.Time
}

// searchFilterHash identifies a search: the RPC, the normalised query and
// legal_as_of (whether it is set, and its value). Queries that normalise the
// same share cursors. The page size isn't part of it.
func searchFilterHash(normalizedQuery string, legalAsOf *timestamppb.Timestamp) []byte {
	f := newFilterHasher()
	f.write("SearchParcelsAddresses")
	f.write(normalizedQuery)
	if legalAsOf == nil {
		f.write("unset")
	} else {
		f.write("set")
		f.write(legalAsOf.AsTime().UTC().Format(time.RFC3339Nano))
	}
	return f.sum()
}

// encodeSearchCursor returns the opaque cursor for a hit.
func encodeSearchCursor(key searchKey, filterHash []byte) (string, error) {
	b, err := proto.Marshal(&paginationv1.SearchCursor{
		Version:       searchCursorVersion,
		Tier:          uint32(key.tier),
		Similarity:    key.similarity,
		LastParcelId:  key.parcelID,
		FilterHash:    filterHash,
		AsOf:          timestamppb.New(key.asOf),
		LastAddressId: key.addressID,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// decodeSearchCursor checks a cursor against the request and returns its
// position. legalAsOf is the request's; when it is set, the cursor's as_of
// must equal it. Every failure is InvalidArgument with the shared cursor
// message, so the format isn't revealed.
func decodeSearchCursor(s string, filterHash []byte, legalAsOf *timestamppb.Timestamp) (searchKey, error) {
	invalid := connect.NewError(connect.CodeInvalidArgument, errInvalidCursor)
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return searchKey{}, invalid
	}
	var c paginationv1.SearchCursor
	if err := proto.Unmarshal(b, &c); err != nil {
		return searchKey{}, invalid
	}
	if c.GetVersion() != searchCursorVersion || !bytes.Equal(c.GetFilterHash(), filterHash) {
		return searchKey{}, invalid
	}
	if c.GetTier() > maxSearchTier {
		return searchKey{}, invalid
	}
	sim := c.GetSimilarity()
	if math.IsNaN(float64(sim)) || math.IsInf(float64(sim), 0) || sim < 0 || sim > 1 {
		return searchKey{}, invalid
	}
	if _, err := uuid.Parse(c.GetLastParcelId()); err != nil {
		return searchKey{}, invalid
	}
	if _, err := uuid.Parse(c.GetLastAddressId()); err != nil {
		return searchKey{}, invalid
	}
	asOf := c.GetAsOf()
	if asOf == nil || asOf.CheckValid() != nil || asOf.GetSeconds() <= minLegalTimeSeconds {
		return searchKey{}, invalid
	}
	if legalAsOf != nil && !asOf.AsTime().Equal(legalAsOf.AsTime()) {
		return searchKey{}, invalid
	}
	return searchKey{
		tier:       int(c.GetTier()),
		similarity: sim,
		addressID:  c.GetLastAddressId(),
		parcelID:   c.GetLastParcelId(),
		asOf:       asOf.AsTime(),
	}, nil
}
