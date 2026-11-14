package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	publicneighborhoodsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/neighborhoods/v1"
	meshneighborhoodsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1"
	meshneighborhoodsv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1/neighborhoodsv1connect"
)

type NeighborhoodServer struct {
	dbReaderClient meshneighborhoodsv1connect.NeighborhoodReadServiceClient
	logger         *slog.Logger
}

func (s *NeighborhoodServer) GetNeighborhoodDefinitions(
	ctx context.Context,
	req *connect.Request[publicneighborhoodsv1.GetNeighborhoodDefinitionsRequest],
) (*connect.Response[publicneighborhoodsv1.GetNeighborhoodDefinitionsResponse], error) {
	s.logger.Debug("received GetNeighborhoodDefinitions request")

	meshReq := connect.NewRequest(&meshneighborhoodsv1.GetNeighborhoodDefinitionsRequest{})
	meshRes, err := s.dbReaderClient.GetNeighborhoodDefinitions(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetNeighborhoodDefinitions request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	publicNeighborhoods := make(map[string]*publicneighborhoodsv1.Neighborhood, len(meshRes.Msg.Neighborhoods))
	for k, v := range meshRes.Msg.Neighborhoods {
		if v == nil {
			continue
		}
		publicNeighborhoods[k] = &publicneighborhoodsv1.Neighborhood{
			Id:   v.Id,
			Name: v.Name,
		}
	}

	return connect.NewResponse(&publicneighborhoodsv1.GetNeighborhoodDefinitionsResponse{
		Neighborhoods: publicNeighborhoods,
	}), nil
}

func (s *NeighborhoodServer) GetNeighborhoods(
	ctx context.Context,
	req *connect.Request[publicneighborhoodsv1.GetNeighborhoodsRequest],
) (*connect.Response[publicneighborhoodsv1.GetNeighborhoodsResponse], error) {
	s.logger.Debug("received GetNeighborhoods request")

	meshReq := connect.NewRequest(&meshneighborhoodsv1.GetNeighborhoodsRequest{})
	meshRes, err := s.dbReaderClient.GetNeighborhoods(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetNeighborhoods request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	publicNeighborhoods := make(map[string]*publicneighborhoodsv1.Neighborhood, len(meshRes.Msg.Neighborhoods))
	for k, v := range meshRes.Msg.Neighborhoods {
		if v == nil {
			continue
		}
		publicNeighborhoods[k] = &publicneighborhoodsv1.Neighborhood{
			Id:   v.Id,
			Name: v.Name,
		}
	}

	return connect.NewResponse(&publicneighborhoodsv1.GetNeighborhoodsResponse{
		Neighborhoods: publicNeighborhoods,
	}), nil
}
