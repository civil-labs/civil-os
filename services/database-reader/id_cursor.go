package main

import (
	"bytes"
	"encoding/base64"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
)

// Cursors for lookups ordered by one public ID, ascending: the address and
// party reverse lookups (docs/address-party-read-api/spec.md, Pagination).

// idCursorVersion is the only ID cursor format this reader accepts.
const idCursorVersion = 1

// idFilterHash identifies a reverse lookup's filter: the RPC, the requested
// ID and legal_as_of (whether it is set, and its value). The page size isn't
// part of it.
func idFilterHash(rpc, id string, legalAsOf *timestamppb.Timestamp) []byte {
	f := newFilterHasher()
	f.write(rpc)
	f.write(id)
	if legalAsOf == nil {
		f.write("unset")
	} else {
		f.write("set")
		f.write(legalAsOf.AsTime().UTC().Format(time.RFC3339Nano))
	}
	return f.sum()
}

// encodeIDCursor returns the opaque cursor for the edge whose public ID is lastID.
func encodeIDCursor(lastID string, filterHash []byte) (string, error) {
	b, err := proto.Marshal(&paginationv1.IdCursor{
		Version:    idCursorVersion,
		LastId:     lastID,
		FilterHash: filterHash,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// decodeIDCursor checks a cursor against the request's filter and returns
// the last public ID it covers. Every failure is InvalidArgument with the
// same message as the transfer cursors, so the format isn't revealed.
func decodeIDCursor(s string, filterHash []byte) (string, error) {
	invalid := connect.NewError(connect.CodeInvalidArgument, errInvalidCursor)
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", invalid
	}
	var c paginationv1.IdCursor
	if err := proto.Unmarshal(b, &c); err != nil {
		return "", invalid
	}
	if c.GetVersion() != idCursorVersion || c.GetLastId() == "" ||
		!bytes.Equal(c.GetFilterHash(), filterHash) {
		return "", invalid
	}
	return c.GetLastId(), nil
}
