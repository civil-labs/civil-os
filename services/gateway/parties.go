package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	publicpartiesv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parties/v1"
	meshpartiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
	meshpartiesv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1/partiesv1connect"
)

type PartyServer struct {
	dbReaderClient meshpartiesv1connect.PartiesReadServiceClient
	logger         *slog.Logger
}

func (s *PartyServer) GetPartiesById(
	ctx context.Context,
	req *connect.Request[publicpartiesv1.GetPartiesByIdRequest],
) (*connect.Response[publicpartiesv1.GetPartiesByIdResponse], error) {
	s.logger.Debug("received GetPartiesById request", slog.Int("ids", len(req.Msg.PartyIds)))

	meshRes, err := s.dbReaderClient.GetPartiesById(ctx, connect.NewRequest(&meshpartiesv1.GetPartiesByIdRequest{
		PartyIds:  req.Msg.PartyIds,
		LegalAsOf: req.Msg.LegalAsOf,
	}))
	if err != nil {
		s.logger.Error("upstream GetPartiesById request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	parties := make(map[string]*publicpartiesv1.Party, len(meshRes.Msg.Parties))
	for id, p := range meshRes.Msg.Parties {
		parties[id] = mapParty(p)
	}
	return connect.NewResponse(&publicpartiesv1.GetPartiesByIdResponse{Parties: parties}), nil
}

func mapParty(m *meshpartiesv1.Party) *publicpartiesv1.Party {
	return &publicpartiesv1.Party{
		PartyId:          m.GetPartyId(),
		Name:             m.GetName(),
		MailingAddressId: m.MailingAddressId,
		MailingAddress:   m.MailingAddress,
	}
}

func (s *PartyServer) GetPartiesByAddressId(
	ctx context.Context,
	req *connect.Request[publicpartiesv1.GetPartiesByAddressIdRequest],
) (*connect.Response[publicpartiesv1.GetPartiesByAddressIdResponse], error) {
	s.logger.Debug("received GetPartiesByAddressId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetPartiesByAddressId(ctx, connect.NewRequest(&meshpartiesv1.GetPartiesByAddressIdRequest{
		AddressId: m.AddressId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetPartiesByAddressId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicpartiesv1.GetPartiesByAddressIdResponse{
		Parties:  mapSlice(meshRes.Msg.Parties, mapParty),
		PageInfo: mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}
