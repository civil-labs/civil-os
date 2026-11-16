package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	publicaddressesv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/addresses/v1"
	meshaddressesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/addresses/v1"
	meshaddressesv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/addresses/v1/addressesv1connect"
)

type AddressServer struct {
	dbReaderClient meshaddressesv1connect.AddressesReadServiceClient
	logger         *slog.Logger
}

func (s *AddressServer) GetAddressesById(
	ctx context.Context,
	req *connect.Request[publicaddressesv1.GetAddressesByIdRequest],
) (*connect.Response[publicaddressesv1.GetAddressesByIdResponse], error) {
	s.logger.Debug("received GetAddressesById request", slog.Int("ids", len(req.Msg.AddressIds)))

	meshRes, err := s.dbReaderClient.GetAddressesById(ctx, connect.NewRequest(&meshaddressesv1.GetAddressesByIdRequest{
		AddressIds: req.Msg.AddressIds,
		LegalAsOf:  req.Msg.LegalAsOf,
	}))
	if err != nil {
		s.logger.Error("upstream GetAddressesById request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	addresses := make(map[string]*publicaddressesv1.Address, len(meshRes.Msg.Addresses))
	for id, a := range meshRes.Msg.Addresses {
		addresses[id] = mapAddress(a)
	}
	return connect.NewResponse(&publicaddressesv1.GetAddressesByIdResponse{Addresses: addresses}), nil
}

func (s *AddressServer) GetParcelsByAddressId(
	ctx context.Context,
	req *connect.Request[publicaddressesv1.GetParcelsByAddressIdRequest],
) (*connect.Response[publicaddressesv1.GetParcelsByAddressIdResponse], error) {
	s.logger.Debug("received GetParcelsByAddressId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetParcelsByAddressId(ctx, connect.NewRequest(&meshaddressesv1.GetParcelsByAddressIdRequest{
		AddressId: m.AddressId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetParcelsByAddressId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicaddressesv1.GetParcelsByAddressIdResponse{
		Edges:    mapSlice(meshRes.Msg.Edges, mapAddressParcelEdge),
		PageInfo: mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func (s *AddressServer) GetImprovementsByAddressId(
	ctx context.Context,
	req *connect.Request[publicaddressesv1.GetImprovementsByAddressIdRequest],
) (*connect.Response[publicaddressesv1.GetImprovementsByAddressIdResponse], error) {
	s.logger.Debug("received GetImprovementsByAddressId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetImprovementsByAddressId(ctx, connect.NewRequest(&meshaddressesv1.GetImprovementsByAddressIdRequest{
		AddressId: m.AddressId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetImprovementsByAddressId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicaddressesv1.GetImprovementsByAddressIdResponse{
		Edges:    mapSlice(meshRes.Msg.Edges, mapAddressImprovementEdge),
		PageInfo: mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func (s *AddressServer) GetPartiesByAddressId(
	ctx context.Context,
	req *connect.Request[publicaddressesv1.GetPartiesByAddressIdRequest],
) (*connect.Response[publicaddressesv1.GetPartiesByAddressIdResponse], error) {
	s.logger.Debug("received GetPartiesByAddressId request")

	m := req.Msg
	meshRes, err := s.dbReaderClient.GetPartiesByAddressId(ctx, connect.NewRequest(&meshaddressesv1.GetPartiesByAddressIdRequest{
		AddressId: m.AddressId, LegalAsOf: m.LegalAsOf, First: pageSize(m.First), After: m.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetPartiesByAddressId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publicaddressesv1.GetPartiesByAddressIdResponse{
		Edges:    mapSlice(meshRes.Msg.Edges, mapPartyEdge),
		PageInfo: mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func mapAddress(m *meshaddressesv1.Address) *publicaddressesv1.Address {
	a := &publicaddressesv1.Address{
		AddressId:        m.GetAddressId(),
		FormattedAddress: m.GetFormattedAddress(),
		AddressLine_1:    m.AddressLine_1,
		AddressLine_2:    m.AddressLine_2,
		AddressLine_3:    m.AddressLine_3,
		Locality:         m.Locality,
		Sublocality:      m.Sublocality,
		PostalCode:       m.PostalCode,
		Coordinates:      m.Coordinates, // google.type.LatLng on both sides
	}
	if ar := m.AdministrativeArea; ar != nil {
		a.AdministrativeArea = &publicaddressesv1.AdministrativeArea{Code: ar.GetCode(), Name: ar.GetName()}
	}
	if c := m.Country; c != nil {
		a.Country = &publicaddressesv1.Country{Code: c.GetCode(), Name: c.GetName()}
	}
	return a
}

func mapAddressParcelEdge(m *meshaddressesv1.AddressParcelEdge) *publicaddressesv1.AddressParcelEdge {
	n := m.GetNode()
	return &publicaddressesv1.AddressParcelEdge{Cursor: m.GetCursor(), Node: &publicaddressesv1.AddressParcel{
		ParcelId:  n.GetParcelId(),
		FeatureId: n.FeatureId,
	}}
}

func mapAddressImprovementEdge(m *meshaddressesv1.AddressImprovementEdge) *publicaddressesv1.AddressImprovementEdge {
	n := m.GetNode()
	return &publicaddressesv1.AddressImprovementEdge{Cursor: m.GetCursor(), Node: &publicaddressesv1.AddressImprovement{
		ImprovementId: n.GetImprovementId(),
		ParcelIds:     n.GetParcelIds(),
	}}
}
