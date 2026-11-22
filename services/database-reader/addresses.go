package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/genproto/googleapis/type/latlng"

	addressesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/addresses/v1"
	improvementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	partiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
)

// Addresses are read at a legal time t ($1). An address is visible when it
// isn't voided and has attributes at t (docs/address-party-read-api/spec.md,
// rule 1). The reverse lookups return nothing for an address that isn't
// visible, and their keyset is $3 (the last public ID) with limit $4.

// visibleAddressCTE selects the requested address ($2) if it is visible at t.
const visibleAddressCTE = `
	WITH addr AS (
		SELECT a.address_id
		FROM core.addresses a
		JOIN core.address_attributes aa ON aa.address_id = a.address_id AND aa.legal_valid_range @> $1::timestamptz
		WHERE a.public_id = $2::uuid AND NOT a.is_voided
	)`

func (s *APIServer) GetAddressesById(
	ctx context.Context,
	req *connect.Request[addressesv1.GetAddressesByIdRequest],
) (*connect.Response[addressesv1.GetAddressesByIdResponse], error) {
	s.logger.Debug("received GetAddressesById request", slog.Int("ids", len(req.Msg.AddressIds)))

	res := &addressesv1.GetAddressesByIdResponse{Addresses: map[string]*addressesv1.Address{}}
	if len(req.Msg.AddressIds) == 0 {
		return connect.NewResponse(res), nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT a.public_id::text, aa.formatted_address,
		       aa.address_line_1, aa.address_line_2, aa.address_line_3,
		       aa.locality, aa.sublocality, aa.postal_code,
		       ar.code, ar.name, c.code, c.name,
		       ST_Y(aa.coordinates), ST_X(aa.coordinates)
		FROM core.addresses a
		JOIN core.address_attributes aa ON aa.address_id = a.address_id AND aa.legal_valid_range @> $1::timestamptz
		LEFT JOIN core.administrative_areas ar ON ar.administrative_area_id = aa.administrative_area_id
		LEFT JOIN core.countries c ON c.country_id = aa.country_id
		WHERE NOT a.is_voided AND a.public_id = ANY($2::uuid[])`,
		getTargetTime(req.Msg.LegalAsOf), req.Msg.AddressIds)
	if err != nil {
		return nil, s.dbError("addresses", err)
	}
	defer rows.Close()
	for rows.Next() {
		a := &addressesv1.Address{}
		var areaCode, areaName, countryCode, countryName *string
		var lat, lng *float64
		if err := rows.Scan(&a.AddressId, &a.FormattedAddress,
			&a.AddressLine_1, &a.AddressLine_2, &a.AddressLine_3,
			&a.Locality, &a.Sublocality, &a.PostalCode,
			&areaCode, &areaName, &countryCode, &countryName, &lat, &lng); err != nil {
			return nil, s.dbError("addresses", err)
		}
		if areaCode != nil {
			a.AdministrativeArea = &addressesv1.AdministrativeArea{Code: *areaCode, Name: deref(areaName)}
		}
		if countryCode != nil {
			a.Country = &addressesv1.Country{Code: *countryCode, Name: deref(countryName)}
		}
		if lat != nil && lng != nil {
			a.Coordinates = &latlng.LatLng{Latitude: *lat, Longitude: *lng}
		}
		res.Addresses[a.AddressId] = a
	}
	if err := rows.Err(); err != nil {
		return nil, s.dbError("addresses", err)
	}
	return connect.NewResponse(res), nil
}

func (s *APIServer) GetParcelsByAddressId(
	ctx context.Context,
	req *connect.Request[parcelsv1.GetParcelsByAddressIdRequest],
) (*connect.Response[parcelsv1.GetParcelsByAddressIdResponse], error) {
	s.logger.Debug("received GetParcelsByAddressId request")

	m := req.Msg
	at := getTargetTime(m.LegalAsOf)
	read := func(afterID *string, limit int) ([]*parcelsv1.AddressParcel, error) {
		// Visible parcels whose attributes at t name the address (rules 3 and 5).
		rows, err := s.db.Query(ctx, visibleAddressCTE+`
			SELECT p.public_id::text, pg.feature_id
			FROM addr
			JOIN core.parcel_attributes pca ON pca.address_id = addr.address_id AND pca.legal_valid_range @> $1::timestamptz
			JOIN core.parcels p ON p.parcel_id = pca.parcel_id AND NOT p.is_voided
			JOIN geo.parcel_geometry pg ON pg.parcel_id = p.parcel_id AND pg.legal_valid_range @> $1::timestamptz
			WHERE ($3::uuid IS NULL OR p.public_id > $3::uuid)
			ORDER BY p.public_id
			LIMIT $4`, at, m.AddressId, afterID, limit)
		if err != nil {
			return nil, s.dbError("address parcels", err)
		}
		return collectRows(rows, func(r pgx.Rows) (*parcelsv1.AddressParcel, error) {
			n := &parcelsv1.AddressParcel{}
			return n, r.Scan(&n.ParcelId, &n.FeatureId)
		}, func(err error) error { return s.dbError("address parcels", err) })
	}

	nodes, pageInfo, err := pageByID("GetParcelsByAddressId", m.AddressId, m.LegalAsOf,
		m.First, m.After, read, func(n *parcelsv1.AddressParcel) string { return n.ParcelId })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&parcelsv1.GetParcelsByAddressIdResponse{Parcels: nodes, PageInfo: pageInfo}), nil
}

func (s *APIServer) GetImprovementsByAddressId(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementsByAddressIdRequest],
) (*connect.Response[improvementsv1.GetImprovementsByAddressIdResponse], error) {
	s.logger.Debug("received GetImprovementsByAddressId request")

	m := req.Msg
	at := getTargetTime(m.LegalAsOf)
	read := func(afterID *string, limit int) ([]*improvementsv1.AddressImprovement, error) {
		// Improvements whose attributes at t name the address, with the
		// visible parcels they sit on then. The inner joins drop an
		// improvement with no visible parcel (rule 4).
		rows, err := s.db.Query(ctx, visibleAddressCTE+`
			SELECT i.public_id::text, array_agg(p.public_id::text ORDER BY p.public_id)
			FROM addr
			JOIN core.improvement_attributes ia ON ia.address_id = addr.address_id AND ia.legal_valid_range @> $1::timestamptz
			JOIN core.improvements i ON i.improvement_id = ia.improvement_id AND NOT i.is_voided
			JOIN core.parcel_improvements pi ON pi.improvement_id = i.improvement_id AND pi.legal_valid_range @> $1::timestamptz
			JOIN core.parcels p ON p.parcel_id = pi.parcel_id AND NOT p.is_voided`+visibleParcelJoinsSQL+`
			WHERE ($3::uuid IS NULL OR i.public_id > $3::uuid)
			GROUP BY i.public_id
			ORDER BY i.public_id
			LIMIT $4`, at, m.AddressId, afterID, limit)
		if err != nil {
			return nil, s.dbError("address improvements", err)
		}
		return collectRows(rows, func(r pgx.Rows) (*improvementsv1.AddressImprovement, error) {
			n := &improvementsv1.AddressImprovement{}
			return n, r.Scan(&n.ImprovementId, &n.ParcelIds)
		}, func(err error) error { return s.dbError("address improvements", err) })
	}

	nodes, pageInfo, err := pageByID("GetImprovementsByAddressId", m.AddressId, m.LegalAsOf,
		m.First, m.After, read, func(n *improvementsv1.AddressImprovement) string { return n.ImprovementId })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&improvementsv1.GetImprovementsByAddressIdResponse{Improvements: nodes, PageInfo: pageInfo}), nil
}

func (s *APIServer) GetPartiesByAddressId(
	ctx context.Context,
	req *connect.Request[partiesv1.GetPartiesByAddressIdRequest],
) (*connect.Response[partiesv1.GetPartiesByAddressIdResponse], error) {
	s.logger.Debug("received GetPartiesByAddressId request")

	m := req.Msg
	at := getTargetTime(m.LegalAsOf)
	read := func(afterID *string, limit int) ([]*partiesv1.Party, error) {
		// Visible parties whose attributes at t give the address as their
		// mailing address (rules 2 and 5).
		rows, err := s.db.Query(ctx, visibleAddressCTE+`
			SELECT `+visiblePartyColumnsSQL+`
			FROM core.parties pt`+visiblePartyJoinsSQL+`
			JOIN addr ON addr.address_id = pa.address_id
			WHERE NOT pt.is_voided AND ($3::uuid IS NULL OR pt.public_id > $3::uuid)
			ORDER BY pt.public_id
			LIMIT $4`, at, m.AddressId, afterID, limit)
		if err != nil {
			return nil, s.dbError("address parties", err)
		}
		parties, err := scanParties(rows)
		if err != nil {
			return nil, s.dbError("address parties", err)
		}
		return parties, nil
	}

	nodes, pageInfo, err := pageByID("GetPartiesByAddressId", m.AddressId, m.LegalAsOf,
		m.First, m.After, read, func(n *partiesv1.Party) string { return n.PartyId })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&partiesv1.GetPartiesByAddressIdResponse{Parties: nodes, PageInfo: pageInfo}), nil
}

// collectRows scans every row with scan, mapping any error with wrap.
func collectRows[T any](rows pgx.Rows, scan func(pgx.Rows) (T, error), wrap func(error) error) ([]T, error) {
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	return out, nil
}
