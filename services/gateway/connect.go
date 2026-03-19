package main

import (
	"context"

	parcelsv1 "github.com/civil-labs/civil-api-go/civil/parcels/v1"
)

type ParcelServer struct{}

func (s *ParcelServer) UpdateParcelAttribute(
	_ context.Context,
	req *parcelsv1.UpdateParcelAttributeRequest,
) (*parcelsv1.UpdateParcelAttributeResponse, error) {
	return nil, nil
}