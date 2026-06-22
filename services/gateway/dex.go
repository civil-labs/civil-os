package main

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	dexv1 "github.com/civil-labs/civil-api-go/civil/public/dex/v1"

	"github.com/dexidp/dex/api/v2"
)

type DexServer struct {
	dexClient api.DexClient
	logger    *slog.Logger
}

func (s *DexServer) GetClient(
	ctx context.Context,
	req *connect.Request[dexv1.GetClientRequest],
) (*connect.Response[dexv1.GetClientResponse], error) {
	s.logger.Debug("Received GetClient request")

	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}

	dexReq := &api.GetClientReq{
		Id: req.Msg.Id,
	}

	dexRes, err := s.dexClient.GetClient(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex GetClient request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	res := &dexv1.GetClientResponse{}
	if dexRes.Client != nil {
		res.Client = &dexv1.Client{
			Id:           dexRes.Client.Id,
			Secret:       dexRes.Client.Secret, // Perhaps drop this in the future. Returning the secret does not seem like great prod practice
			RedirectUris: dexRes.Client.RedirectUris,
			TrustedPeers: dexRes.Client.TrustedPeers,
			Public:       dexRes.Client.Public,
			Name:         dexRes.Client.Name,
			LogoUrl:      dexRes.Client.LogoUrl,
		}
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) CreateClient(
	ctx context.Context,
	req *connect.Request[dexv1.CreateClientRequest],
) (*connect.Response[dexv1.CreateClientResponse], error) {
	s.logger.Debug("Received CreateClient request")

	if req == nil || req.Msg == nil || req.Msg.Client == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client configuration is required"))
	}

	dexReq := &api.CreateClientReq{
		Client: &api.Client{
			Id:           req.Msg.Client.Id,
			Secret:       req.Msg.Client.Secret,
			RedirectUris: req.Msg.Client.RedirectUris,
			TrustedPeers: req.Msg.Client.TrustedPeers,
			Public:       req.Msg.Client.Public,
			Name:         req.Msg.Client.Name,
			LogoUrl:      req.Msg.Client.LogoUrl,
		},
	}

	dexRes, err := s.dexClient.CreateClient(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex CreateClient request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	res := &dexv1.CreateClientResponse{
		AlreadyExists: dexRes.AlreadyExists,
	}
	if dexRes.Client != nil {
		res.Client = &dexv1.Client{
			Id:           dexRes.Client.Id,
			Secret:       dexRes.Client.Secret, // Perhaps drop this in the future. Returning the secret does not seem like great prod practice
			RedirectUris: dexRes.Client.RedirectUris,
			TrustedPeers: dexRes.Client.TrustedPeers,
			Public:       dexRes.Client.Public,
			Name:         dexRes.Client.Name,
			LogoUrl:      dexRes.Client.LogoUrl,
		}
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) UpdateClient(
	ctx context.Context,
	req *connect.Request[dexv1.UpdateClientRequest],
) (*connect.Response[dexv1.UpdateClientResponse], error) {
	s.logger.Debug("Received UpdateClient request")

	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}

	dexReq := &api.UpdateClientReq{
		Id:           req.Msg.Id,
		RedirectUris: req.Msg.RedirectUris,
		TrustedPeers: req.Msg.TrustedPeers,
		Name:         req.Msg.Name,
		LogoUrl:      req.Msg.LogoUrl,
	}

	dexRes, err := s.dexClient.UpdateClient(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex UpdateClient request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	res := &dexv1.UpdateClientResponse{
		NotFound: dexRes.NotFound,
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) DeleteClient(
	ctx context.Context,
	req *connect.Request[dexv1.DeleteClientRequest],
) (*connect.Response[dexv1.DeleteClientResponse], error) {
	s.logger.Debug("Received DeleteClient request")

	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}

	dexReq := &api.DeleteClientReq{
		Id: req.Msg.Id,
	}

	dexRes, err := s.dexClient.DeleteClient(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex DeleteClient request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	res := &dexv1.DeleteClientResponse{
		NotFound: dexRes.NotFound,
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) ListClients(
	ctx context.Context,
	req *connect.Request[dexv1.ListClientsRequest],
) (*connect.Response[dexv1.ListClientsResponse], error) {
	s.logger.Debug("Received ListClients request")

	dexReq := &api.ListClientReq{}

	dexRes, err := s.dexClient.ListClients(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex ListClients request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	clients := make([]*dexv1.ClientInfo, 0, len(dexRes.Clients))

	for _, client := range dexRes.Clients {
		if client == nil {
			continue
		}

		newClient := &dexv1.ClientInfo{
			Id:           client.Id,
			RedirectUris: client.RedirectUris,
			TrustedPeers: client.TrustedPeers,
			Public:       client.Public,
			Name:         client.Name,
			LogoUrl:      client.LogoUrl,
		}

		clients = append(clients, newClient)
	}

	res := &dexv1.ListClientsResponse{
		Clients: clients,
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) ListRefresh(
	ctx context.Context,
	req *connect.Request[dexv1.ListRefreshRequest],
) (*connect.Response[dexv1.ListRefreshResponse], error) {
	s.logger.Debug("Received ListRefresh request")

	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}

	dexReq := &api.ListRefreshReq{
		UserId: req.Msg.UserId,
	}

	dexRes, err := s.dexClient.ListRefresh(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex ListRefresh request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	refreshTokens := make([]*dexv1.RefreshTokenRef, 0, len(dexRes.RefreshTokens))

	for _, refreshToken := range dexRes.RefreshTokens {
		if refreshToken == nil {
			continue
		}

		newRefreshToken := &dexv1.RefreshTokenRef{
			Id:        refreshToken.Id,
			ClientId:  refreshToken.ClientId,
			CreatedAt: refreshToken.CreatedAt,
			LastUsed:  refreshToken.LastUsed,
		}

		refreshTokens = append(refreshTokens, newRefreshToken)
	}

	res := &dexv1.ListRefreshResponse{
		RefreshTokens: refreshTokens,
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) RevokeRefresh(
	ctx context.Context,
	req *connect.Request[dexv1.RevokeRefreshRequest],
) (*connect.Response[dexv1.RevokeRefreshResponse], error) {
	s.logger.Debug("Received RevokeRefresh request")

	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}

	dexReq := &api.RevokeRefreshReq{
		UserId:   req.Msg.UserId,
		ClientId: req.Msg.ClientId,
	}

	dexRes, err := s.dexClient.RevokeRefresh(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex RevokeRefresh request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if dexRes == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("empty response from upstream dex"))
	}

	res := &dexv1.RevokeRefreshResponse{
		NotFound: dexRes.NotFound,
	}

	return connect.NewResponse(res), nil
}
