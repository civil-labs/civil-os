package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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
	// last_id is cast to uuid in SQL, so a crafted cursor must not reach
	// the database with anything else.
	if c.GetVersion() != idCursorVersion || !bytes.Equal(c.GetFilterHash(), filterHash) {
		return "", invalid
	}
	if _, err := uuid.Parse(c.GetLastId()); err != nil {
		return "", invalid
	}
	return c.GetLastId(), nil
}

// pageByID reads one page of a lookup ordered by public ID, ascending. read
// returns up to limit nodes whose public ID is after afterID (every node
// when afterID is nil), in order; idOf gives a node's public ID. It returns
// the nodes with their cursors.
func pageByID[T any](
	rpc, id string,
	legalAsOf *timestamppb.Timestamp,
	first *int32,
	after *string,
	read func(afterID *string, limit int) ([]T, error),
	idOf func(T) string,
) ([]T, []string, *paginationv1.PageInfo, error) {
	pageInfo := &paginationv1.PageInfo{HasPreviousPage: after != nil}
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the ID must be a UUID"))
	}

	hash := idFilterHash(rpc, id, legalAsOf)
	var afterID *string
	if after != nil {
		last, err := decodeIDCursor(*after, hash)
		if err != nil {
			return nil, nil, nil, err
		}
		afterID = &last
	}

	size := pageSize(first)
	nodes, err := read(afterID, size+1)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(nodes) > size {
		pageInfo.HasNextPage = true
		nodes = nodes[:size]
	}

	cursors := make([]string, len(nodes))
	for i, n := range nodes {
		c, err := encodeIDCursor(idOf(n), hash)
		if err != nil {
			return nil, nil, nil, connect.NewError(connect.CodeInternal, errors.New("cursor encoding error"))
		}
		cursors[i] = c
	}
	if len(cursors) > 0 {
		pageInfo.StartCursor = &cursors[0]
		pageInfo.EndCursor = &cursors[len(cursors)-1]
	}
	return nodes, cursors, pageInfo, nil
}
