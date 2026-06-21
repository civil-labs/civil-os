package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	meshlandusesv1 "github.com/civil-labs/civil-api-go/civil/mesh/landuses/v1"
	meshlandusesv1connect "github.com/civil-labs/civil-api-go/civil/mesh/landuses/v1/landusesv1connect"
	publiclandusesv1 "github.com/civil-labs/civil-api-go/civil/public/landuses/v1"
)

type LandUseServer struct {
	dbReaderClient meshlandusesv1connect.LandUsesServiceClient
	logger         *slog.Logger
}

func (s *LandUseServer) GetLandUses(
	ctx context.Context,
	req *connect.Request[publiclandusesv1.GetLandUsesRequest],
) (*connect.Response[publiclandusesv1.GetLandUsesResponse], error) {
	s.logger.Debug("received GetLandUses request")

	meshReq := connect.NewRequest(&meshlandusesv1.GetLandUsesRequest{})
	meshRes, err := s.dbReaderClient.GetLandUses(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetLandUses request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publiclandusesv1.GetLandUsesResponse{
		LandUses: mapLandUses(meshRes.Msg.LandUses),
	}

	return connect.NewResponse(publicRes), nil
}

func (s *LandUseServer) GetLandUseTypes(
	ctx context.Context,
	req *connect.Request[publiclandusesv1.GetLandUseTypesRequest],
) (*connect.Response[publiclandusesv1.GetLandUseTypesResponse], error) {
	s.logger.Debug("received GetLandUseTypes request")

	meshReq := connect.NewRequest(&meshlandusesv1.GetLandUseTypesRequest{})
	meshRes, err := s.dbReaderClient.GetLandUseTypes(ctx, meshReq)
	if err != nil {
		s.logger.Error("upstream GetLandUseTypes request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	publicRes := &publiclandusesv1.GetLandUseTypesResponse{
		LandUseTypes: mapLandUseTypes(meshRes.Msg.LandUseTypes),
	}

	return connect.NewResponse(publicRes), nil
}

func mapLandUses(meshLandUses map[string]*meshlandusesv1.LandUse) map[string]*publiclandusesv1.LandUse {
	if meshLandUses == nil {
		return nil
	}
	publicLandUses := make(map[string]*publiclandusesv1.LandUse, len(meshLandUses))
	for k, v := range meshLandUses {
		if v == nil {
			continue
		}
		publicLandUses[k] = &publiclandusesv1.LandUse{
			Id:              v.Id,
			Name:            v.Name,
			Code:            v.Code,
			Description:     v.Description,
			LandUseTypeId:   v.LandUseTypeId,
			LandUseTypeName: v.LandUseTypeName,
		}
	}
	return publicLandUses
}

func mapLandUseTypes(meshTypes map[string]*meshlandusesv1.LandUseType) map[string]*publiclandusesv1.LandUseType {
	if meshTypes == nil {
		return nil
	}
	publicTypes := make(map[string]*publiclandusesv1.LandUseType, len(meshTypes))
	for k, v := range meshTypes {
		if v == nil {
			continue
		}
		publicTypes[k] = &publiclandusesv1.LandUseType{
			Id:          v.Id,
			Name:        v.Name,
			Code:        v.Code,
			Description: v.Description,
		}
	}
	return publicTypes
}
