package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	meshneighborhoodsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1"
	"github.com/google/uuid"
)

func (s *APIServer) GetNeighborhoodDefinitions(
	ctx context.Context,
	req *connect.Request[meshneighborhoodsv1.GetNeighborhoodDefinitionsRequest],
) (*connect.Response[meshneighborhoodsv1.GetNeighborhoodDefinitionsResponse], error) {
	s.logger.Debug("received GetNeighborhoodDefinitions request")

	rows, err := s.db.Query(ctx, `
		SELECT
			public_id::text,
			name,
			is_legal
		FROM core.neighborhood_definitions
	`)
	if err != nil {
		s.logger.Error("failed to query neighborhood definitions", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}
	defer rows.Close()

	defsMap := make(map[string]*meshneighborhoodsv1.NeighborhoodDefinition)
	for rows.Next() {
		var publicID, name string
		var isLegal bool
		err := rows.Scan(
			&publicID,
			&name,
			&isLegal,
		)
		if err != nil {
			s.logger.Error("failed to scan neighborhood definition row", slog.Any("error", err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
		}

		defsMap[publicID] = &meshneighborhoodsv1.NeighborhoodDefinition{
			Id:      publicID,
			Name:    name,
			IsLegal: isLegal,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("error reading database rows"))
	}

	return connect.NewResponse(&meshneighborhoodsv1.GetNeighborhoodDefinitionsResponse{
		NeighborhoodDefinitions: defsMap,
	}), nil
}

func (s *APIServer) GetNeighborhoods(
	ctx context.Context,
	req *connect.Request[meshneighborhoodsv1.GetNeighborhoodsRequest],
) (*connect.Response[meshneighborhoodsv1.GetNeighborhoodsResponse], error) {
	defID := req.Msg.NeighborhoodDefinitionId
	s.logger.Debug("received GetNeighborhoods request", slog.String("neighborhood_definition_id", req.Msg.GetNeighborhoodDefinitionId()))

	// The mesh has no validation, so check the ID before it reaches ::uuid.
	if defID != nil {
		if _, err := uuid.Parse(*defID); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("neighborhood_definition_id must be a UUID"))
		}
	}

	// A definition uses a neighborhood when any assignment row links them, at
	// any time, for a parcel that isn't voided.
	rows, err := s.db.Query(ctx, `
		SELECT
			n.public_id::text,
			n.name,
			COALESCE(used.definition_ids, ARRAY[]::text[])
		FROM core.neighborhoods n
		LEFT JOIN (
			SELECT
				pnd.neighborhood_id,
				array_agg(DISTINCT nd.public_id::text ORDER BY nd.public_id::text) AS definition_ids
			FROM core.parcel_neighborhood_definitions pnd
			JOIN core.parcels p ON p.parcel_id = pnd.parcel_id AND NOT p.is_voided
			JOIN core.neighborhood_definitions nd ON nd.neighborhood_definition_id = pnd.neighborhood_definition_id
			GROUP BY pnd.neighborhood_id
		) used ON used.neighborhood_id = n.neighborhood_id
		WHERE $1::uuid IS NULL OR $1::uuid::text = ANY(used.definition_ids)
	`, defID)
	if err != nil {
		s.logger.Error("failed to query neighborhoods", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}
	defer rows.Close()

	neighborhoodsMap := make(map[string]*meshneighborhoodsv1.Neighborhood)
	for rows.Next() {
		var publicID, name string
		var definitionIDs []string
		err := rows.Scan(
			&publicID,
			&name,
			&definitionIDs,
		)
		if err != nil {
			s.logger.Error("failed to scan neighborhood row", slog.Any("error", err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
		}

		neighborhoodsMap[publicID] = &meshneighborhoodsv1.Neighborhood{
			Id:                        publicID,
			Name:                      name,
			NeighborhoodDefinitionIds: definitionIDs,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("error reading database rows"))
	}

	return connect.NewResponse(&meshneighborhoodsv1.GetNeighborhoodsResponse{
		Neighborhoods: neighborhoodsMap,
	}), nil
}
