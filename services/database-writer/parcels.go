package main

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	parcelsv1 "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ParcelServer struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func (s *ParcelServer) GetParcelAttribute(
	ctx context.Context,
	req *connect.Request[parcelsv1.GetParcelAttributeRequest],
) (*connect.Response[parcelsv1.GetParcelAttributeResponse], error) {

	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("this operation is not supported on this service"))
}

func (s *ParcelServer) GetParcelProperty(
	ctx context.Context,
	req *connect.Request[parcelsv1.GetParcelPropertyRequest],
) (*connect.Response[parcelsv1.GetParcelPropertyResponse], error) {

	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("this operation is not supported on this service"))
}
