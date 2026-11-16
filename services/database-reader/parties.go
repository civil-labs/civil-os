package main

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	partiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
)

// Parties are read at a legal time t ($1). A party is visible when it isn't
// voided and has attributes at t; its mailing address is set only when that
// address is visible too (docs/address-party-read-api/spec.md, rules 1-2).
const (
	visiblePartyColumnsSQL = `pt.public_id::text, pa.name,
		CASE WHEN maa.address_id IS NOT NULL THEN ma.public_id::text END, maa.formatted_address`
	visiblePartyJoinsSQL = `
	JOIN core.party_attributes pa ON pa.party_id = pt.party_id AND pa.legal_valid_range @> $1::timestamptz
	LEFT JOIN core.addresses ma ON ma.address_id = pa.address_id AND NOT ma.is_voided
	LEFT JOIN core.address_attributes maa ON maa.address_id = ma.address_id AND maa.legal_valid_range @> $1::timestamptz`
)

// visibleParcelJoinsSQL joins parcel p to the rows that make it visible at
// $1 (rule 3: not voided, with geometry and attributes then) and gives pg,
// its geometry then.
const visibleParcelJoinsSQL = `
	JOIN geo.parcel_geometry pg ON pg.parcel_id = p.parcel_id AND pg.legal_valid_range @> $1::timestamptz
	JOIN core.parcel_attributes pva ON pva.parcel_id = p.parcel_id AND pva.legal_valid_range @> $1::timestamptz`

func scanParties(rows pgx.Rows) ([]*partiesv1.Party, error) {
	defer rows.Close()
	var out []*partiesv1.Party
	for rows.Next() {
		p := &partiesv1.Party{}
		if err := rows.Scan(&p.PartyId, &p.Name, &p.MailingAddressId, &p.MailingAddress); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *APIServer) dbError(what string, err error) error {
	s.logger.Error("failed to read "+what, slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
}

func (s *APIServer) GetPartiesById(
	ctx context.Context,
	req *connect.Request[partiesv1.GetPartiesByIdRequest],
) (*connect.Response[partiesv1.GetPartiesByIdResponse], error) {
	s.logger.Debug("received GetPartiesById request", slog.Int("ids", len(req.Msg.PartyIds)))

	res := &partiesv1.GetPartiesByIdResponse{Parties: map[string]*partiesv1.Party{}}
	if len(req.Msg.PartyIds) == 0 {
		return connect.NewResponse(res), nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+visiblePartyColumnsSQL+`
		FROM core.parties pt`+visiblePartyJoinsSQL+`
		WHERE NOT pt.is_voided AND pt.public_id = ANY($2::uuid[])`,
		getTargetTime(req.Msg.LegalAsOf), req.Msg.PartyIds)
	if err != nil {
		return nil, s.dbError("parties", err)
	}
	parties, err := scanParties(rows)
	if err != nil {
		return nil, s.dbError("parties", err)
	}
	for _, p := range parties {
		res.Parties[p.PartyId] = p
	}
	return connect.NewResponse(res), nil
}

func (s *APIServer) GetParcelsByPartyId(
	ctx context.Context,
	req *connect.Request[partiesv1.GetParcelsByPartyIdRequest],
) (*connect.Response[partiesv1.GetParcelsByPartyIdResponse], error) {
	s.logger.Debug("received GetParcelsByPartyId request")

	at := getTargetTime(req.Msg.LegalAsOf)
	read := func(afterID *string, limit int) ([]*partiesv1.PartyParcel, error) {
		// The party itself must be visible at t (rule 5).
		rows, err := s.db.Query(ctx, `
			SELECT p.public_id::text, pg.feature_id, pp.ownership_share::text
			FROM core.parties pt
			JOIN core.party_attributes pa ON pa.party_id = pt.party_id AND pa.legal_valid_range @> $1::timestamptz
			JOIN core.parcel_parties pp ON pp.party_id = pt.party_id AND pp.legal_valid_range @> $1::timestamptz
			JOIN core.parcels p ON p.parcel_id = pp.parcel_id AND NOT p.is_voided`+visibleParcelJoinsSQL+`
			WHERE pt.public_id = $2::uuid AND NOT pt.is_voided
			  AND ($3::uuid IS NULL OR p.public_id > $3::uuid)
			ORDER BY p.public_id
			LIMIT $4`, at, req.Msg.PartyId, afterID, limit)
		if err != nil {
			return nil, s.dbError("party parcels", err)
		}
		defer rows.Close()
		var out []*partiesv1.PartyParcel
		for rows.Next() {
			pp := &partiesv1.PartyParcel{}
			if err := rows.Scan(&pp.ParcelId, &pp.FeatureId, &pp.OwnershipShare); err != nil {
				return nil, s.dbError("party parcels", err)
			}
			out = append(out, pp)
		}
		if err := rows.Err(); err != nil {
			return nil, s.dbError("party parcels", err)
		}
		return out, nil
	}

	nodes, cursors, pageInfo, err := pageByID("GetParcelsByPartyId", req.Msg.PartyId, req.Msg.LegalAsOf,
		req.Msg.First, req.Msg.After, read, func(n *partiesv1.PartyParcel) string { return n.ParcelId })
	if err != nil {
		return nil, err
	}
	edges := make([]*partiesv1.PartyParcelEdge, len(nodes))
	for i, n := range nodes {
		edges[i] = &partiesv1.PartyParcelEdge{Cursor: cursors[i], Node: n}
	}
	return connect.NewResponse(&partiesv1.GetParcelsByPartyIdResponse{Edges: edges, PageInfo: pageInfo}), nil
}
