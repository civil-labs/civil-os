package main

import (
	"math"
	"regexp"
	"strings"
	"testing"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
)

func ptr[T any](v T) *T { return &v }

func TestNewParcelUpdateConvertsUnits(t *testing.T) {
	upd, err := newParcelUpdate(&parcelsv1.UpdateParcelRequest{
		ParcelId:     "p",
		LandAreaSqFt: ptr(10763.9),
		FrontageFt:   ptr(32.8084),
		DepthFt:      ptr(0.0),
	})
	if err != nil {
		t.Fatal(err)
	}
	near := func(got *float64, want float64) bool { return got != nil && math.Abs(*got-want) < 1e-9 }
	if !near(upd.landAreaSqM, 1000) || !near(upd.frontageM, 10) || !near(upd.depthM, 0) {
		t.Errorf("got area %v, frontage %v, depth %v; want 1000, 10, 0", *upd.landAreaSqM, *upd.frontageM, *upd.depthM)
	}
}

func TestNewParcelUpdateRoundTripsReaderUnits(t *testing.T) {
	// The reader converts back with the same constants, so feet in == feet out.
	for _, ft := range []float64{0.1, 1, 52.5, 123456.789} {
		upd, err := newParcelUpdate(&parcelsv1.UpdateParcelRequest{ParcelId: "p", FrontageFt: ptr(ft)})
		if err != nil {
			t.Fatal(err)
		}
		if back := *upd.frontageM * feetPerMeter; math.Abs(back-ft) > 1e-9 {
			t.Errorf("%v ft -> %v m -> %v ft", ft, *upd.frontageM, back)
		}
	}
}

func TestNewParcelUpdatePartial(t *testing.T) {
	upd, err := newParcelUpdate(&parcelsv1.UpdateParcelRequest{ParcelId: "p", LandUseId: ptr("lu")})
	if err != nil {
		t.Fatal(err)
	}
	if upd.landUsePublicID == nil || *upd.landUsePublicID != "lu" {
		t.Error("land use not carried")
	}
	if upd.addressPublicID != nil || upd.landAreaSqM != nil || upd.frontageM != nil || upd.depthM != nil || upd.properties != nil {
		t.Errorf("unset fields should stay nil: %+v", upd)
	}
}

func TestNewParcelUpdateRejects(t *testing.T) {
	tests := []struct {
		name string
		req  *parcelsv1.UpdateParcelRequest
		want string
	}{
		{"no parcel", &parcelsv1.UpdateParcelRequest{}, "parcel_id"},
		{"negative area", &parcelsv1.UpdateParcelRequest{ParcelId: "p", LandAreaSqFt: ptr(-1.0)}, "land_area_sq_ft"},
		{"NaN frontage", &parcelsv1.UpdateParcelRequest{ParcelId: "p", FrontageFt: ptr(math.NaN())}, "frontage_ft"},
		{"infinite depth", &parcelsv1.UpdateParcelRequest{ParcelId: "p", DepthFt: ptr(math.Inf(1))}, "depth_ft"},
		{"properties array", &parcelsv1.UpdateParcelRequest{ParcelId: "p", Properties: ptr(`[1,2]`)}, "JSON object"},
		{"properties null", &parcelsv1.UpdateParcelRequest{ParcelId: "p", Properties: ptr(`null`)}, "JSON object"},
		{"properties string", &parcelsv1.UpdateParcelRequest{ParcelId: "p", Properties: ptr(`"x"`)}, "JSON object"},
		{"properties invalid", &parcelsv1.UpdateParcelRequest{ParcelId: "p", Properties: ptr(`{`)}, "JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newParcelUpdate(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

func TestNewParcelUpdateAcceptsEmptyObject(t *testing.T) {
	upd, err := newParcelUpdate(&parcelsv1.UpdateParcelRequest{ParcelId: "p", Properties: ptr(`{}`)})
	if err != nil || upd.properties == nil || *upd.properties != "{}" {
		t.Errorf("got %v, %v; want properties {}", upd.properties, err)
	}
}

func TestTraceIDFromTraceparent(t *testing.T) {
	const id = "4bf92f3577b34da6a3ce929d0e0e4736"
	random := regexp.MustCompile(`^[0-9a-f]{32}$`)

	valid := map[string]string{
		"00-" + id + "-00f067aa0ba902b7-01":                  id,
		"  00-" + id + "-00f067aa0ba902b7-00  ":              id,
		"00-" + strings.ToUpper(id) + "-00f067aa0ba902b7-01": id,
		"01-" + id + "-00f067aa0ba902b7-01":                  id, // future version, same layout
	}
	for header, want := range valid {
		if got := traceIDFromTraceparent(header); got != want {
			t.Errorf("traceIDFromTraceparent(%q) = %q, want %q", header, got, want)
		}
	}

	invalid := []string{
		"",
		"garbage",
		"ff-" + id + "-00f067aa0ba902b7-01", // forbidden version
		"00-" + strings.Repeat("0", 32) + "-00f067aa0ba902b7-01", // all-zero trace ID
		"00-" + id[:31] + "-00f067aa0ba902b7-01",                 // short
		"00-" + id[:31] + "g-00f067aa0ba902b7-01",                // not hex
		"00-" + id + "-00f067aa0ba902b7",                         // missing flags
	}
	seen := map[string]bool{}
	for _, header := range invalid {
		got := traceIDFromTraceparent(header)
		if !random.MatchString(got) || got == id {
			t.Errorf("traceIDFromTraceparent(%q) = %q, want a fresh random ID", header, got)
		}
		if seen[got] {
			t.Errorf("random trace ID repeated: %s", got)
		}
		seen[got] = true
	}
}
