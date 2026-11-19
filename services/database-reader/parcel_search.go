package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
)

// Address search (docs/parcel-search/spec.md).
const (
	// searchSimilarityThreshold is pg_trgm's word_similarity_threshold for
	// the fuzzy tier. It changes results, so it's a reviewed constant, not
	// configuration. 0.6 was checked against the fixtures
	// (docs/parcel-search/plan.md, step 2.5).
	searchSimilarityThreshold = "0.6"
	// defaultSearchCap is the most hits a query returns across all pages.
	defaultSearchCap = 1000
	// minSearchQueryRunes is the fewest letters and digits a normalised
	// query may have: shorter ones can't use the trigram index selectively.
	minSearchQueryRunes = 3
	// maxSearchQueryRunes repeats the gateway's limit on the raw query,
	// since the mesh has no validation.
	maxSearchQueryRunes = 200
)

var errSearchQueryTooShort = errors.New("query must contain at least 3 letters or digits")

// searchSQL ranks every visible parcel whose current address matches the
// normalised query $2, caps the ranked list at $5 hits, and returns the page
// after the keyset position ($6, $7, $8), or the first page when $6 is null.
//
//	$1 the legal time t
//	$2 the normalised query
//	$3 the normalised query, escaped for LIKE
//	$4 the page size plus one
//	$5 the cap
//	$6 $7 $8 the cursor's tier, similarity and parcel public_id
//
// Visibility (spec, rules 1 and 2): the parcel isn't voided and has geometry
// and attributes at t; the address is the one those attributes name, isn't
// voided, and has attributes at t. Both trigram conditions are on the
// indexed expression, so the GIN index drives the candidate scan.
const searchSQL = `
WITH cand AS (
	SELECT p.public_id AS parcel_id, pg.feature_id,
	       a.public_id AS address_id, aa.formatted_address,
	       core.search_normalize(aa.formatted_address) AS t
	FROM core.address_attributes aa
	JOIN core.addresses a ON a.address_id = aa.address_id AND NOT a.is_voided
	JOIN core.parcel_attributes pva ON pva.address_id = a.address_id AND pva.legal_valid_range @> $1::timestamptz
	JOIN core.parcels p ON p.parcel_id = pva.parcel_id AND NOT p.is_voided
	JOIN geo.parcel_geometry pg ON pg.parcel_id = p.parcel_id AND pg.legal_valid_range @> $1::timestamptz
	WHERE aa.legal_valid_range @> $1::timestamptz
	  AND ($2::text <% core.search_normalize(aa.formatted_address)
	       OR core.search_normalize(aa.formatted_address) LIKE '%' || $3::text || '%')
), ranked AS (
	SELECT parcel_id, feature_id, address_id, formatted_address,
	       CASE WHEN t LIKE $3::text || '%'               THEN 0
	            WHEN ' ' || t LIKE '% ' || $3::text || '%' THEN 1
	            WHEN t LIKE '%' || $3::text || '%'         THEN 2
	            ELSE 3 END AS tier,
	       word_similarity($2::text, t) AS sim
	FROM cand
	ORDER BY tier, sim DESC, parcel_id
	LIMIT $5
)
SELECT parcel_id::text, feature_id, address_id::text, formatted_address, tier, sim
FROM ranked
WHERE $6::int IS NULL
   OR (tier, -sim, parcel_id) > ($6::int, -($7::real), $8::uuid)
ORDER BY tier, sim DESC, parcel_id
LIMIT $4`

// escapeLike escapes LIKE's wildcards and its escape character.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *APIServer) SearchParcelsAddresses(
	ctx context.Context,
	req *connect.Request[parcelsv1.SearchParcelsAddressesRequest],
) (*connect.Response[parcelsv1.SearchParcelsAddressesResponse], error) {
	msg := req.Msg
	s.logger.Debug("received SearchParcelsAddresses request",
		slog.Int("query_length", utf8.RuneCountInString(msg.Query)),
		slog.Bool("after", msg.After != nil))

	if utf8.RuneCountInString(msg.Query) > maxSearchQueryRunes {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("query must be at most 200 characters"))
	}
	// Postgres text can't hold NUL. It isn't a letter or digit, so a query
	// is judged without it.
	raw := strings.ReplaceAll(msg.Query, "\x00", " ")

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, s.dbError("parcel search", err)
	}
	defer tx.Rollback(ctx)

	// Normalise in SQL, so the query and the index see the same function.
	var q string
	if err := tx.QueryRow(ctx, "SELECT core.search_normalize($1::text)", raw).Scan(&q); err != nil {
		return nil, s.dbError("parcel search", err)
	}
	if utf8.RuneCountInString(strings.ReplaceAll(q, " ", "")) < minSearchQueryRunes {
		return nil, connect.NewError(connect.CodeInvalidArgument, errSearchQueryTooShort)
	}

	hash := searchFilterHash(q, msg.LegalAsOf)
	var after *searchKey
	if msg.After != nil {
		k, err := decodeSearchCursor(*msg.After, hash, msg.LegalAsOf)
		if err != nil {
			return nil, err
		}
		after = &k
	}

	// t is legal_as_of, or the snapshot the first page took. Postgres keeps
	// microseconds, so the snapshot is truncated to match what it compares.
	var at time.Time
	switch {
	case msg.LegalAsOf != nil:
		at = msg.LegalAsOf.AsTime()
	case after != nil:
		at = after.asOf
	default:
		at = time.Now().UTC().Truncate(time.Microsecond)
	}

	if _, err := tx.Exec(ctx, "SELECT set_config('pg_trgm.word_similarity_threshold', $1, true)", searchSimilarityThreshold); err != nil {
		return nil, s.dbError("parcel search", err)
	}

	var afterTier *int
	var afterSim *float32
	var afterParcel *string
	if after != nil {
		afterTier, afterSim, afterParcel = &after.tier, &after.similarity, &after.parcelID
	}
	size := pageSize(msg.First)
	rows, err := tx.Query(ctx, searchSQL, at, q, escapeLike(q), size+1, s.searchCap, afterTier, afterSim, afterParcel)
	if err != nil {
		return nil, s.dbError("parcel search", err)
	}
	type hit struct {
		*parcelsv1.ParcelSearchHit
		key searchKey
	}
	hits, err := collectRows(rows, func(r pgx.Rows) (hit, error) {
		h := hit{ParcelSearchHit: &parcelsv1.ParcelSearchHit{}, key: searchKey{asOf: at}}
		err := r.Scan(&h.ParcelId, &h.FeatureId, &h.AddressId, &h.FormattedAddress, &h.key.tier, &h.key.similarity)
		h.key.parcelID = h.ParcelId
		return h, err
	}, func(err error) error { return s.dbError("parcel search", err) })
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, s.dbError("parcel search", err)
	}

	pageInfo := &paginationv1.PageInfo{HasPreviousPage: after != nil}
	if len(hits) > size {
		pageInfo.HasNextPage = true
		hits = hits[:size]
	}
	res := &parcelsv1.SearchParcelsAddressesResponse{
		Hits:     make([]*parcelsv1.ParcelSearchHit, len(hits)),
		PageInfo: pageInfo,
	}
	for i, h := range hits {
		res.Hits[i] = h.ParcelSearchHit
	}
	if len(hits) > 0 {
		start, err := encodeSearchCursor(hits[0].key, hash)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("cursor encoding error"))
		}
		end, err := encodeSearchCursor(hits[len(hits)-1].key, hash)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("cursor encoding error"))
		}
		pageInfo.StartCursor, pageInfo.EndCursor = &start, &end
	}

	s.logger.Debug("SearchParcelsAddresses returned", slog.Int("hits", len(res.Hits)))
	return connect.NewResponse(res), nil
}
