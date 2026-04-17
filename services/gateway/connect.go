package main

import (
	"context"
	"log"

	parcelsv1 "github.com/civil-labs/civil-api-go/civil/public/parcels/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

type ParcelServer struct{}

func (s *ParcelServer) UpdateParcelsAttribute(
	_ context.Context,
	req *connect.Request[parcelsv1.UpdateParcelsAttributeRequest],
) (*connect.Response[parcelsv1.UpdateParcelsAttributeResponse], error) {
	return connect.NewResponse(&parcelsv1.UpdateParcelsAttributeResponse{}), nil
}

func (s *ParcelServer) GetParcel(
	_ context.Context,
	req *connect.Request[parcelsv1.GetParcelRequest],
) (*connect.Response[parcelsv1.GetParcelResponse], error) {
	res := &parcelsv1.GetParcelResponse{
		// Access the request data via .Msg
		ParcelAttributes: GetHardcodedParcelDetails(),
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetParcelAttribute(
	_ context.Context,
	req *connect.Request[parcelsv1.GetParcelAttributeRequest],
) (*connect.Response[parcelsv1.GetParcelAttributeResponse], error) {
	res := &parcelsv1.GetParcelAttributeResponse{
		AttributeValue: "1337",
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetParcelAttributes(
	_ context.Context,
	req *connect.Request[parcelsv1.GetParcelAttributesRequest],
) (*connect.Response[parcelsv1.GetParcelAttributesResponse], error) {
	res := &parcelsv1.GetParcelAttributesResponse{
		ParcelAttributes: GetHardcodedParcelDetails(),
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetCategoricalStats(
	_ context.Context,
	req *connect.Request[parcelsv1.GetCategoricalStatsRequest],
) (*connect.Response[parcelsv1.GetCategoricalStatsResponse], error) {
	res := &parcelsv1.GetCategoricalStatsResponse{
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
	req *connect.Request[parcelsv1.GetNumericalStatsRequest],
) (*connect.Response[parcelsv1.GetNumericalStatsResponse], error) {
	res := &parcelsv1.GetNumericalStatsResponse{
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
