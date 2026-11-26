package main

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	improvementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	"github.com/jackc/pgx/v5"
)

func (s *APIServer) GetImprovementTypes(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementTypesRequest],
) (*connect.Response[improvementsv1.GetImprovementTypesResponse], error) {
	s.logger.Debug("received GetImprovementTypes request")

	rows, err := s.db.Query(ctx, `
		SELECT 
			public_id::text, 
			COALESCE(code, ''), 
			name, 
			COALESCE(description, '') 
		FROM improvement_types
	`)
	if err != nil {
		s.logger.Error("failed to query improvement types", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}
	defer rows.Close()

	types := make(map[string]*improvementsv1.ImprovementType)
	for rows.Next() {
		var publicID, code, name, description string
		if err := rows.Scan(&publicID, &code, &name, &description); err != nil {
			s.logger.Error("failed to scan improvement type", slog.Any("error", err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
		}
		types[publicID] = &improvementsv1.ImprovementType{
			Id:          publicID,
			Code:        code,
			Name:        name,
			Description: description,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("error reading database rows"))
	}

	return connect.NewResponse(&improvementsv1.GetImprovementTypesResponse{
		ImprovementTypes: types,
	}), nil
}

func (s *APIServer) GetImprovementConditions(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementConditionsRequest],
) (*connect.Response[improvementsv1.GetImprovementConditionsResponse], error) {
	s.logger.Debug("received GetImprovementConditions request")

	rows, err := s.db.Query(ctx, `
		SELECT 
			c.public_id::text, 
			a.name, 
			a.depreciation_modifier::float4 
		FROM improvement_conditions c
		JOIN improvement_condition_attributes a ON c.improvement_condition_id = a.improvement_condition_id
		WHERE NOT c.is_voided
		  AND CURRENT_TIMESTAMP <@ a.legal_valid_range
	`)
	if err != nil {
		s.logger.Error("failed to query improvement conditions", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database query error"))
	}
	defer rows.Close()

	conditions := make(map[string]*improvementsv1.ImprovementCondition)
	for rows.Next() {
		var publicID, name string
		var depreciationModifier float32
		if err := rows.Scan(&publicID, &name, &depreciationModifier); err != nil {
			s.logger.Error("failed to scan improvement condition", slog.Any("error", err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("data scanning error"))
		}
		depVal := depreciationModifier
		conditions[publicID] = &improvementsv1.ImprovementCondition{
			Id:                   publicID,
			Name:                 name,
			DepreciationModifier: &depVal,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("error reading database rows"))
	}

	return connect.NewResponse(&improvementsv1.GetImprovementConditionsResponse{
		ImprovementConditions: conditions,
	}), nil
}

// GetImprovementQualities lists the qualities that aren't voided, with their
// attributes at the current time, as GetImprovementConditions does
// (docs/improvement-read-api/spec.md, "Quality lookup").
func (s *APIServer) GetImprovementQualities(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementQualitiesRequest],
) (*connect.Response[improvementsv1.GetImprovementQualitiesResponse], error) {
	s.logger.Debug("received GetImprovementQualities request")

	rows, err := s.db.Query(ctx, `
		SELECT q.public_id::text, a.name, a.quality_adjustment_modifier
		FROM improvement_qualities q
		JOIN improvement_quality_attributes a ON q.improvement_quality_id = a.improvement_quality_id
		WHERE NOT q.is_voided
		  AND CURRENT_TIMESTAMP <@ a.legal_valid_range
	`)
	if err != nil {
		return nil, s.dbError("improvement qualities", err)
	}
	qualities, err := collectRows(rows, func(r pgx.Rows) (*improvementsv1.ImprovementQuality, error) {
		q := &improvementsv1.ImprovementQuality{}
		return q, r.Scan(&q.Id, &q.Name, &q.QualityAdjustmentModifier)
	}, func(err error) error { return s.dbError("improvement qualities", err) })
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*improvementsv1.ImprovementQuality, len(qualities))
	for _, q := range qualities {
		byID[q.Id] = q
	}
	return connect.NewResponse(&improvementsv1.GetImprovementQualitiesResponse{
		ImprovementQualities: byID,
	}), nil
}

// sqFtPerSqM converts the schema's square metres to the API's square feet,
// with the factor the parcel reads use.
const sqFtPerSqM = 10.7639

// improvementBathrooms is Improvement.bathrooms: bathroomTotal with an unset
// count taken as 0, and unset only when both counts are.
func improvementBathrooms(full, half *int32) *float64 {
	if full == nil && half == nil {
		return nil
	}
	var f, h int32
	if full != nil {
		f = *full
	}
	if half != nil {
		h = *half
	}
	total := bathroomTotal(f, h)
	return &total
}

// improvementOnVisibleParcelSQL is the link half of improvement visibility
// rule 1 (docs/improvement-read-api/spec.md): improvement i has a
// parcel_improvements link at $1 to a visible parcel. The caller checks that
// i isn't voided and has attributes at $1.
const improvementOnVisibleParcelSQL = `EXISTS (
		SELECT 1
		FROM core.parcel_improvements vpi
		JOIN core.parcels p ON p.parcel_id = vpi.parcel_id AND NOT p.is_voided` + visibleParcelJoinsSQL + `
		WHERE vpi.improvement_id = i.improvement_id AND vpi.legal_valid_range @> $1::timestamptz
	)`

// GetImprovementsById reads improvements at legal_as_of, each with its
// visible parcel links and party shares, and its values in the requested
// valuation. Hidden and unknown IDs are left out of the map.
func (s *APIServer) GetImprovementsById(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementsByIdRequest],
) (*connect.Response[improvementsv1.GetImprovementsByIdResponse], error) {
	s.logger.Debug("received GetImprovementsById request", slog.Int("ids", len(req.Msg.ImprovementIds)))

	res := &improvementsv1.GetImprovementsByIdResponse{Improvements: map[string]*improvementsv1.Improvement{}}
	if len(req.Msg.ImprovementIds) == 0 {
		return connect.NewResponse(res), nil
	}
	at := getTargetTime(req.Msg.LegalAsOf)

	// Step 1: the visible improvements and their own fields. Condition and
	// quality IDs are set only when their lookups would return them now; the
	// address only when it's visible at t; values only in the requested
	// valuation at t.
	rows, err := s.db.Query(ctx, `
		SELECT i.improvement_id, i.public_id::text, it.public_id::text,
		       CASE WHEN ca.improvement_condition_id IS NOT NULL THEN c.public_id::text END,
		       CASE WHEN qa.improvement_quality_id IS NOT NULL THEN q.public_id::text END,
		       CASE WHEN aa.address_id IS NOT NULL THEN a.public_id::text END, aa.formatted_address,
		       ia.area_sq_m, ia.full_bathrooms, ia.half_bathrooms, ia.bedrooms, ia.units,
		       ia.year_built, ia.effective_year_built, ia.properties::text,
		       iv.market_value::numeric(19,4)::text, iv.assessed_value::numeric(19,4)::text
		FROM core.improvements i
		JOIN core.improvement_attributes ia ON ia.improvement_id = i.improvement_id AND ia.legal_valid_range @> $1::timestamptz
		JOIN core.improvement_types it ON it.improvement_type_id = ia.improvement_type_id
		LEFT JOIN core.improvement_conditions c ON c.improvement_condition_id = ia.improvement_condition_id AND NOT c.is_voided
		LEFT JOIN core.improvement_condition_attributes ca ON ca.improvement_condition_id = c.improvement_condition_id
		       AND CURRENT_TIMESTAMP <@ ca.legal_valid_range
		LEFT JOIN core.improvement_qualities q ON q.improvement_quality_id = ia.improvement_quality_id AND NOT q.is_voided
		LEFT JOIN core.improvement_quality_attributes qa ON qa.improvement_quality_id = q.improvement_quality_id
		       AND CURRENT_TIMESTAMP <@ qa.legal_valid_range
		LEFT JOIN core.addresses a ON a.address_id = ia.address_id AND NOT a.is_voided
		LEFT JOIN core.address_attributes aa ON aa.address_id = a.address_id AND aa.legal_valid_range @> $1::timestamptz
		LEFT JOIN core.valuations v ON v.public_id = $3::uuid
		LEFT JOIN core.improvement_valuations iv ON iv.improvement_id = i.improvement_id
		       AND iv.valuation_id = v.valuation_id AND iv.legal_valid_range @> $1::timestamptz
		WHERE i.public_id = ANY($2::uuid[]) AND NOT i.is_voided
		  AND `+improvementOnVisibleParcelSQL,
		at, req.Msg.ImprovementIds, req.Msg.ValuationId)
	if err != nil {
		return nil, s.dbError("improvements", err)
	}
	byInternal := map[int64]*improvementsv1.Improvement{}
	var internalIDs []int64
	for rows.Next() {
		var id int64
		var areaSqM *float64
		m := &improvementsv1.Improvement{}
		if err := rows.Scan(&id, &m.ImprovementId, &m.TypeId, &m.ConditionId, &m.QualityId,
			&m.AddressId, &m.FormattedAddress, &areaSqM, &m.FullBathrooms, &m.HalfBathrooms,
			&m.Bedrooms, &m.Units, &m.YearBuilt, &m.EffectiveYearBuilt, &m.Properties,
			&m.MarketValue, &m.AssessedValue); err != nil {
			rows.Close()
			return nil, s.dbError("improvements", err)
		}
		if areaSqM != nil {
			sqFt := *areaSqM * sqFtPerSqM
			m.AreaSqFt = &sqFt
		}
		m.Bathrooms = improvementBathrooms(m.FullBathrooms, m.HalfBathrooms)
		byInternal[id] = m
		internalIDs = append(internalIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, s.dbError("improvements", err)
	}
	if len(internalIDs) == 0 {
		return connect.NewResponse(res), nil
	}

	// Step 2: visible parcel links at t (rule 2), ordered by parcel ID.
	rows, err = s.db.Query(ctx, `
		SELECT pi.improvement_id, p.public_id::text, pg.feature_id, pi.is_primary
		FROM core.parcel_improvements pi
		JOIN core.parcels p ON p.parcel_id = pi.parcel_id AND NOT p.is_voided`+visibleParcelJoinsSQL+`
		WHERE pi.improvement_id = ANY($2::bigint[]) AND pi.legal_valid_range @> $1::timestamptz
		ORDER BY pi.improvement_id, p.public_id`, at, internalIDs)
	if err != nil {
		return nil, s.dbError("improvement parcels", err)
	}
	for rows.Next() {
		var id int64
		link := &improvementsv1.ImprovementParcel{}
		if err := rows.Scan(&id, &link.ParcelId, &link.FeatureId, &link.IsPrimary); err != nil {
			rows.Close()
			return nil, s.dbError("improvement parcels", err)
		}
		if m := byInternal[id]; m != nil {
			m.Parcels = append(m.Parcels, link)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, s.dbError("improvement parcels", err)
	}

	// Step 3: visible party shares at t (rule 3), ordered by party ID. A
	// hidden party never hides the improvement.
	rows, err = s.db.Query(ctx, `
		SELECT ip.improvement_id, pt.public_id::text, ip.ownership_share::text
		FROM core.improvement_parties ip
		JOIN core.parties pt ON pt.party_id = ip.party_id AND NOT pt.is_voided
		JOIN core.party_attributes pa ON pa.party_id = pt.party_id AND pa.legal_valid_range @> $1::timestamptz
		WHERE ip.improvement_id = ANY($2::bigint[]) AND ip.legal_valid_range @> $1::timestamptz
		ORDER BY ip.improvement_id, pt.public_id`, at, internalIDs)
	if err != nil {
		return nil, s.dbError("improvement parties", err)
	}
	for rows.Next() {
		var id int64
		share := &improvementsv1.ImprovementShare{}
		if err := rows.Scan(&id, &share.PartyId, &share.OwnershipShare); err != nil {
			rows.Close()
			return nil, s.dbError("improvement parties", err)
		}
		if m := byInternal[id]; m != nil {
			m.Parties = append(m.Parties, share)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, s.dbError("improvement parties", err)
	}

	for _, m := range byInternal {
		// Step 1 checked the same links, so this only happens if a link
		// changed between the queries. Leave it out, so parcels is never empty.
		if len(m.Parcels) == 0 {
			s.logger.Warn("improvement lost its visible parcels between queries", slog.String("improvement_id", m.ImprovementId))
			continue
		}
		res.Improvements[m.ImprovementId] = m
	}
	return connect.NewResponse(res), nil
}

// GetImprovementsByPartyId pages through the improvements a party holds at
// legal_as_of (improvement_parties), ordered by improvement ID. A hidden or
// unknown party gives an empty page (docs/improvement-read-api/spec.md,
// visibility rule 4).
func (s *APIServer) GetImprovementsByPartyId(
	ctx context.Context,
	req *connect.Request[improvementsv1.GetImprovementsByPartyIdRequest],
) (*connect.Response[improvementsv1.GetImprovementsByPartyIdResponse], error) {
	s.logger.Debug("received GetImprovementsByPartyId request")

	at := getTargetTime(req.Msg.LegalAsOf)
	read := func(afterID *string, limit int) ([]*improvementsv1.PartyImprovement, error) {
		// The party itself must be visible at t; the improvement must pass rule 1.
		rows, err := s.db.Query(ctx, `
			SELECT i.public_id::text, ip.ownership_share::text
			FROM core.parties pt
			JOIN core.party_attributes pa ON pa.party_id = pt.party_id AND pa.legal_valid_range @> $1::timestamptz
			JOIN core.improvement_parties ip ON ip.party_id = pt.party_id AND ip.legal_valid_range @> $1::timestamptz
			JOIN core.improvements i ON i.improvement_id = ip.improvement_id AND NOT i.is_voided
			JOIN core.improvement_attributes ia ON ia.improvement_id = i.improvement_id AND ia.legal_valid_range @> $1::timestamptz
			WHERE pt.public_id = $2::uuid AND NOT pt.is_voided
			  AND `+improvementOnVisibleParcelSQL+`
			  AND ($3::uuid IS NULL OR i.public_id > $3::uuid)
			ORDER BY i.public_id
			LIMIT $4`, at, req.Msg.PartyId, afterID, limit)
		if err != nil {
			return nil, s.dbError("party improvements", err)
		}
		return collectRows(rows, func(r pgx.Rows) (*improvementsv1.PartyImprovement, error) {
			n := &improvementsv1.PartyImprovement{}
			return n, r.Scan(&n.ImprovementId, &n.OwnershipShare)
		}, func(err error) error { return s.dbError("party improvements", err) })
	}

	nodes, pageInfo, err := pageByID("GetImprovementsByPartyId", req.Msg.PartyId, req.Msg.LegalAsOf,
		req.Msg.First, req.Msg.After, read, func(n *improvementsv1.PartyImprovement) string { return n.ImprovementId })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&improvementsv1.GetImprovementsByPartyIdResponse{Improvements: nodes, PageInfo: pageInfo}), nil
}
