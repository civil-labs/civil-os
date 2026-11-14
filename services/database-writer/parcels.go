package main

import (
	"log/slog"

	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1/parcelsv1connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ParcelServer serves parcel writes.
//
// Temporary stub: every RPC returns CodeUnimplemented via the embedded handler.
// It is replaced by the ParcelWriteService implementation in PR 3 of
// docs/monorepo-migration/plan.md.
type ParcelServer struct {
	parcelsv1connect.UnimplementedParcelWriteServiceHandler

	db     *pgxpool.Pool
	logger *slog.Logger
}
