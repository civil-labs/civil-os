package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/genproto/googleapis/type/interval"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	recordingsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/recordings/v1"
	transfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
)

// Real property transfer reads. The contract and the visibility rules are in
// docs/real-property-transfers-api/spec.md:
//
//  1. a parcel entry is hidden when the parcel is voided;
//  2. an improvement entry is hidden when the improvement is voided, or when
//     its current parcel links lead only to voided parcels;
//  3. a voided party is left out of every share list and of parties, but its
//     parcel or improvement entry stays, possibly with an empty list;
//  4. a transfer is returned only if it has a visible parcel or improvement.

// visibleImprovementSQL is rule 2 for the improvement with ID column col.
const visibleImprovementSQL = `EXISTS (
		SELECT 1
		FROM core.improvements vi
		JOIN core.parcel_improvements vpi ON vpi.improvement_id = vi.improvement_id AND vpi.legal_valid_range @> now()
		JOIN core.parcels vp ON vp.parcel_id = vpi.parcel_id
		WHERE vi.improvement_id = %s AND NOT vi.is_voided AND NOT vp.is_voided
	)`

// transferSelectSQL selects the matching transfers. %s is the request's
// filter on rpt, which uses $1. $2 and $3 are the optional time range,
// $4 and $5 the optional keyset position, and $6 the optional row limit.
var transferSelectSQL = `
	SELECT rpt.real_property_transfer_id, rpt.public_id::text, rpt.transfer_timestamp,
	       rpt.transfer_amount::numeric(19,4)::text, rpt.deed_book, rpt.deed_page, rpt.deed_uri
	FROM core.real_property_transfers rpt
	WHERE (%s)
	  AND ($2::timestamptz IS NULL OR rpt.transfer_timestamp >= $2::timestamptz)
	  AND ($3::timestamptz IS NULL OR rpt.transfer_timestamp < $3::timestamptz)
	  AND (
	    EXISTS (
	      SELECT 1
	      FROM core.real_property_transfer_party_parcels x
	      JOIN core.parcels p ON p.parcel_id = x.parcel_id
	      WHERE x.real_property_transfer_id = rpt.real_property_transfer_id AND NOT p.is_voided
	    )
	    OR EXISTS (
	      SELECT 1
	      FROM core.real_property_transfer_party_improvements x
	      WHERE x.real_property_transfer_id = rpt.real_property_transfer_id
	        AND ` + fmt.Sprintf(visibleImprovementSQL, "x.improvement_id") + `
	    )
	  )
	  AND ($4::timestamptz IS NULL OR (rpt.transfer_timestamp, rpt.public_id) < ($4::timestamptz, $5::uuid))
	ORDER BY rpt.transfer_timestamp DESC, rpt.public_id DESC
	LIMIT $6`

const (
	transfersByIDFilter = `rpt.public_id = ANY($1::uuid[])`

	transfersByParcelFilter = `EXISTS (
		SELECT 1
		FROM core.real_property_transfer_party_parcels f
		JOIN core.parcels fp ON fp.parcel_id = f.parcel_id
		WHERE f.real_property_transfer_id = rpt.real_property_transfer_id
		  AND NOT fp.is_voided AND fp.public_id = ANY($1::uuid[])
	)`

	transfersByFeatureFilter = `EXISTS (
		SELECT 1
		FROM core.real_property_transfer_party_parcels f
		JOIN core.parcels fp ON fp.parcel_id = f.parcel_id
		JOIN geo.parcel_geometry fg ON fg.parcel_id = fp.parcel_id AND fg.legal_valid_range @> now()
		WHERE f.real_property_transfer_id = rpt.real_property_transfer_id
		  AND NOT fp.is_voided AND fg.feature_id = ANY($1::bigint[])
	)`
)

// partyColumnsSQL and partyJoinsSQL load a share row's party, with its name
// and mailing address as of the transfer (unset when the address is voided).
// The attribute tables' exclusion constraints allow at most one row per party
// (or address) and time, so the joins can't duplicate a share.
const (
	partyColumnsSQL = `pt.public_id::text, pt.is_voided, x.is_grantor, x.transferred_share::text, pa.name, aa.formatted_address`
	partyJoinsSQL   = `
	JOIN core.real_property_transfers rpt ON rpt.real_property_transfer_id = x.real_property_transfer_id
	JOIN core.parties pt ON pt.party_id = x.party_id
	LEFT JOIN core.party_attributes pa ON pa.party_id = pt.party_id AND pa.legal_valid_range @> rpt.transfer_timestamp
	LEFT JOIN core.addresses ma ON ma.address_id = pa.address_id AND NOT ma.is_voided
	LEFT JOIN core.address_attributes aa ON aa.address_id = ma.address_id AND aa.legal_valid_range @> rpt.transfer_timestamp`
)

// Rows of voided parties are selected too (rule 3 drops them in Go), so an
// entry whose parties are all voided is still returned.
var (
	transferParcelRowsSQL = `
	SELECT x.real_property_transfer_id, p.public_id::text, g.feature_id, ` + partyColumnsSQL + `
	FROM core.real_property_transfer_party_parcels x
	JOIN core.parcels p ON p.parcel_id = x.parcel_id AND NOT p.is_voided
	LEFT JOIN geo.parcel_geometry g ON g.parcel_id = p.parcel_id AND g.legal_valid_range @> now()` + partyJoinsSQL + `
	WHERE x.real_property_transfer_id = ANY($1::bigint[])`

	transferImprovementRowsSQL = `
	SELECT x.real_property_transfer_id, i.public_id::text, ` + partyColumnsSQL + `
	FROM core.real_property_transfer_party_improvements x
	JOIN core.improvements i ON i.improvement_id = x.improvement_id` + partyJoinsSQL + `
	WHERE x.real_property_transfer_id = ANY($1::bigint[])
	  AND ` + fmt.Sprintf(visibleImprovementSQL, "x.improvement_id")
)

const transferCodeRowsSQL = `
	SELECT ca.real_property_transfer_id, c.public_id::text, c.name, c.description,
	       ct.public_id::text, ct.name, ct.description
	FROM core.real_property_transfer_code_assignments ca
	JOIN core.real_property_transfer_codes c ON c.real_property_transfer_code_id = ca.real_property_transfer_code_id
	LEFT JOIN core.real_property_transfer_code_types ct ON ct.real_property_transfer_code_type_id = c.real_property_transfer_code_type_id
	WHERE ca.real_property_transfer_id = ANY($1::bigint[])`

func (s *APIServer) GetTransfersById(
	ctx context.Context,
	req *connect.Request[transfersv1.GetTransfersByIdRequest],
) (*connect.Response[transfersv1.GetTransfersByIdResponse], error) {
	s.logger.Debug("received GetTransfersById request", slog.Int("ids", len(req.Msg.TransferIds)))

	if len(req.Msg.TransferIds) == 0 {
		return connect.NewResponse(&transfersv1.GetTransfersByIdResponse{}), nil
	}
	transfers, _, err := s.readTransfers(ctx, transfersByIDFilter, req.Msg.TransferIds, nil, nil, 0)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&transfersv1.GetTransfersByIdResponse{Transfers: transfers}), nil
}

func (s *APIServer) GetTransfersByParcelId(
	ctx context.Context,
	req *connect.Request[transfersv1.GetTransfersByParcelIdRequest],
) (*connect.Response[transfersv1.GetTransfersByParcelIdResponse], error) {
	s.logger.Debug("received GetTransfersByParcelId request", slog.Int("ids", len(req.Msg.ParcelIds)))

	edges, pageInfo, err := s.pageTransfers(ctx, "GetTransfersByParcelId", transfersByParcelFilter,
		req.Msg.ParcelIds, req.Msg.ParcelIds, req.Msg.TimeRange, req.Msg.First, req.Msg.After)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&transfersv1.GetTransfersByParcelIdResponse{Edges: edges, PageInfo: pageInfo}), nil
}

func (s *APIServer) GetTransfersByFeatureId(
	ctx context.Context,
	req *connect.Request[transfersv1.GetTransfersByFeatureIdRequest],
) (*connect.Response[transfersv1.GetTransfersByFeatureIdResponse], error) {
	s.logger.Debug("received GetTransfersByFeatureId request", slog.Int("ids", len(req.Msg.FeatureIds)))

	hashIDs := make([]string, len(req.Msg.FeatureIds))
	for i, id := range req.Msg.FeatureIds {
		hashIDs[i] = strconv.FormatInt(id, 10)
	}
	edges, pageInfo, err := s.pageTransfers(ctx, "GetTransfersByFeatureId", transfersByFeatureFilter,
		req.Msg.FeatureIds, hashIDs, req.Msg.TimeRange, req.Msg.First, req.Msg.After)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&transfersv1.GetTransfersByFeatureIdResponse{Edges: edges, PageInfo: pageInfo}), nil
}

// pageTransfers reads one page of a paginated transfer lookup. ids is the
// filter's $1 argument, and hashIDs the same IDs as strings for the cursor's
// filter hash.
func (s *APIServer) pageTransfers(
	ctx context.Context,
	rpc, filter string,
	ids any,
	hashIDs []string,
	timeRange *interval.Interval,
	first *int32,
	after *string,
) ([]*transfersv1.RealPropertyTransferEdge, *paginationv1.PageInfo, error) {
	pageInfo := &paginationv1.PageInfo{HasPreviousPage: after != nil}
	if len(hashIDs) == 0 {
		return nil, pageInfo, nil
	}

	hash := transferFilterHash(rpc, hashIDs, timeRange)
	var afterKey *transferKey
	if after != nil {
		key, err := decodeTransferCursor(*after, hash)
		if err != nil {
			return nil, nil, err
		}
		afterKey = &key
	}

	size := transferPageSize(first)
	transfers, keys, err := s.readTransfers(ctx, filter, ids, timeRange, afterKey, size+1)
	if err != nil {
		return nil, nil, err
	}
	if len(transfers) > size {
		pageInfo.HasNextPage = true
		transfers, keys = transfers[:size], keys[:size]
	}

	edges := make([]*transfersv1.RealPropertyTransferEdge, len(transfers))
	for i, t := range transfers {
		cursor, err := encodeTransferCursor(keys[i], hash)
		if err != nil {
			s.logger.Error("failed to encode transfer cursor", slog.Any("error", err))
			return nil, nil, connect.NewError(connect.CodeInternal, fmt.Errorf("cursor encoding error"))
		}
		edges[i] = &transfersv1.RealPropertyTransferEdge{Cursor: cursor, Node: t}
	}
	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}
	return edges, pageInfo, nil
}

// readTransfers returns the visible transfers matching filter, in sort
// order, with each one's sort key. limit 0 means no limit.
func (s *APIServer) readTransfers(
	ctx context.Context,
	filter string,
	ids any,
	timeRange *interval.Interval,
	after *transferKey,
	limit int,
) ([]*transfersv1.RealPropertyTransfer, []transferKey, error) {
	var start, end, afterTime *time.Time
	if ts := timeRange.GetStartTime(); ts != nil {
		t := ts.AsTime()
		start = &t
	}
	if ts := timeRange.GetEndTime(); ts != nil {
		t := ts.AsTime()
		end = &t
	}
	var afterID *string
	if after != nil {
		afterTime, afterID = &after.time, &after.id
	}
	var limitArg *int
	if limit > 0 {
		limitArg = &limit
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(transferSelectSQL, filter), ids, start, end, afterTime, afterID, limitArg)
	if err != nil {
		s.logger.Error("failed to query transfers", slog.Any("error", err))
		return nil, nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}

	defer rows.Close()

	var (
		transfers []*transfersv1.RealPropertyTransfer
		keys      []transferKey
		dbIDs     []int64
		byDBID    = map[int64]*transferBuilder{}
	)
	for rows.Next() {
		var (
			dbID                    int64
			publicID                string
			ts                      time.Time
			amount, book, page, uri *string
		)
		if err := rows.Scan(&dbID, &publicID, &ts, &amount, &book, &page, &uri); err != nil {
			s.logger.Error("failed to scan transfer", slog.Any("error", err))
			return nil, nil, connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
		}
		t := &transfersv1.RealPropertyTransfer{
			TransferId:     publicID,
			TransferTime:   timestamppb.New(ts),
			TransferAmount: amount,
		}
		if book != nil || page != nil || uri != nil {
			t.Recordings = []*recordingsv1.Recording{{Book: book, Page: page, Uri: uri}}
		}
		transfers = append(transfers, t)
		keys = append(keys, transferKey{time: ts, id: publicID})
		dbIDs = append(dbIDs, dbID)
		byDBID[dbID] = newTransferBuilder(t)
	}
	if err := rows.Err(); err != nil {
		s.logger.Error("failed to read transfers", slog.Any("error", err))
		return nil, nil, connect.NewError(connect.CodeInternal, fmt.Errorf("error reading database rows"))
	}
	rows.Close()

	if len(dbIDs) == 0 {
		return nil, nil, nil
	}
	if err := s.loadTransferDetails(ctx, dbIDs, byDBID); err != nil {
		return nil, nil, err
	}
	for _, b := range byDBID {
		b.finish()
	}
	return transfers, keys, nil
}

// loadTransferDetails fills in the parcel entries, improvement entries,
// parties and codes of the transfers in byDBID.
func (s *APIServer) loadTransferDetails(ctx context.Context, dbIDs []int64, byDBID map[int64]*transferBuilder) error {
	queryErr := func(what string, err error) error {
		s.logger.Error("failed to query transfer "+what, slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}
	scanErr := func(what string, err error) error {
		s.logger.Error("failed to scan transfer "+what, slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
	}

	rows, err := s.db.Query(ctx, transferParcelRowsSQL, dbIDs)
	if err != nil {
		return queryErr("parcels", err)
	}
	var (
		dbID      int64
		entityID  string
		featureID *int64
		share     shareRow
	)
	_, err = pgx.ForEachRow(rows, []any{&dbID, &entityID, &featureID,
		&share.partyID, &share.partyVoided, &share.isGrantor, &share.share, &share.name, &share.address,
	}, func() error {
		byDBID[dbID].addParcelShare(entityID, clone(featureID), share.copied())
		return nil
	})
	if err != nil {
		return scanErr("parcels", err)
	}

	rows, err = s.db.Query(ctx, transferImprovementRowsSQL, dbIDs)
	if err != nil {
		return queryErr("improvements", err)
	}
	_, err = pgx.ForEachRow(rows, []any{&dbID, &entityID,
		&share.partyID, &share.partyVoided, &share.isGrantor, &share.share, &share.name, &share.address,
	}, func() error {
		byDBID[dbID].addImprovementShare(entityID, share.copied())
		return nil
	})
	if err != nil {
		return scanErr("improvements", err)
	}

	rows, err = s.db.Query(ctx, transferCodeRowsSQL, dbIDs)
	if err != nil {
		return queryErr("codes", err)
	}
	var (
		code                       transfersv1.TransferCode
		typeID, typeName, typeDesc *string
	)
	_, err = pgx.ForEachRow(rows, []any{&dbID, &code.CodeId, &code.Name, &code.Description, &typeID, &typeName, &typeDesc}, func() error {
		c := &transfersv1.TransferCode{CodeId: code.CodeId, Name: code.Name, Description: clone(code.Description)}
		if typeID != nil {
			c.Type = &transfersv1.TransferCodeType{CodeTypeId: *typeID, Name: deref(typeName), Description: clone(typeDesc)}
		}
		byDBID[dbID].transfer.Codes = append(byDBID[dbID].transfer.Codes, c)
		return nil
	})
	if err != nil {
		return scanErr("codes", err)
	}
	return nil
}

// clone copies a scanned pointer value, so a value kept from one row can't
// change when pgx scans the next row into the same target.
func clone[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// copied returns r with its pointer fields cloned.
func (r shareRow) copied() shareRow {
	r.name, r.address = clone(r.name), clone(r.address)
	return r
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// shareRow is one party's share of a parcel or improvement in a transfer.
type shareRow struct {
	partyID     string
	partyVoided bool
	isGrantor   bool
	share       string
	name        *string
	address     *string
}

// transferBuilder collects a transfer's detail rows, then sorts them so
// responses are deterministic.
type transferBuilder struct {
	transfer     *transfersv1.RealPropertyTransfer
	parcels      map[string]*transfersv1.TransferredParcel
	improvements map[string]*transfersv1.TransferredImprovement
	parties      map[string]*transfersv1.TransferParty
}

func newTransferBuilder(t *transfersv1.RealPropertyTransfer) *transferBuilder {
	return &transferBuilder{
		transfer:     t,
		parcels:      map[string]*transfersv1.TransferredParcel{},
		improvements: map[string]*transfersv1.TransferredImprovement{},
		parties:      map[string]*transfersv1.TransferParty{},
	}
}

// addShare applies rule 3 and returns the share to list, or nil for a voided
// party.
func (b *transferBuilder) addShare(r shareRow) *transfersv1.TransferShare {
	if r.partyVoided {
		return nil
	}
	if _, ok := b.parties[r.partyID]; !ok {
		b.parties[r.partyID] = &transfersv1.TransferParty{PartyId: r.partyID, Name: r.name, MailingAddress: r.address}
	}
	return &transfersv1.TransferShare{PartyId: r.partyID, TransferredShare: r.share}
}

func (b *transferBuilder) addParcelShare(parcelID string, featureID *int64, r shareRow) {
	p, ok := b.parcels[parcelID]
	if !ok {
		p = &transfersv1.TransferredParcel{ParcelId: parcelID, FeatureId: featureID}
		b.parcels[parcelID] = p
	}
	if sh := b.addShare(r); sh != nil {
		if r.isGrantor {
			p.Grantors = append(p.Grantors, sh)
		} else {
			p.Grantees = append(p.Grantees, sh)
		}
	}
}

func (b *transferBuilder) addImprovementShare(improvementID string, r shareRow) {
	imp, ok := b.improvements[improvementID]
	if !ok {
		imp = &transfersv1.TransferredImprovement{ImprovementId: improvementID}
		b.improvements[improvementID] = imp
	}
	if sh := b.addShare(r); sh != nil {
		if r.isGrantor {
			imp.Grantors = append(imp.Grantors, sh)
		} else {
			imp.Grantees = append(imp.Grantees, sh)
		}
	}
}

// finish moves the collected entries onto the transfer, each list sorted by
// its ID.
func (b *transferBuilder) finish() {
	byParty := func(a, c *transfersv1.TransferShare) int { return cmp.Compare(a.PartyId, c.PartyId) }

	t := b.transfer
	for _, id := range sortedMapKeys(b.parcels) {
		p := b.parcels[id]
		slices.SortFunc(p.Grantors, byParty)
		slices.SortFunc(p.Grantees, byParty)
		t.Parcels = append(t.Parcels, p)
	}
	for _, id := range sortedMapKeys(b.improvements) {
		imp := b.improvements[id]
		slices.SortFunc(imp.Grantors, byParty)
		slices.SortFunc(imp.Grantees, byParty)
		t.Improvements = append(t.Improvements, imp)
	}
	for _, id := range sortedMapKeys(b.parties) {
		t.Parties = append(t.Parties, b.parties[id])
	}
	slices.SortFunc(t.Codes, func(a, c *transfersv1.TransferCode) int { return cmp.Compare(a.CodeId, c.CodeId) })
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
