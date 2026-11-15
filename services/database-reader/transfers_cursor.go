package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/interval"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	transfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
)

// Page sizes for the paginated transfer reads. The public API enforces the
// same limits with protovalidate; the reader clamps as a second line of
// defence, since the mesh has no validation.
const (
	defaultTransferPageSize = 25
	maxTransferPageSize     = 100
)

// transferCursorVersion is the only cursor format this reader accepts.
const transferCursorVersion = 1

// transferPageSize turns the request's optional first into a page size.
func transferPageSize(first *int32) int {
	if first == nil {
		return defaultTransferPageSize
	}
	return min(max(int(*first), 1), maxTransferPageSize)
}

// transferKey is a transfer's position in the sort order
// (transfer_time DESC, transfer_id DESC).
type transferKey struct {
	time time.Time
	id   string
}

// transferFilterHash identifies a request's filter: the RPC, the sorted,
// de-duplicated IDs and the time range. A cursor carries it so it can't be
// reused with a different filter. The page size isn't part of it.
func transferFilterHash(rpc string, ids []string, timeRange *interval.Interval) []byte {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	bound := func(ts *timestamppb.Timestamp) string {
		if ts == nil {
			return ""
		}
		return ts.AsTime().UTC().Format(time.RFC3339Nano)
	}
	// Every field is length-prefixed, so no choice of IDs can make two
	// different filters hash the same input.
	h := sha256.New()
	write := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	write(rpc)
	write(strconv.Itoa(len(sorted)))
	for _, id := range sorted {
		write(id)
	}
	write(bound(timeRange.GetStartTime()))
	write(bound(timeRange.GetEndTime()))
	return h.Sum(nil)
}

// encodeTransferCursor returns the opaque cursor for a transfer.
func encodeTransferCursor(key transferKey, filterHash []byte) (string, error) {
	b, err := proto.Marshal(&transfersv1.TransferCursor{
		Version:      transferCursorVersion,
		TransferTime: timestamppb.New(key.time),
		TransferId:   key.id,
		FilterHash:   filterHash,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var errInvalidCursor = errors.New("invalid cursor: request a new first page")

// decodeTransferCursor checks a cursor against the request's filter and
// returns its sort key. Every failure is InvalidArgument with the same
// message, so the cursor format isn't revealed.
func decodeTransferCursor(s string, filterHash []byte) (transferKey, error) {
	invalid := connect.NewError(connect.CodeInvalidArgument, errInvalidCursor)
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return transferKey{}, invalid
	}
	var c transfersv1.TransferCursor
	if err := proto.Unmarshal(b, &c); err != nil {
		return transferKey{}, invalid
	}
	if c.GetVersion() != transferCursorVersion ||
		!bytes.Equal(c.GetFilterHash(), filterHash) ||
		c.GetTransferTime() == nil || c.GetTransferId() == "" {
		return transferKey{}, invalid
	}
	if err := c.GetTransferTime().CheckValid(); err != nil {
		return transferKey{}, invalid
	}
	return transferKey{time: c.GetTransferTime().AsTime(), id: c.GetTransferId()}, nil
}
