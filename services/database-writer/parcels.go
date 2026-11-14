package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
)

// Unit conversions are the exact inverses of the reader's (services/database-reader/parcels.go),
// so a value written in feet reads back unchanged.
const (
	feetPerMeter             = 3.28084
	squareFeetPerSquareMeter = 10.7639
)

// Postgres SQLSTATE for check_violation (e.g. the properties_size_limit constraint).
const pgCheckViolation = "23514"

// ParcelServer implements ParcelWriteService.
type ParcelServer struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// parcelUpdate is a validated UpdateParcelRequest with values in storage units.
// A nil field means "leave unchanged".
type parcelUpdate struct {
	parcelPublicID  string
	addressPublicID *string
	landUsePublicID *string
	landAreaSqM     *float64
	frontageM       *float64
	depthM          *float64
	properties      *string // a JSON object, stored as jsonb
}

// UpdateParcel corrects the current legal record of a parcel's attributes in
// place (spec §5.3, decision C1). The row's legal_valid_range is not changed;
// the database trigger records the previous version in parcel_attributes_history.
func (s *ParcelServer) UpdateParcel(
	ctx context.Context,
	req *connect.Request[parcelsv1.UpdateParcelRequest],
) (*connect.Response[parcelsv1.UpdateParcelResponse], error) {
	upd, err := newParcelUpdate(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	traceID := traceIDFromTraceparent(req.Header().Get("traceparent"))
	logger := s.logger.With(slog.String("parcel_id", upd.parcelPublicID), slog.String("trace_id", traceID))
	logger.Debug("received UpdateParcel request")

	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		return applyParcelUpdate(ctx, tx, upd, traceID)
	})
	if err != nil {
		var connectErr *connect.Error
		if errors.As(err, &connectErr) {
			return nil, connectErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgCheckViolation {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("value rejected by database constraint %q", pgErr.ConstraintName))
		}
		logger.Error("UpdateParcel failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to update parcel"))
	}

	logger.Info("parcel updated")
	return connect.NewResponse(&parcelsv1.UpdateParcelResponse{}), nil
}

// newParcelUpdate validates the request and converts feet / square feet to
// the meters / square meters the schema stores.
func newParcelUpdate(msg *parcelsv1.UpdateParcelRequest) (parcelUpdate, error) {
	if msg.GetParcelId() == "" {
		return parcelUpdate{}, errors.New("parcel_id is required")
	}
	upd := parcelUpdate{
		parcelPublicID:  msg.GetParcelId(),
		addressPublicID: msg.AddressId,
		landUsePublicID: msg.LandUseId,
	}

	var err error
	if upd.landAreaSqM, err = convertNonNegative("land_area_sq_ft", msg.LandAreaSqFt, squareFeetPerSquareMeter); err != nil {
		return parcelUpdate{}, err
	}
	if upd.frontageM, err = convertNonNegative("frontage_ft", msg.FrontageFt, feetPerMeter); err != nil {
		return parcelUpdate{}, err
	}
	if upd.depthM, err = convertNonNegative("depth_ft", msg.DepthFt, feetPerMeter); err != nil {
		return parcelUpdate{}, err
	}

	if msg.Properties != nil {
		var obj map[string]any
		if err := json.Unmarshal([]byte(*msg.Properties), &obj); err != nil || obj == nil {
			return parcelUpdate{}, errors.New("properties must be a JSON object")
		}
		upd.properties = msg.Properties
	}

	return upd, nil
}

func convertNonNegative(field string, value *float64, unitsPerStorageUnit float64) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return nil, fmt.Errorf("%s must be a finite, non-negative number", field)
	}
	converted := *value / unitsPerStorageUnit
	return &converted, nil
}

// applyParcelUpdate resolves public IDs and updates the current attributes row
// inside tx. Expected failures are returned as *connect.Error.
func applyParcelUpdate(ctx context.Context, tx pgx.Tx, upd parcelUpdate, traceID string) error {
	var parcelID int64
	err := tx.QueryRow(ctx,
		`SELECT parcel_id FROM core.parcels WHERE public_id = $1::uuid AND NOT is_voided FOR UPDATE`,
		upd.parcelPublicID,
	).Scan(&parcelID)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("parcel %s not found", upd.parcelPublicID))
	}
	if err != nil {
		return err
	}

	set := []string{}
	args := []any{parcelID}
	add := func(column string, value any) {
		args = append(args, value)
		set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if upd.addressPublicID != nil {
		id, err := lookupID(ctx, tx, `SELECT address_id FROM core.addresses WHERE public_id = $1::uuid AND NOT is_voided`, *upd.addressPublicID, "address_id")
		if err != nil {
			return err
		}
		add("address_id", id)
	}
	if upd.landUsePublicID != nil {
		id, err := lookupID(ctx, tx, `SELECT land_use_id FROM core.land_uses WHERE public_id = $1::uuid`, *upd.landUsePublicID, "land_use_id")
		if err != nil {
			return err
		}
		add("land_use_id", id)
	}
	if upd.landAreaSqM != nil {
		add("land_area_sq_m", *upd.landAreaSqM)
	}
	if upd.frontageM != nil {
		add("frontage_m", *upd.frontageM)
	}
	if upd.depthM != nil {
		add("depth_m", *upd.depthM)
	}
	if upd.properties != nil {
		args = append(args, *upd.properties)
		set = append(set, fmt.Sprintf("properties = $%d::jsonb", len(args)))
	}
	add("trace_id", traceID)

	tag, err := tx.Exec(ctx,
		`UPDATE core.parcel_attributes SET `+strings.Join(set, ", ")+
			` WHERE parcel_id = $1 AND legal_valid_range @> now()`,
		args...,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("parcel %s has no attributes record valid now", upd.parcelPublicID))
	}
	return nil
}

func lookupID(ctx context.Context, tx pgx.Tx, query, publicID, field string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, query, publicID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s %s not found", field, publicID))
	}
	return id, err
}

// traceIDFromTraceparent returns the trace ID from a W3C traceparent header
// ("00-<32 hex trace id>-<16 hex span id>-<2 hex flags>"), or a new random
// 32-hex-character ID when the header is absent or invalid (spec decision C2).
func traceIDFromTraceparent(header string) string {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) == 4 && len(parts[0]) == 2 && parts[0] != "ff" && len(parts[1]) == 32 && len(parts[2]) == 16 && len(parts[3]) == 2 {
		traceID := strings.ToLower(parts[1])
		if _, err := hex.DecodeString(traceID); err == nil && traceID != strings.Repeat("0", 32) {
			return traceID
		}
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error on supported platforms
	return hex.EncodeToString(b)
}
