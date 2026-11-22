package main

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	publicpaginationv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/pagination/v1"
	publicrecordingsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/recordings/v1"
	publictransfersv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/transfers/v1"
	meshpaginationv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/pagination/v1"
	meshrecordingsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/recordings/v1"
	meshtransfersv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1"
	meshtransfersv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/transfers/v1/transfersv1connect"
)

// defaultTransferPageSize is the page size when a request leaves first unset.
// The maximum (100) is a protovalidate rule in the public proto.
const defaultTransferPageSize int32 = 25

type TransferServer struct {
	dbReaderClient meshtransfersv1connect.RealPropertyTransfersReadServiceClient
	logger         *slog.Logger
}

// pageSize applies the default page size. Validation has already rejected
// values outside 1..100.
func pageSize(first *int32) *int32 {
	if first != nil {
		return first
	}
	v := defaultTransferPageSize
	return &v
}

func (s *TransferServer) GetTransfersById(
	ctx context.Context,
	req *connect.Request[publictransfersv1.GetTransfersByIdRequest],
) (*connect.Response[publictransfersv1.GetTransfersByIdResponse], error) {
	s.logger.Debug("received GetTransfersById request", slog.Int("ids", len(req.Msg.TransferIds)))

	meshRes, err := s.dbReaderClient.GetTransfersById(ctx, connect.NewRequest(&meshtransfersv1.GetTransfersByIdRequest{
		TransferIds: req.Msg.TransferIds,
	}))
	if err != nil {
		s.logger.Error("upstream GetTransfersById request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publictransfersv1.GetTransfersByIdResponse{
		Transfers: mapSlice(meshRes.Msg.Transfers, mapTransfer),
	}), nil
}

func (s *TransferServer) GetTransfersByParcelId(
	ctx context.Context,
	req *connect.Request[publictransfersv1.GetTransfersByParcelIdRequest],
) (*connect.Response[publictransfersv1.GetTransfersByParcelIdResponse], error) {
	s.logger.Debug("received GetTransfersByParcelId request", slog.Int("ids", len(req.Msg.ParcelIds)))

	meshRes, err := s.dbReaderClient.GetTransfersByParcelId(ctx, connect.NewRequest(&meshtransfersv1.GetTransfersByParcelIdRequest{
		ParcelIds: req.Msg.ParcelIds,
		TimeRange: req.Msg.TimeRange,
		First:     pageSize(req.Msg.First),
		After:     req.Msg.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetTransfersByParcelId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publictransfersv1.GetTransfersByParcelIdResponse{
		Transfers: mapSlice(meshRes.Msg.Transfers, mapTransfer),
		PageInfo:  mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func (s *TransferServer) GetTransfersByFeatureId(
	ctx context.Context,
	req *connect.Request[publictransfersv1.GetTransfersByFeatureIdRequest],
) (*connect.Response[publictransfersv1.GetTransfersByFeatureIdResponse], error) {
	s.logger.Debug("received GetTransfersByFeatureId request", slog.Int("ids", len(req.Msg.FeatureIds)))

	meshRes, err := s.dbReaderClient.GetTransfersByFeatureId(ctx, connect.NewRequest(&meshtransfersv1.GetTransfersByFeatureIdRequest{
		FeatureIds: req.Msg.FeatureIds,
		TimeRange:  req.Msg.TimeRange,
		First:      pageSize(req.Msg.First),
		After:      req.Msg.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetTransfersByFeatureId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publictransfersv1.GetTransfersByFeatureIdResponse{
		Transfers: mapSlice(meshRes.Msg.Transfers, mapTransfer),
		PageInfo:  mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

func (s *TransferServer) GetTransfersByPartyId(
	ctx context.Context,
	req *connect.Request[publictransfersv1.GetTransfersByPartyIdRequest],
) (*connect.Response[publictransfersv1.GetTransfersByPartyIdResponse], error) {
	s.logger.Debug("received GetTransfersByPartyId request", slog.Int("ids", len(req.Msg.PartyIds)))

	meshRes, err := s.dbReaderClient.GetTransfersByPartyId(ctx, connect.NewRequest(&meshtransfersv1.GetTransfersByPartyIdRequest{
		PartyIds:  req.Msg.PartyIds,
		TimeRange: req.Msg.TimeRange,
		First:     pageSize(req.Msg.First),
		After:     req.Msg.After,
	}))
	if err != nil {
		s.logger.Error("upstream GetTransfersByPartyId request failed", slog.Any("error", err))
		return nil, err // keep the mesh status code
	}
	return connect.NewResponse(&publictransfersv1.GetTransfersByPartyIdResponse{
		Transfers: mapSlice(meshRes.Msg.Transfers, mapTransfer),
		PageInfo:  mapPageInfo(meshRes.Msg.PageInfo),
	}), nil
}

// mapSlice maps each non-nil element of in.
func mapSlice[M, P any](in []*M, f func(*M) *P) []*P {
	if in == nil {
		return nil
	}
	out := make([]*P, 0, len(in))
	for _, m := range in {
		if m != nil {
			out = append(out, f(m))
		}
	}
	return out
}

func mapPageInfo(m *meshpaginationv1.PageInfo) *publicpaginationv1.PageInfo {
	if m == nil {
		return nil
	}
	return &publicpaginationv1.PageInfo{
		HasNextPage:     m.HasNextPage,
		HasPreviousPage: m.HasPreviousPage,
		StartCursor:     m.StartCursor,
		EndCursor:       m.EndCursor,
	}
}

func mapTransfer(m *meshtransfersv1.RealPropertyTransfer) *publictransfersv1.RealPropertyTransfer {
	return &publictransfersv1.RealPropertyTransfer{
		TransferId:     m.TransferId,
		TransferTime:   m.TransferTime,
		TransferAmount: m.TransferAmount,
		Recordings:     mapSlice(m.Recordings, mapRecording),
		Parcels:        mapSlice(m.Parcels, mapTransferredParcel),
		Improvements:   mapSlice(m.Improvements, mapTransferredImprovement),
		Codes:          mapSlice(m.Codes, mapTransferCode),
		Parties:        mapSlice(m.Parties, mapTransferParty),
	}
}

func mapRecording(m *meshrecordingsv1.Recording) *publicrecordingsv1.Recording {
	return &publicrecordingsv1.Recording{Book: m.Book, Page: m.Page, Uri: m.Uri}
}

func mapTransferredParcel(m *meshtransfersv1.TransferredParcel) *publictransfersv1.TransferredParcel {
	return &publictransfersv1.TransferredParcel{
		ParcelId:  m.ParcelId,
		FeatureId: m.FeatureId,
		Grantors:  mapSlice(m.Grantors, mapTransferShare),
		Grantees:  mapSlice(m.Grantees, mapTransferShare),
	}
}

func mapTransferredImprovement(m *meshtransfersv1.TransferredImprovement) *publictransfersv1.TransferredImprovement {
	return &publictransfersv1.TransferredImprovement{
		ImprovementId: m.ImprovementId,
		Grantors:      mapSlice(m.Grantors, mapTransferShare),
		Grantees:      mapSlice(m.Grantees, mapTransferShare),
	}
}

func mapTransferShare(m *meshtransfersv1.TransferShare) *publictransfersv1.TransferShare {
	return &publictransfersv1.TransferShare{PartyId: m.PartyId, TransferredShare: m.TransferredShare}
}

func mapTransferParty(m *meshtransfersv1.TransferParty) *publictransfersv1.TransferParty {
	return &publictransfersv1.TransferParty{PartyId: m.PartyId, Name: m.Name, MailingAddress: m.MailingAddress}
}

func mapTransferCode(m *meshtransfersv1.TransferCode) *publictransfersv1.TransferCode {
	c := &publictransfersv1.TransferCode{CodeId: m.CodeId, Name: m.Name, Description: m.Description}
	if m.Type != nil {
		c.Type = &publictransfersv1.TransferCodeType{
			CodeTypeId:  m.Type.CodeTypeId,
			Name:        m.Type.Name,
			Description: m.Type.Description,
		}
	}
	return c
}
