package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	meshzoningv1 "github.com/civil-labs/civil-api-go/civil/mesh/zoning/v1"
	meshzoningv1connect "github.com/civil-labs/civil-api-go/civil/mesh/zoning/v1/zoningv1connect"
	publiczoningv1 "github.com/civil-labs/civil-api-go/civil/public/zoning/v1"
)

type ZoningServer struct {
	dbReaderClient meshzoningv1connect.ZoningServiceClient
	logger         *slog.Logger
}

func (s *ZoningServer) GetZoning(
	ctx context.Context,
	req *connect.Request[publiczoningv1.GetZoningRequest],
) (*connect.Response[publiczoningv1.GetZoningResponse], error) {
	s.logger.Debug("received GetZoning request")

	meshReq := connect.NewRequest(&meshzoningv1.GetZoningRequest{})
	meshRes, err := s.dbReaderClient.GetZoning(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetZoning request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publiczoningv1.GetZoningResponse{
		Zoning: mapZoning(meshRes.Msg.Zoning),
	}

	return connect.NewResponse(publicRes), nil
}

func mapZoning(meshZoning map[string]*meshzoningv1.Zoning) map[string]*publiczoningv1.Zoning {
	if meshZoning == nil {
		return nil
	}
	publicZoning := make(map[string]*publiczoningv1.Zoning, len(meshZoning))
	for k, v := range meshZoning {
		if v == nil {
			continue
		}
		publicZoning[k] = &publiczoningv1.Zoning{
			Id:                      v.Id,
			Name:                    v.Name,
			Code:                    v.Code,
			MaxFar:                  v.MaxFar,
			MinLotSizeSqFt:          v.MinLotSizeSqFt,
			MaxHeightFt:             v.MaxHeightFt,
			MaxDwellingUnitsPerAcre: v.MaxDwellingUnitsPerAcre,
			MaxLotCoveragePct:       v.MaxLotCoveragePct,
		}
	}
	return publicZoning
}
