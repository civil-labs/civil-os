package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	publicvaluationsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/valuations/v1"
	meshvaluationsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/valuations/v1"
	meshvaluationsv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/valuations/v1/valuationsv1connect"
)

type ValuationServer struct {
	dbReaderClient meshvaluationsv1connect.ValuationReadServiceClient
	logger         *slog.Logger
}

func (s *ValuationServer) GetValuations(
	ctx context.Context,
	req *connect.Request[publicvaluationsv1.GetValuationsRequest],
) (*connect.Response[publicvaluationsv1.GetValuationsResponse], error) {
	s.logger.Debug("received GetValuations request")

	meshReq := connect.NewRequest(&meshvaluationsv1.GetValuationsRequest{})
	meshRes, err := s.dbReaderClient.GetValuations(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetValuations request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	publicValuations := make(map[string]*publicvaluationsv1.Valuation, len(meshRes.Msg.Valuations))
	for k, v := range meshRes.Msg.Valuations {
		if v == nil {
			continue
		}
		publicValuations[k] = &publicvaluationsv1.Valuation{
			Id:                 v.Id,
			ValuationTimestamp: v.ValuationTimestamp,
		}
	}

	return connect.NewResponse(&publicvaluationsv1.GetValuationsResponse{
		Valuations: publicValuations,
	}), nil
}
