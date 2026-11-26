package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	publicimprovementsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1"
	meshimprovementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	meshimprovementsv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1/improvementsv1connect"
)

type ImprovementServer struct {
	dbReaderClient meshimprovementsv1connect.ImprovementsReadServiceClient
	logger         *slog.Logger
}

func (s *ImprovementServer) GetImprovementTypes(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementTypesRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementTypesResponse], error) {
	s.logger.Debug("received GetImprovementTypes request")

	meshReq := connect.NewRequest(&meshimprovementsv1.GetImprovementTypesRequest{})
	meshRes, err := s.dbReaderClient.GetImprovementTypes(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetImprovementTypes request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	publicRes := &publicimprovementsv1.GetImprovementTypesResponse{
		ImprovementTypes: mapImprovementTypes(meshRes.Msg.ImprovementTypes),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ImprovementServer) GetImprovementConditions(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementConditionsRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementConditionsResponse], error) {
	s.logger.Debug("received GetImprovementConditions request")

	meshReq := connect.NewRequest(&meshimprovementsv1.GetImprovementConditionsRequest{})
	meshRes, err := s.dbReaderClient.GetImprovementConditions(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetImprovementConditions request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	publicRes := &publicimprovementsv1.GetImprovementConditionsResponse{
		ImprovementConditions: mapImprovementConditions(meshRes.Msg.ImprovementConditions),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *ImprovementServer) GetImprovementQualities(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementQualitiesRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementQualitiesResponse], error) {
	s.logger.Debug("received GetImprovementQualities request")

	meshRes, err := s.dbReaderClient.GetImprovementQualities(ctx, connect.NewRequest(&meshimprovementsv1.GetImprovementQualitiesRequest{}))
	if err != nil {
		s.logger.Error("upstream GetImprovementQualities request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}

	qualities := make(map[string]*publicimprovementsv1.ImprovementQuality, len(meshRes.Msg.ImprovementQualities))
	for k, v := range meshRes.Msg.ImprovementQualities {
		if v != nil {
			qualities[k] = mapImprovementQuality(v)
		}
	}
	return connect.NewResponse(&publicimprovementsv1.GetImprovementQualitiesResponse{ImprovementQualities: qualities}), nil
}

func mapImprovementQuality(m *meshimprovementsv1.ImprovementQuality) *publicimprovementsv1.ImprovementQuality {
	return &publicimprovementsv1.ImprovementQuality{
		Id:                        m.Id,
		Name:                      m.Name,
		QualityAdjustmentModifier: m.QualityAdjustmentModifier,
	}
}

func mapImprovementTypes(meshTypes map[string]*meshimprovementsv1.ImprovementType) map[string]*publicimprovementsv1.ImprovementType {
	if meshTypes == nil {
		return nil
	}
	publicTypes := make(map[string]*publicimprovementsv1.ImprovementType, len(meshTypes))
	for k, v := range meshTypes {
		if v == nil {
			continue
		}
		publicTypes[k] = &publicimprovementsv1.ImprovementType{
			Id:          v.Id,
			Name:        v.Name,
			Code:        v.Code,
			Description: v.Description,
		}
	}
	return publicTypes
}

func mapImprovementConditions(meshConditions map[string]*meshimprovementsv1.ImprovementCondition) map[string]*publicimprovementsv1.ImprovementCondition {
	if meshConditions == nil {
		return nil
	}
	publicConditions := make(map[string]*publicimprovementsv1.ImprovementCondition, len(meshConditions))
	for k, v := range meshConditions {
		if v == nil {
			continue
		}
		publicConditions[k] = &publicimprovementsv1.ImprovementCondition{
			Id:                   v.Id,
			Name:                 v.Name,
			DepreciationModifier: v.DepreciationModifier,
		}
	}
	return publicConditions
}

func (s *ImprovementServer) GetImprovementsByAddressId(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementsByAddressIdRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementsByAddressIdResponse], error) {
	s.logger.Debug("received GetImprovementsByAddressId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetImprovementsByAddressId(ctx, connect.NewRequest(&meshimprovementsv1.GetImprovementsByAddressIdRequest{
		AddressId: m.AddressId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetImprovementsByAddressId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicimprovementsv1.GetImprovementsByAddressIdResponse{
		Improvements: mapSlice(meshRes.Msg.Improvements, mapAddressImprovement),
		PageInfo:     mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func mapAddressImprovement(m *meshimprovementsv1.AddressImprovement) *publicimprovementsv1.AddressImprovement {
	return &publicimprovementsv1.AddressImprovement{
		ImprovementId: m.GetImprovementId(),
		ParcelIds:     m.GetParcelIds(),
	}
}

func (s *ImprovementServer) GetImprovementsById(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementsByIdRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementsByIdResponse], error) {
	s.logger.Debug("received GetImprovementsById request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetImprovementsById(ctx, connect.NewRequest(&meshimprovementsv1.GetImprovementsByIdRequest{
		ImprovementIds: m.ImprovementIds, LegalAsOf: m.LegalAsOf, ValuationId: m.ValuationId,
	}))
	if err != nil {
		s.logger.Error("upstream GetImprovementsById request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	improvements := make(map[string]*publicimprovementsv1.Improvement, len(meshRes.Msg.Improvements))
	for k, v := range meshRes.Msg.Improvements {
		if v != nil {
			improvements[k] = mapImprovement(v)
		}
	}
	return connect.NewResponse(&publicimprovementsv1.GetImprovementsByIdResponse{Improvements: improvements}), nil
}

func mapImprovement(m *meshimprovementsv1.Improvement) *publicimprovementsv1.Improvement {
	return &publicimprovementsv1.Improvement{
		ImprovementId:      m.GetImprovementId(),
		TypeId:             m.GetTypeId(),
		ConditionId:        m.ConditionId,
		QualityId:          m.QualityId,
		AddressId:          m.AddressId,
		FormattedAddress:   m.FormattedAddress,
		AreaSqFt:           m.AreaSqFt,
		FullBathrooms:      m.FullBathrooms,
		HalfBathrooms:      m.HalfBathrooms,
		Bathrooms:          m.Bathrooms,
		Bedrooms:           m.Bedrooms,
		Units:              m.Units,
		YearBuilt:          m.YearBuilt,
		EffectiveYearBuilt: m.EffectiveYearBuilt,
		Properties:         m.Properties,
		Parcels:            mapSlice(m.GetParcels(), mapImprovementParcel),
		Parties:            mapSlice(m.GetParties(), mapImprovementShare),
		MarketValue:        m.MarketValue,
		AssessedValue:      m.AssessedValue,
	}
}

func mapImprovementParcel(m *meshimprovementsv1.ImprovementParcel) *publicimprovementsv1.ImprovementParcel {
	return &publicimprovementsv1.ImprovementParcel{
		ParcelId:  m.GetParcelId(),
		FeatureId: m.FeatureId,
		IsPrimary: m.GetIsPrimary(),
	}
}

func mapImprovementShare(m *meshimprovementsv1.ImprovementShare) *publicimprovementsv1.ImprovementShare {
	return &publicimprovementsv1.ImprovementShare{
		PartyId:        m.GetPartyId(),
		OwnershipShare: m.GetOwnershipShare(),
	}
}

func (s *ImprovementServer) GetImprovementsByPartyId(
	ctx context.Context,
	req *connect.Request[publicimprovementsv1.GetImprovementsByPartyIdRequest],
) (*connect.Response[publicimprovementsv1.GetImprovementsByPartyIdResponse], error) {
	s.logger.Debug("received GetImprovementsByPartyId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetImprovementsByPartyId(ctx, connect.NewRequest(&meshimprovementsv1.GetImprovementsByPartyIdRequest{
		PartyId: m.PartyId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetImprovementsByPartyId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicimprovementsv1.GetImprovementsByPartyIdResponse{
		Improvements: mapSlice(meshRes.Msg.Improvements, mapPartyImprovement),
		PageInfo:     mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func mapPartyImprovement(m *meshimprovementsv1.PartyImprovement) *publicimprovementsv1.PartyImprovement {
	return &publicimprovementsv1.PartyImprovement{
		ImprovementId:  m.GetImprovementId(),
		OwnershipShare: m.GetOwnershipShare(),
	}
}
