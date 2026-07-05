package main

import (
	"context"
	"log/slog"

	publicparcelsv1 "github.com/civil-labs/civil-api-go/civil/public/parcels/v1"

	meshparcelsv1 "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1"
	meshparcelsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
)

type ParcelServer struct {
	dbReaderClient meshparcelsv1connect.ParcelsServiceClient
	logger         *slog.Logger
}

func (s *ParcelServer) GetParcelsWithImprovementSummaryByParcelId(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest],
) (*connect.Response[publicparcelsv1.GetParcelsWithImprovementSummaryByParcelIdResponse], error) {

	s.logger.Debug("received GetParcelsWithImprovementSummaryByParcelId request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetParcelsWithImprovementSummaryByParcelIdRequest{
		ParcelIds:                req.Msg.ParcelIds,
		LegalAsOf:                req.Msg.GetLegalAsOf(),
		ValuationId:              req.Msg.ValuationId,
		NeighborhoodDefinitionId: req.Msg.NeighborhoodDefinitionId,
	})

	meshRes, err := s.dbReaderClient.GetParcelsWithImprovementSummaryByParcelId(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetParcelsWithImprovementSummaryByParcelId request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetParcelsWithImprovementSummaryByParcelIdResponse{
		Parcels: mapParcelWithImprovementSummary(meshRes.Msg.Parcels),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) UpdateParcel(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.UpdateParcelRequest],
) (*connect.Response[publicparcelsv1.UpdateParcelResponse], error) {

	s.logger.Debug("received UpdateParcel request")

	publicRes := &publicparcelsv1.UpdateParcelResponse{}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetCategoricalParcelAttributeStatsById(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetCategoricalParcelAttributeStatsByIdRequest],
) (*connect.Response[publicparcelsv1.GetCategoricalParcelAttributeStatsByIdResponse], error) {
	res := &publicparcelsv1.GetCategoricalParcelAttributeStatsByIdResponse{
		Mode: "example",
		UniqueValues: map[string]int32{
			"value1": 12,
			"value2": 13,
		},
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetNumericalParcelAttributeStatsById(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetNumericalParcelAttributeStatsByIdRequest],
) (*connect.Response[publicparcelsv1.GetNumericalParcelAttributeStatsByIdResponse], error) {
	res := &publicparcelsv1.GetNumericalParcelAttributeStatsByIdResponse{
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

func (s *ParcelServer) GetEquityComparables(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetEquityComparablesRequest],
) (*connect.Response[publicparcelsv1.GetEquityComparablesResponse], error) {

	s.logger.Debug("received GetEquityComparables request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetEquityComparablesRequest{
		WktPolygon:        req.Msg.WktPolygon,
		Criteria:          mapCriteria(req.Msg.Criteria),
		SelectedParcelIds: req.Msg.SelectedParcelIds,
	})

	meshRes, err := s.dbReaderClient.GetEquityComparables(ctx, meshReq)

	if err != nil {
		s.logger.Error("upstream GetEquityComparables request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetEquityComparablesResponse{
		Parcels: mapEquityComparableParcels(meshRes.Msg.Parcels),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetSalesComparables(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetSalesComparablesRequest],
) (*connect.Response[publicparcelsv1.GetSalesComparablesResponse], error) {

	s.logger.Debug("received GetSalesComparables request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetSalesComparablesRequest{
		WktPolygon:        req.Msg.WktPolygon,
		Criteria:          mapCriteria(req.Msg.Criteria),
		SelectedParcelIds: req.Msg.SelectedParcelIds,
		TimeRange:         req.Msg.TimeRange,
	})

	meshRes, err := s.dbReaderClient.GetSalesComparables(ctx, meshReq)

	if err != nil {
		s.logger.Error("upstream GetSalesComparables request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetSalesComparablesResponse{
		Parcels: mapSaleComparableParcels(meshRes.Msg.Parcels),
	}

	return connect.NewResponse(publicRes), nil
}

func mapCriteria(publicCriteria []*publicparcelsv1.ComparableCriteria) []*meshparcelsv1.ComparableCriteria {
	if publicCriteria == nil {
		return nil
	}
	meshCriteria := make([]*meshparcelsv1.ComparableCriteria, len(publicCriteria))
	for i, pc := range publicCriteria {
		if pc == nil {
			continue
		}
		meshCriteria[i] = &meshparcelsv1.ComparableCriteria{
			Attribute:             meshparcelsv1.ParcelAttribute(pc.Attribute),
			MinNumericalTolerance: pc.MinNumericalTolerance,
			MaxNumericalTolerance: pc.MaxNumericalTolerance,
			CategoricalTolerance:  pc.CategoricalTolerance,
		}
	}
	return meshCriteria
}

func mapComparableAttributes(meshAttrs []*meshparcelsv1.ComparableAttribute) []*publicparcelsv1.ComparableAttribute {
	if meshAttrs == nil {
		return nil
	}
	publicAttrs := make([]*publicparcelsv1.ComparableAttribute, len(meshAttrs))
	for i, ma := range meshAttrs {
		if ma == nil {
			continue
		}
		publicAttrs[i] = &publicparcelsv1.ComparableAttribute{
			Attribute:        publicparcelsv1.ParcelAttribute(ma.Attribute),
			NumericalValue:   ma.NumericalValue,
			CategoricalValue: ma.CategoricalValue,
		}
	}
	return publicAttrs
}

func mapEquityComparableParcels(meshParcels map[string]*meshparcelsv1.EquityComparableParcel) map[string]*publicparcelsv1.EquityComparableParcel {
	if meshParcels == nil {
		return nil
	}
	publicParcels := make(map[string]*publicparcelsv1.EquityComparableParcel, len(meshParcels))
	for id, mp := range meshParcels {
		if mp == nil {
			continue
		}
		publicParcels[id] = &publicparcelsv1.EquityComparableParcel{
			ParcelId:         mp.ParcelId,
			FeatureId:        mp.FeatureId,
			AddressId:        mp.AddressId,
			FormattedAddress: mp.FormattedAddress,
			Attributes:       mapComparableAttributes(mp.Attributes),
		}
	}
	return publicParcels
}

func mapSaleComparableParcels(meshParcels map[string]*meshparcelsv1.SaleComparableParcel) map[string]*publicparcelsv1.SaleComparableParcel {
	if meshParcels == nil {
		return nil
	}
	publicParcels := make(map[string]*publicparcelsv1.SaleComparableParcel, len(meshParcels))
	for id, mp := range meshParcels {
		if mp == nil {
			continue
		}
		publicParcels[id] = &publicparcelsv1.SaleComparableParcel{
			ParcelId:         mp.ParcelId,
			FeatureId:        mp.FeatureId,
			AddressId:        mp.AddressId,
			FormattedAddress: mp.FormattedAddress,
			SaleTime:         mp.SaleTime,
			SalePrice:        mp.SalePrice,
			Attributes:       mapComparableAttributes(mp.Attributes),
		}
	}
	return publicParcels
}

func (s *ParcelServer) GetParcelsWithImprovementSummaryByFeatureId(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetParcelsWithImprovementSummaryByFeatureIdRequest],
) (*connect.Response[publicparcelsv1.GetParcelsWithImprovementSummaryByFeatureIdResponse], error) {

	s.logger.Debug("received GetParcelsWithImprovementSummaryByFeatureId request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetParcelsWithImprovementSummaryByFeatureIdRequest{
		FeatureIds:               req.Msg.FeatureIds,
		LegalAsOf:                req.Msg.GetLegalAsOf(),
		ValuationId:              req.Msg.ValuationId,
		NeighborhoodDefinitionId: req.Msg.NeighborhoodDefinitionId,
	})

	meshRes, err := s.dbReaderClient.GetParcelsWithImprovementSummaryByFeatureId(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetParcelsWithImprovementSummaryByFeatureId request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetParcelsWithImprovementSummaryByFeatureIdResponse{
		Parcels: mapParcelWithImprovementSummary(meshRes.Msg.Parcels),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetParcelIdsByFeatureId(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetParcelIdsByFeatureIdRequest],
) (*connect.Response[publicparcelsv1.GetParcelIdsByFeatureIdResponse], error) {

	s.logger.Debug("received GetParcelIdsByFeatureId request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetParcelIdsByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
	})

	meshRes, err := s.dbReaderClient.GetParcelIdsByFeatureId(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetParcelIdsByFeatureId request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetParcelIdsByFeatureIdResponse{
		ParcelIds: meshRes.Msg.ParcelIds,
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetEstimatedParcelsExtentWGS84(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetEstimatedParcelsExtentWGS84Request],
) (*connect.Response[publicparcelsv1.GetEstimatedParcelsExtentWGS84Response], error) {

	s.logger.Debug("received GetEstimatedParcelsExtentWGS84 request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetEstimatedParcelsExtentWGS84Request{})

	meshRes, err := s.dbReaderClient.GetEstimatedParcelsExtentWGS84(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetEstimatedParcelsExtentWGS84 request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicparcelsv1.GetEstimatedParcelsExtentWGS84Response{
		MinX: meshRes.Msg.MinX,
		MinY: meshRes.Msg.MinY,
		MaxX: meshRes.Msg.MaxX,
		MaxY: meshRes.Msg.MaxY,
	}

	return connect.NewResponse(publicRes), nil
}

func mapParcelWithImprovementSummary(meshParcels map[string]*meshparcelsv1.ParcelWithImprovementSummary) map[string]*publicparcelsv1.ParcelWithImprovementSummary {
	if meshParcels == nil {
		return nil
	}
	publicParcels := make(map[string]*publicparcelsv1.ParcelWithImprovementSummary, len(meshParcels))
	for id, mp := range meshParcels {
		if mp == nil {
			continue
		}

		var publicDetails *publicparcelsv1.ParcelDetails
		if mp.ParcelDetails != nil {
			md := mp.ParcelDetails
			publicDetails = &publicparcelsv1.ParcelDetails{
				ParcelId:            md.ParcelId,
				FeatureId:           md.FeatureId,
				FormattedAddress:    md.FormattedAddress,
				AddressId:           md.AddressId,
				PrimaryOwnerName:    md.PrimaryOwnerName,
				PrimaryOwnerAddress: md.PrimaryOwnerAddress,
				PartyIds:            md.PartyIds,
				LandUseId:           md.LandUseId,
				NeighborhoodId:      md.NeighborhoodId,
				LandAreaSqFt:        md.LandAreaSqFt,
				FrontageFt:          md.FrontageFt,
				DepthFt:             md.DepthFt,
				ZoningIds:           md.ZoningIds,
				MarketLandValue:     md.MarketLandValue,
				AssessedLandValue:   md.AssessedLandValue,
				Properties:          md.Properties,
			}
		}

		var publicSummary *publicparcelsv1.ParcelImprovementsSummary
		if mp.ImprovementSummary != nil {
			ms := mp.ImprovementSummary
			publicSummary = &publicparcelsv1.ParcelImprovementsSummary{
				ImprovementIds:                ms.ImprovementIds,
				PrimaryImprovementId:           ms.PrimaryImprovementId,
				TotalAreaSqFt:                  ms.TotalAreaSqFt,
				TotalBathrooms:                 ms.TotalBathrooms,
				TotalBedrooms:                  ms.TotalBedrooms,
				TotalUnits:                     ms.TotalUnits,
				PrimaryYearBuilt:               ms.PrimaryYearBuilt,
				PrimaryEffectiveYearBuilt:      ms.PrimaryEffectiveYearBuilt,
				PrimaryConditionId:             ms.PrimaryConditionId,
				TotalMarketImprovementValue:   ms.TotalMarketImprovementValue,
				TotalAssessedImprovementValue: ms.TotalAssessedImprovementValue,
			}
		}

		publicParcels[id] = &publicparcelsv1.ParcelWithImprovementSummary{
			ParcelDetails:      publicDetails,
			ImprovementSummary: publicSummary,
		}
	}
	return publicParcels
}

func (s *ParcelServer) GetLandAreaSqftByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetLandAreaSqftByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetLandAreaSqftByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetLandAreaSqftByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetLandAreaSqftByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetLandAreaSqftByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetFrontageFtByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetFrontageFtByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetFrontageFtByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetFrontageFtByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetFrontageFtByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetFrontageFtByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetDepthFtByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetDepthFtByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetDepthFtByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetDepthFtByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetDepthFtByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetDepthFtByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetLandUseIdSqftByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetLandUseIdSqftByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetLandUseIdSqftByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetLandUseIdSqftByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetLandUseIdSqftByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetLandUseIdSqftByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetZoningIdByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetZoningIdByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetZoningIdByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetZoningIdByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetZoningIdByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetZoningIdByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetImprovementAreaSqftByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetImprovementAreaSqftByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetImprovementAreaSqftByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetImprovementAreaSqftByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetImprovementAreaSqftByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetImprovementAreaSqftByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetBedroomsByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetBedroomsByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetBedroomsByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetBedroomsByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetBedroomsByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetBedroomsByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetBathroomsByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetBathroomsByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetBathroomsByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetBathroomsByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetBathroomsByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetBathroomsByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetUnitsByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetUnitsByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetUnitsByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetUnitsByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetUnitsByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetUnitsByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementYearBuiltByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementYearBuiltByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementYearBuiltByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementYearBuiltByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementYearBuiltByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementYearBuiltByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementEffectiveYearBuiltByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementEffectiveYearBuiltByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementConditionIdByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementConditionIdByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementConditionIdByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementConditionIdByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementConditionIdByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementConditionIdByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementTypeIdByParcelId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementTypeIdByParcelIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementTypeIdByParcelIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementTypeIdByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		LegalAsOf: req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementTypeIdByParcelId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementTypeIdByParcelIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetLandAreaSqftByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetLandAreaSqftByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetLandAreaSqftByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetLandAreaSqftByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetLandAreaSqftByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetLandAreaSqftByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetFrontageFtByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetFrontageFtByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetFrontageFtByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetFrontageFtByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetFrontageFtByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetFrontageFtByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetDepthFtByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetDepthFtByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetDepthFtByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetDepthFtByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetDepthFtByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetDepthFtByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetLandUseIdSqftByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetLandUseIdSqftByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetLandUseIdSqftByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetLandUseIdSqftByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetLandUseIdSqftByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetLandUseIdSqftByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetZoningIdByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetZoningIdByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetZoningIdByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetZoningIdByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetZoningIdByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetZoningIdByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetImprovementAreaSqftByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetImprovementAreaSqftByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetImprovementAreaSqftByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetImprovementAreaSqftByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetImprovementAreaSqftByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetImprovementAreaSqftByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetBedroomsByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetBedroomsByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetBedroomsByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetBedroomsByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetBedroomsByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetBedroomsByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetBathroomsByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetBathroomsByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetBathroomsByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetBathroomsByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetBathroomsByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetBathroomsByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetUnitsByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetUnitsByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetUnitsByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetUnitsByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetUnitsByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetUnitsByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementYearBuiltByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementYearBuiltByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementYearBuiltByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementYearBuiltByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementYearBuiltByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementYearBuiltByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementEffectiveYearBuiltByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementEffectiveYearBuiltByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementEffectiveYearBuiltByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementConditionIdByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementConditionIdByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementConditionIdByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementConditionIdByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementConditionIdByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementConditionIdByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}

func (s *ParcelServer) GetPrimaryImprovementTypeIdByFeatureId(ctx context.Context, req *connect.Request[publicparcelsv1.GetPrimaryImprovementTypeIdByFeatureIdRequest]) (*connect.Response[publicparcelsv1.GetPrimaryImprovementTypeIdByFeatureIdResponse], error) {
	meshReq := connect.NewRequest(&meshparcelsv1.GetPrimaryImprovementTypeIdByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	})
	meshRes, err := s.dbReaderClient.GetPrimaryImprovementTypeIdByFeatureId(ctx, meshReq)
	if err != nil { return nil, err }
	return connect.NewResponse(&publicparcelsv1.GetPrimaryImprovementTypeIdByFeatureIdResponse{Values: meshRes.Msg.Values}), nil
}
