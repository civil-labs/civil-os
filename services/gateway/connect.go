package main

import (
	"context"

	parcelsv1 "github.com/civil-labs/civil-api-go/civil/parcels/v1"

	"google.golang.org/protobuf/types/known/structpb"
)

type ParcelServer struct{}

func (s *ParcelServer) UpdateParcelsAttribute(
	_ context.Context,
	req *parcelsv1.UpdateParcelsAttributeRequest,
) (*parcelsv1.UpdateParcelsAttributeResponse, error) {
	return nil, nil
}

func (s *ParcelServer) GetParcel(
	_ context.Context,
	req *parcelsv1.GetParcelRequest,
) (*parcelsv1.GetParcelResponse, error) {
	return GetHardcodedParcelDetails(), nil
}

func (s *ParcelServer) GetParcelAttribute(
	_ context.Context,
	req *parcelsv1.GetParcelAttributeRequest,
) (*parcelsv1.GetParcelAttributeResponse, error) {
	return "1337", nil
}

func (s *ParcelServer) GetParcelAttributes(
	_ context.Context,
	req *parcelsv1.GetParcelAttributesRequest,
) (*parcelsv1.GetParcelAttributesResponse, error) {
	return GetHardcodedParcelDetails(), nil
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
