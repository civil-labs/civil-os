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
