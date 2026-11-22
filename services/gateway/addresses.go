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
