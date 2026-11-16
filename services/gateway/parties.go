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

func (s *PartyServer) GetParcelsByPartyId(
	ctx context.Context,
	req *connect.Request[publicpartiesv1.GetParcelsByPartyIdRequest],
) (*connect.Response[publicpartiesv1.GetParcelsByPartyIdResponse], error) {
	s.logger.Debug("received GetParcelsByPartyId request")

	meshRes, err := s.dbReaderClient.GetParcelsByPartyId(ctx, connect.NewRequest(&meshpartiesv1.GetParcelsByPartyIdRequest{
		PartyId:   req.Msg.PartyId,
		LegalAsOf: req.Msg.LegalAsOf,
		First:     pageSize(req.Msg.First),
		After:     req.Msg.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetParcelsByPartyId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicpartiesv1.GetParcelsByPartyIdResponse{
		Edges:    mapSlice(meshRes.Msg.Edges, mapPartyParcelEdge),
		PageInfo: mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func mapParty(m *meshpartiesv1.Party) *publicpartiesv1.Party {
	return &publicpartiesv1.Party{
		PartyId:          m.GetPartyId(),
		Name:             m.GetName(),
		MailingAddressId: m.MailingAddressId,
		MailingAddress:   m.MailingAddress,
	}
}

func mapPartyEdge(m *meshpartiesv1.PartyEdge) *publicpartiesv1.PartyEdge {
	return &publicpartiesv1.PartyEdge{Cursor: m.GetCursor(), Node: mapParty(m.GetNode())}
}

func mapPartyParcelEdge(m *meshpartiesv1.PartyParcelEdge) *publicpartiesv1.PartyParcelEdge {
	n := m.GetNode()
	return &publicpartiesv1.PartyParcelEdge{Cursor: m.GetCursor(), Node: &publicpartiesv1.PartyParcel{
		ParcelId:       n.GetParcelId(),
		FeatureId:      n.FeatureId,
		OwnershipShare: n.GetOwnershipShare(),
	}}
}
