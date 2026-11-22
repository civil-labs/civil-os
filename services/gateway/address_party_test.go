package main

import (
	"testing"

	"google.golang.org/protobuf/types/dynamicpb"

	publicaddressesv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/addresses/v1/addressesv1connect"
	publicpartiesv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/parties/v1/partiesv1connect"
	meshaddressesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/addresses/v1"
	meshpartiesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parties/v1"
)

// TestMapAddressPartyResults: the paginated lookups' results are copied
// through field by field (docs/pagination-without-edges/spec.md, "Gateway").
func TestMapAddressPartyResults(t *testing.T) {
	checkFullMapping(t, &meshaddressesv1.AddressParcel{}, mapAddressParcel)
	checkFullMapping(t, &meshaddressesv1.AddressImprovement{}, mapAddressImprovement)
	checkFullMapping(t, &meshpartiesv1.Party{}, mapParty)
	checkFullMapping(t, &meshpartiesv1.PartyParcel{}, mapPartyParcel)
}

// TestAddressPartyPageSizeDefault: a paginated address or party lookup sent
// without first reaches the mesh with the default of 25.
func TestAddressPartyPageSizeDefault(t *testing.T) {
	g := startGateway(t, nil)
	tested := 0
	for _, service := range []string{publicaddressesv1connect.AddressesServiceName, publicpartiesv1connect.PartiesServiceName} {
		for _, md := range methods(t, service) {
			fd := md.Input().Fields().ByName("first")
			if fd == nil {
				continue
			}
			tested++
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
	if tested != 4 {
		t.Errorf("tested %d paginated RPCs, want 4", tested)
	}
}
