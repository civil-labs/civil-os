package main

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/types/dynamicpb"

	publicimprovementsv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1/improvementsv1connect"
	publicparcelsv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1/parcelsv1connect"
	publicpartiesv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/parties/v1/partiesv1connect"
	meshimprovementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	meshparcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	meshpartiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
)

// TestMapAddressPartyResults: the paginated lookups' results are copied
// through field by field (docs/pagination-without-edges/spec.md, "Gateway").
func TestMapAddressPartyResults(t *testing.T) {
	checkFullMapping(t, &meshparcelsv1.AddressParcel{}, mapAddressParcel)
	checkFullMapping(t, &meshimprovementsv1.AddressImprovement{}, mapAddressImprovement)
	checkFullMapping(t, &meshpartiesv1.Party{}, mapParty)
	checkFullMapping(t, &meshparcelsv1.PartyParcel{}, mapPartyParcel)
}

// TestAddressPartyPageSizeDefault: a paginated reverse lookup (or parcel
// search) sent without first reaches the mesh with the default of 25. The
// lookups live on the service of what they return
// (docs/reverse-lookups-by-returned-type/spec.md).
func TestAddressPartyPageSizeDefault(t *testing.T) {
	g := startGateway(t, nil)
	var tested []string
	for _, service := range []string{
		publicparcelsv1connect.ParcelsServiceName,
		publicimprovementsv1connect.ImprovementsServiceName,
		publicpartiesv1connect.PartiesServiceName,
	} {
		for _, md := range methods(t, service) {
			fd := md.Input().Fields().ByName("first")
			if fd == nil {
				continue
			}
			tested = append(tested, string(md.Name()))
			t.Run(string(md.Name()), func(t *testing.T) {
				req := dynamicpb.NewMessage(md.Input())
				fillValid(req, 3)
				req.Clear(fd)
				req.Clear(md.Input().Fields().ByName("after"))

				g.mesh.reset(0)
				if _, err := g.call(t, md, req, nil); err != nil {
					t.Fatal(err)
				}
				if got := meshFirst(t, g); got != 25 {
					t.Errorf("unset first reached the mesh as %d, want 25", got)
				}
			})
		}
	}
	slices.Sort(tested)
	want := []string{
		"GetImprovementsByAddressId",
		"GetParcelsByAddressId",
		"GetParcelsByPartyId",
		"GetPartiesByAddressId",
		"SearchParcelsAddresses",
	}
	if !slices.Equal(tested, want) {
		t.Errorf("tested paginated RPCs %v, want %v", tested, want)
	}
}
