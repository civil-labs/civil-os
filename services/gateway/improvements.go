package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	meshimprovementsv1 "github.com/civil-labs/civil-api-go/civil/mesh/improvements/v1"
	meshimprovementsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/improvements/v1/improvementsv1connect"
	publicimprovementsv1 "github.com/civil-labs/civil-api-go/civil/public/improvements/v1"
)

type ImprovementServer struct {
	dbReaderClient meshimprovementsv1connect.ImprovementsServiceClient
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
		return nil, connect.NewError(connect.CodeInternal, err)
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
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publicimprovementsv1.GetImprovementConditionsResponse{
		ImprovementConditions: mapImprovementConditions(meshRes.Msg.ImprovementConditions),
	}

	return connect.NewResponse(publicRes), nil
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
