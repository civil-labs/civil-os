package main

import (
	"context"
	"errors"
	"log"
	"log/slog"

	publicparcelsv1 "github.com/civil-labs/civil-api-go/civil/public/parcels/v1"

	meshparcelsv1 "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1"
	meshparcelsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

type ParcelServer struct {
	dbReaderClient meshparcelsv1connect.ParcelsServiceClient
	logger         *slog.Logger
}

func (s *ParcelServer) UpdateParcelsAttribute(
	_ context.Context,
	req *connect.Request[publicparcelsv1.UpdateParcelsAttributeRequest],
) (*connect.Response[publicparcelsv1.UpdateParcelsAttributeResponse], error) {
	return connect.NewResponse(&publicparcelsv1.UpdateParcelsAttributeResponse{}), nil
}

func (s *ParcelServer) GetParcel(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetParcelRequest],
) (*connect.Response[publicparcelsv1.GetParcelResponse], error) {
	res := &publicparcelsv1.GetParcelResponse{
		// Access the request data via .Msg
		ParcelAttributes: GetHardcodedParcelDetails(),
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetParcelAttribute(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetParcelAttributeRequest],
) (*connect.Response[publicparcelsv1.GetParcelAttributeResponse], error) {

	s.logger.Debug("received GetParcelAttribute request")

	if req.Msg.ParcelId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("parcel ID is required"))
	}

	if req.Msg.AttributeName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("attribute name is required"))
	}

	meshReq := connect.NewRequest(&meshparcelsv1.GetParcelAttributeRequest{
		ParcelId: req.Msg.GetParcelId(),
	})

	meshRes, err := s.dbReaderClient.GetParcelAttribute(ctx, meshReq)
	if err != nil {
		// ConnectRPC automatically handles wrapping standard gRPC error codes
		// You might want to log the internal error here, but return a sanitized error to the public client
		slog.Error("upstream GetParcelAttribute request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetParcelAttributeResponse{
		AttributeValue: meshRes.Msg.AttributeValue,
	}
	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetParcelAttributes(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetParcelAttributesRequest],
) (*connect.Response[publicparcelsv1.GetParcelAttributesResponse], error) {
	res := &publicparcelsv1.GetParcelAttributesResponse{
		ParcelAttributes: GetHardcodedParcelDetails(),
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetCategoricalStats(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetCategoricalStatsRequest],
) (*connect.Response[publicparcelsv1.GetCategoricalStatsResponse], error) {
	res := &publicparcelsv1.GetCategoricalStatsResponse{
		Mode: "example",
		UniqueValues: map[string]int32{
			"value1": 12,
			"value2": 13,
		},
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetNumericalStats(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetNumericalStatsRequest],
) (*connect.Response[publicparcelsv1.GetNumericalStatsResponse], error) {
	res := &publicparcelsv1.GetNumericalStatsResponse{
		Mode:                    5,
		Minimum:                 1,
		Maximum:                 10,
		Percentile_10:           1,
		Percentile_20:           2,
		Percentile_30:           3,
		Percentile_40:           4,
		Percentile_50:           5,
		Percentile_60:           6,
		Percentile_70:           7,
		Percentile_80:           8,
		Percentile_90:           9,
		Percentile_100:          10,
		Mean:                    5,
		StandardDeviation:       2,
		CoefficientOfDispersion: 3,
	}
	return connect.NewResponse(res), nil
}

func GetHardcodedParcelDetails() *structpb.Struct {
	// 1. Define your data as a standard Go map
	rawDetails := map[string]any{
		"sq_ft":      2500.5,
		"zone_type":  "Residential",
		"is_cleared": true,
		"tags":       []any{"prime", "corner-lot"},
		"owner": map[string]any{
			"name": "Lars Doucet",
		},
	}

	// 2. Use the helper to convert it to a *structpb.Struct
	details, err := structpb.NewStruct(rawDetails)
	if err != nil {
		log.Fatalf("failed to create struct: %v", err)
	}

	return details
}
