// Package testutil holds shared test helpers: the synthetic fixture IDs,
// a PostGIS test database (testcontainers) and a fake OIDC issuer.
package testutil

import "fmt"

// Fixture public IDs follow <prefix>-0000-4000-8000-<n, 12 digits>; see the
// header of database/fixtures/synthetic.sql for the shape of the data.
const (
	LandUseTypePrefix          = "10000000"
	LandUsePrefix              = "11000000"
	AddressPrefix              = "20000000"
	PartyPrefix                = "30000000"
	ZoningPrefix               = "40000000"
	AffordanceTypePrefix       = "41000000"
	ParcelAffordancePrefix     = "42000000"
	ImprovementTypePrefix      = "50000000"
	ImprovementConditionPrefix = "51000000"
	ImprovementQualityPrefix   = "52000000"
	ImprovementPrefix          = "53000000"
	ValuationPrefix            = "60000000"
	NeighborhoodDefPrefix      = "70000000"
	NeighborhoodPrefix         = "71000000"
	ParcelPrefix               = "a0000000"
)

// Counts and special rows in the fixture set.
const (
	ParcelCount        = 20 // parcels 1..20
	ActiveParcelCount  = 19 // parcels 1..19
	VoidedParcel       = 20
	HistoricalParcel   = 1  // has an attributes row valid 2020-01-01..2024-01-01
	MultiOwnerParcel   = 2  // owned 50/50 by parties 2 and 3
	SecondaryImpParcel = 3  // also has secondary improvement 16
	ImprovedParcels    = 15 // parcels 1..15 have primary improvement n
	UnattachedAddress  = 21 // address on no parcel
)

// FixtureID returns the public ID of row n with the given prefix.
func FixtureID(prefix string, n int) string {
	return fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, n)
}

// ParcelID returns the public ID of fixture parcel n.
func ParcelID(n int) string { return FixtureID(ParcelPrefix, n) }

// ParcelFeatureID returns the tile feature ID of fixture parcel n.
func ParcelFeatureID(n int) int64 { return int64(1000 + n) }

// ParcelLandAreaSqM is the current land area of fixture parcel n.
func ParcelLandAreaSqM(n int) float64 { return float64(400 + 10*n) }

// ParcelLandUse returns the land use number (1..3) of fixture parcel n.
func ParcelLandUse(n int) int { return (n-1)%3 + 1 }

// ParcelMarketValue is fixture parcel n's market value in valuation 1.
func ParcelMarketValue(n int) float64 { return float64(100000 + 5000*n) }

// ParcelZoningCode is the base zoning code of fixture parcel n.
func ParcelZoningCode(n int) string {
	if n <= 12 {
		return "R-1"
	}
	return "C-1"
}

// FixtureTile is a z/x/y web-mercator tile that contains every fixture parcel.
var FixtureTile = struct{ Z, X, Y int }{14, 1820, 10181}

// MissingID is a well-formed UUID that matches no fixture row.
const MissingID = "ffffffff-0000-4000-8000-000000000000"
