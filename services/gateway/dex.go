package main

import (
	"context"
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

	dexReq := &api.GetClientReq{
		Id: req.Msg.Id,
	}

	dexRes, err := s.dexClient.GetClient(ctx, dexReq)

	if err != nil {
		s.logger.Error("upstream dex GetClient request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := &dexv1.GetClientResponse{
		Client: &dexv1.Client{
			Id:           dexRes.Client.Id,
			Secret:       dexRes.Client.Secret, // Perhaps drop this in the future. Returning the secret does not seem like great prod practice
			RedirectUris: dexRes.Client.RedirectUris,
			TrustedPeers: dexRes.Client.TrustedPeers,
			Public:       dexRes.Client.Public,
			Name:         dexRes.Client.Name,
			LogoUrl:      dexRes.Client.LogoUrl,
		},
	}

	return connect.NewResponse(res), nil
}

func (s *DexServer) CreateClient(
	ctx context.Context,
	req *connect.Request[dexv1.CreateClientRequest],
) (*connect.Response[dexv1.CreateClientResponse], error) {
	s.logger.Debug("Received CreateClient request")

	res := &dexv1.CreateClientResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) UpdateClient(
	ctx context.Context,
	req *connect.Request[dexv1.UpdateClientRequest],
) (*connect.Response[dexv1.UpdateClientResponse], error) {
	s.logger.Debug("Received UpdateClient request")

	res := &dexv1.UpdateClientResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) DeleteClient(
	ctx context.Context,
	req *connect.Request[dexv1.DeleteClientRequest],
) (*connect.Response[dexv1.DeleteClientResponse], error) {
	s.logger.Debug("Received DeleteClient request")

	res := &dexv1.DeleteClientResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) ListClients(
	ctx context.Context,
	req *connect.Request[dexv1.ListClientsRequest],
) (*connect.Response[dexv1.ListClientsResponse], error) {
	s.logger.Debug("Received ListClients request")

	res := &dexv1.ListClientsResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) GetVersion(
	ctx context.Context,
	req *connect.Request[dexv1.GetVersionRequest],
) (*connect.Response[dexv1.GetVersionResponse], error) {
	s.logger.Debug("Received GetVersion request")

	res := &dexv1.GetVersionResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) ListRefresh(
	ctx context.Context,
	req *connect.Request[dexv1.ListRefreshRequest],
) (*connect.Response[dexv1.ListRefreshResponse], error) {
	s.logger.Debug("Received ListRefresh request")

	res := &dexv1.ListRefreshResponse{}

	return connect.NewResponse(res), nil
}

func (s *DexServer) RevokeRefresh(
	ctx context.Context,
	req *connect.Request[dexv1.RevokeRefreshRequest],
) (*connect.Response[dexv1.RevokeRefreshResponse], error) {
	s.logger.Debug("Received RevokeRefresh request")

	res := &dexv1.RevokeRefreshResponse{}

	return connect.NewResponse(res), nil
}
