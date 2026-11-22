package main

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	publicaddressesv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/addresses/v1/addressesv1connect"
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

// TestReverseLookupPlacement: AddressesService only reads addresses, and each
// reverse lookup is served by the public and mesh services of the records it
// returns (docs/reverse-lookups-by-returned-type/spec.md). The routing test
// in handlers_test.go only checks the method name, so it wouldn't notice a
// lookup sent to the wrong mesh service.
func TestReverseLookupPlacement(t *testing.T) {
	var names []string
	for _, md := range methods(t, publicaddressesv1connect.AddressesServiceName) {
		names = append(names, string(md.Name()))
	}
	if !slices.Equal(names, []string{"GetAddressesById"}) {
		t.Errorf("AddressesService methods = %v, want only GetAddressesById", names)
	}

	g := startGateway(t, nil)
	for _, c := range []struct{ service, method, mesh string }{
		{publicparcelsv1connect.ParcelsServiceName, "GetParcelsByAddressId", "civil.mesh.parcels.v1.ParcelReadService"},
		{publicparcelsv1connect.ParcelsServiceName, "GetParcelsByPartyId", "civil.mesh.parcels.v1.ParcelReadService"},
		{publicimprovementsv1connect.ImprovementsServiceName, "GetImprovementsByAddressId", "civil.mesh.improvements.v1.ImprovementReadService"},
		{publicpartiesv1connect.PartiesServiceName, "GetPartiesByAddressId", "civil.mesh.parties.v1.PartiesReadService"},
	} {
		t.Run(c.method, func(t *testing.T) {
			var md protoreflect.MethodDescriptor
			for _, m := range methods(t, c.service) {
				if string(m.Name()) == c.method {
					md = m
				}
			}
			if md == nil {
				t.Fatalf("%s has no method %s", c.service, c.method)
			}
			req := dynamicpb.NewMessage(md.Input())
			fillValid(req, 3)
			req.Clear(md.Input().Fields().ByName("after"))
			g.mesh.reset(0)
			if _, err := g.call(t, md, req, nil); err != nil {
				t.Fatal(err)
			}
			calls := g.mesh.reset(0)
			if want := "/" + c.mesh + "/" + c.method; len(calls) != 1 || calls[0].procedure != want {
				t.Errorf("mesh calls = %v, want one to %s", calls, want)
			}
		})
	}
}
