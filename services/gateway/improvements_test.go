package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	publicimprovementsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1/improvementsv1connect"
	meshimprovementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	"github.com/civil-labs/civil-os/internal/testutil"
)

// Gateway tests for the improvement read API
// (docs/improvement-read-api/spec.md, "Gateway handler tests").

func improvementsClient(g *gateway) improvementsv1connect.ImprovementsServiceClient {
	return improvementsv1connect.NewImprovementsServiceClient(http.DefaultClient, g.url,
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+g.token)
				return next(ctx, req)
			}
		})))
}

// TestMapImprovementResults: improvement read results are copied through
// field by field.
func TestMapImprovementResults(t *testing.T) {
	checkFullMapping(t, &meshimprovementsv1.ImprovementQuality{}, mapImprovementQuality)
	checkFullMapping(t, &meshimprovementsv1.Improvement{}, mapImprovement)
	checkFullMapping(t, &meshimprovementsv1.ImprovementParcel{}, mapImprovementParcel)
	checkFullMapping(t, &meshimprovementsv1.ImprovementShare{}, mapImprovementShare)
}

// TestImprovementsByIDValidation: invalid requests fail with InvalidArgument
// and never reach the mesh.
func TestImprovementsByIDValidation(t *testing.T) {
	g := startGateway(t, nil)
	c := improvementsClient(g)
	ctx := context.Background()

	one := []string{testutil.ImprovementID(1)}
	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = testutil.ImprovementID(i + 1)
	}
	notUUID := "not-a-uuid"
	beforeYearZero := timestamppb.New(time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC))

	for name, req := range map[string]*publicimprovementsv1.GetImprovementsByIdRequest{
		"no IDs":                  {},
		"101 IDs":                 {ImprovementIds: tooMany},
		"duplicate IDs":           {ImprovementIds: []string{testutil.ImprovementID(1), testutil.ImprovementID(1)}},
		"non-UUID ID":             {ImprovementIds: []string{notUUID}},
		"non-UUID valuation":      {ImprovementIds: one, ValuationId: &notUUID},
		"legal_as_of before 0 CE": {ImprovementIds: one, LegalAsOf: beforeYearZero},
	} {
		g.mesh.reset(0)
		_, err := c.GetImprovementsById(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
		if calls := g.mesh.reset(0); len(calls) != 0 {
			t.Errorf("%s: invalid request reached the mesh", name)
		}
	}
}
