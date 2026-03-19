package main

import (
	"context"
	"log"

	parcelsv1 "github.com/civil-labs/civil-api-go/civil/parcels/v1"

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

func GetHardcodedParcelDetails() *structpb.Struct {
	// 1. Define your data as a standard Go map
	rawDetails := map[string]any{
		"sq_ft":      2500.5,
		"zone_type":  "Residential",
		"is_cleared": true,
		"tags":       []string{"prime", "corner-lot"},
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
