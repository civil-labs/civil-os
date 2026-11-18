// Package testutil holds shared test helpers: the synthetic fixture IDs,
// a PostGIS test database (testcontainers) and a fake OIDC issuer.
package testutil

import (
	"fmt"
	"time"
)

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
	TransferPrefix             = "90000000"
	TransferCodeTypePrefix     = "91000000"
	TransferCodePrefix         = "92000000"
	ParcelPrefix               = "a0000000"
)

// Counts and special rows in the fixture set.
const (
	ParcelCount        = 21 // parcels 1..21
	ActiveParcelCount  = 20 // parcels 1..19 and 21
	VoidedParcel       = 20
	NoGeometryParcel   = 21 // not voided, but no geometry, so parcel reads never return it
	HistoricalParcel   = 1  // has an attributes row valid 2020-01-01..2024-01-01
	MultiOwnerParcel   = 2  // owned 50/50 by parties 2 and 3
	SecondaryImpParcel = 3  // also has secondary improvement 16
	ImprovedParcels    = 15 // active parcels 1..15 have primary improvement n; voided parcel 20 has improvement 17
	UnattachedAddress  = 21 // address on no parcel
)

// History and voided records for the address and party read API; see the
// header of database/fixtures/synthetic.sql and
// docs/address-party-read-api/spec.md.
const (
	RedrawnParcel                 = 9    // earlier geometry: west half of its square
	RedrawnParcelEarlierFeature   = 1109 // feature ID of that earlier geometry
	VoidedParty                   = 11
	VoidedPartyParcel             = 4  // parcel_parties row for voided party 11, at 0.5
	HistoricalParty               = 8  // earlier name and mailing address until HistoryChange
	HistoricalPartyEarlierAddress = 18 // party 8's mailing address until HistoryChange
	HistoricalAddress             = 5  // earlier line and no coordinates until HistoryChange
)

// HistoryChange is when party 8 and address 5 change; GeometryChange is when
// parcel 9 is redrawn.
var (
	HistoryChange  = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	GeometryChange = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

// Real property transfers; see the header of database/fixtures/synthetic.sql.
const (
	TransferCount          = 7
	MultiGranteeTransfer   = 2 // parcel 2: party 1 -> parties 2 and 3 at 0.5 each
	MultiParcelTransfer    = 3 // parcels 6 and 7 plus improvements 6 and 7
	VoidedOnlyTransfer     = 4 // only voided parcel 20 and improvement 17: never visible
	PartlyVoidedTransfer   = 5 // parcels 19 (two grantors at 0.5 each) and voided 20
	NoRecordingTransfer    = 6 // no amount and no deed columns
	MissingGranteeTransfer = 7 // parcel 8 with a grantor and no grantee
)

// Neighborhoods; see the header of database/fixtures/synthetic.sql and
// docs/neighborhoods-api-fixes/spec.md.
const (
	LegalNeighborhoodDefinition    = 1  // 'Fixture districts'
	NonLegalNeighborhoodDefinition = 2  // 'Fixture market areas'
	NorthNeighborhood              = 1  // used by definitions 1 and 2
	SouthNeighborhood              = 2  // used by definition 1 only
	MarketAreaNeighborhood         = 3  // used by definition 2 only
	UnusedNeighborhood             = 4  // no assignments
	NeighborhoodHistoryParcel      = 10 // definition 1: South until HistoryChange, North from then
	DatedNonLegalParcel            = 6  // definition 2: neighborhood 3 until HistoryChange, none from then
	SharedNeighborhoodParcel       = 7  // definition 2: North, null range
)

// FixtureID returns the public ID of row n with the given prefix.
func FixtureID(prefix string, n int) string {
	return fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, n)
}

// TransferID returns the public ID of fixture real property transfer n.
func TransferID(n int) string { return FixtureID(TransferPrefix, n) }

// PartyID returns the public ID of fixture party n.
func PartyID(n int) string { return FixtureID(PartyPrefix, n) }

// AddressID returns the public ID of fixture address n.
func AddressID(n int) string { return FixtureID(AddressPrefix, n) }

// NeighborhoodDefinitionID returns the public ID of fixture neighborhood definition n.
func NeighborhoodDefinitionID(n int) string { return FixtureID(NeighborhoodDefPrefix, n) }

// NeighborhoodID returns the public ID of fixture neighborhood n.
func NeighborhoodID(n int) string { return FixtureID(NeighborhoodPrefix, n) }

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
