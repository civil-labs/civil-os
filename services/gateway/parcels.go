package main

import (
	"context"
	"log/slog"

	publicparcelsv1 "github.com/civil-labs/civil-api-go/civil/public/parcels/v1"

	meshparcelsv1 "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1"
	meshparcelsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
)

type ParcelServer struct {
	dbReaderClient meshparcelsv1connect.ParcelsServiceClient
	logger         *slog.Logger
}

func (s *ParcelServer) GetParcelsById(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.GetParcelsByIdRequest],
) (*connect.Response[publicparcelsv1.GetParcelsByIdResponse], error) {

	s.logger.Debug("received GetParcelsById request")

	meshReq := connect.NewRequest(&meshparcelsv1.GetParcelsByIdRequest{
		ParcelIds:                req.Msg.ParcelIds,
		LegalAsOf:                req.Msg.LegalAsOf,
		SystemAsOf:               req.Msg.SystemAsOf,
		NeighborhoodDefinitionId: req.Msg.NeighborhoodDefinitionId,
	})

	meshRes, err := s.dbReaderClient.GetParcelsById(ctx, meshReq)
	if err != nil {
		// ConnectRPC automatically handles wrapping standard gRPC error codes
		// You might want to log the internal error here, but return a sanitized error to the public client
		s.logger.Error("upstream GetParcelsById request failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Initialize a new map for the public response, sized to match the incoming data
	publicParcels := make(map[string]*publicparcelsv1.Parcel, len(meshRes.Msg.Parcels))

	// Iterate and manually map the fields
	for id, meshParcel := range meshRes.Msg.Parcels {
		if meshParcel == nil {
			continue // Always good practice to guard against nil pointers in protobuf maps
		}

		publicParcels[id] = &publicparcelsv1.Parcel{
			ParcelId:            meshParcel.ParcelId,
			Address:             meshParcel.Address,
			AddressId:           meshParcel.AddressId,
			PrimaryOwnerName:    meshParcel.PrimaryOwnerName,
			PrimaryOwnerAddress: meshParcel.PrimaryOwnerAddress,
			PartyIds:            meshParcel.PartyIds,
			LandUseId:           meshParcel.LandUseId,
			NeighborhoodId:      meshParcel.NeighborhoodId,
			LandAreaSqFt:        meshParcel.LandAreaSqFt,
			FrontageFt:          meshParcel.FrontageFt,
			DepthFt:             meshParcel.DepthFt,
			ZoningIds:           meshParcel.ZoningIds,
			MarketLandValue:     meshParcel.MarketLandValue,
			AssessedLandValue:   meshParcel.AssessedLandValue,

			Affordances: &publicparcelsv1.ParcelAffordances{
				AffordanceIds:           meshParcel.Affordances.AffordanceIds,
				MaxFar:                  meshParcel.Affordances.MaxFar,
				MinLotSizeSqFt:          meshParcel.Affordances.MinLotSizeSqFt,
				MaxHeightFt:             meshParcel.Affordances.MaxHeightFt,
				MaxDwellingUnitsPerAcre: meshParcel.Affordances.MaxDwellingUnitsPerAcre,
				MaxLotCoveragePct:       meshParcel.Affordances.MaxLotCoveragePct,
			},

			ImprovementSummary: &publicparcelsv1.ParcelImprovementsSummary{
				ImprovementIds:           meshParcel.ImprovementSummary.ImprovementIds,
				TotalAreaSqFt:            meshParcel.ImprovementSummary.TotalAreaSqFt,
				TotalBathrooms:           meshParcel.ImprovementSummary.TotalBathrooms,
				TotalBedrooms:            meshParcel.ImprovementSummary.TotalBedrooms,
				TotalUnits:               meshParcel.ImprovementSummary.TotalUnits,
				OldestYearBuilt:          meshParcel.ImprovementSummary.OldestYearBuilt,
				NewestYearBuilt:          meshParcel.ImprovementSummary.NewestYearBuilt,
				WorstConditionId:         meshParcel.ImprovementSummary.WorstConditionId,
				BestConditionId:          meshParcel.ImprovementSummary.BestConditionId,
				MarketImprovementValue:   meshParcel.ImprovementSummary.MarketImprovementValue,
				AssessedImprovementValue: meshParcel.ImprovementSummary.AssessedImprovementValue,
			},

			Properties: meshParcel.Properties,
		}
	}

	publicRes := &publicparcelsv1.GetParcelsByIdResponse{
		Parcels: publicParcels,
	}
	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) UpdateParcel(
	ctx context.Context,
	req *connect.Request[publicparcelsv1.UpdateParcelRequest],
) (*connect.Response[publicparcelsv1.UpdateParcelResponse], error) {

	s.logger.Debug("received UpdateParcel request")

	publicRes := &publicparcelsv1.UpdateParcelResponse{}

	return connect.NewResponse(publicRes), nil
}

func (s *ParcelServer) GetCategoricalParcelAttributeStatsById(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetCategoricalParcelAttributeStatsByIdRequest],
) (*connect.Response[publicparcelsv1.GetCategoricalParcelAttributeStatsByIdResponse], error) {
	res := &publicparcelsv1.GetCategoricalParcelAttributeStatsByIdResponse{
		Mode: "example",
		UniqueValues: map[string]int32{
			"value1": 12,
			"value2": 13,
		},
	}
	return connect.NewResponse(res), nil
}

func (s *ParcelServer) GetNumericalParcelAttributeStatsById(
	_ context.Context,
	req *connect.Request[publicparcelsv1.GetNumericalParcelAttributeStatsByIdRequest],
) (*connect.Response[publicparcelsv1.GetNumericalParcelAttributeStatsByIdResponse], error) {
	res := &publicparcelsv1.GetNumericalParcelAttributeStatsByIdResponse{
		Mode:                    5,
		Minimum:                 1,
		Maximum:                 10,
		Percentile_10:           1,
		Percentile_20:           2,
		Percentile_30:           3,
		Percentile_40:           4,
		Percentile_50:           5,
		Percentile_60:           6,
		Percentile_70:           7,
		Percentile_80:           8,
		Percentile_90:           9,
		Percentile_100:          10,
		Mean:                    5,
		StandardDeviation:       2,
		CoefficientOfDispersion: 3,
	}
	return connect.NewResponse(res), nil
}
