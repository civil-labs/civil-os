package main

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// Page sizes for every paginated read. The public API enforces the same
// limits with protovalidate; the reader clamps as a second line of defence,
// since the mesh has no validation.
const (
	defaultPageSize = 25
	maxPageSize     = 100
)

// pageSize turns a request's optional first into a page size.
func pageSize(first *int32) int {
	if first == nil {
		return defaultPageSize
	}
	return min(max(int(*first), 1), maxPageSize)
}

// filterHasher builds a cursor's filter hash. Every field is
// length-prefixed, so no choice of values can make two different filters
// hash the same input.
type filterHasher struct{ h hash.Hash }

func newFilterHasher() filterHasher { return filterHasher{sha256.New()} }

func (f filterHasher) write(s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	f.h.Write(n[:])
	f.h.Write([]byte(s))
}

func (f filterHasher) sum() []byte { return f.h.Sum(nil) }
