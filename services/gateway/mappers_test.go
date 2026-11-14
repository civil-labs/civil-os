package main

import (
	"encoding/json"
	"reflect"
	"testing"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	publicparcelsv1 "github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1"
	meshimprovementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
	meshlandusesv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/landuses/v1"
	meshparcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	meshzoningv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/zoning/v1"
)

// fill sets every field of m (recursively, to a small depth) to a non-zero
// value, so a mapper that forgets a field produces a visible difference.
func fill(m protoreflect.Message, depth int) {
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if oo := fd.ContainingOneof(); oo != nil && !oo.IsSynthetic() && oo.Fields().Get(0) != fd {
			continue // only the first member of a real oneof
		}
		switch {
		case fd.IsMap():
			mp := m.Mutable(fd).Map()
			mp.Set(scalar(fd.MapKey(), 0).MapKey(), elem(m, fd.MapValue(), mp.NewValue, depth))
		case fd.IsList():
			l := m.Mutable(fd).List()
			l.Append(elem(m, fd, l.NewElement, depth))
		case fd.Message() != nil:
			if depth > 0 {
				fill(m.Mutable(fd).Message(), depth-1)
			}
		default:
			m.Set(fd, scalar(fd, i))
		}
	}
}

func elem(_ protoreflect.Message, fd protoreflect.FieldDescriptor, newVal func() protoreflect.Value, depth int) protoreflect.Value {
	if fd.Message() != nil {
		v := newVal()
		if depth > 0 {
			fill(v.Message(), depth-1)
		}
		return v
	}
	return scalar(fd, 1)
}

func scalar(fd protoreflect.FieldDescriptor, i int) protoreflect.Value {
	n := int64(i + 3)
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.EnumKind:
		// A value the field's enum.in rule allows, else the first non-zero
		// value (zero is UNSPECIFIED, which rules usually exclude).
		if rules, ok := proto.GetExtension(fd.Options(), validate.E_Field).(*validate.FieldRules); ok {
			if in := rules.GetEnum().GetIn(); len(in) > 0 {
				return protoreflect.ValueOfEnum(protoreflect.EnumNumber(in[0]))
			}
		}
		vals := fd.Enum().Values()
		return protoreflect.ValueOfEnum(vals.Get(min(1, vals.Len()-1)).Number())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(n))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(n)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(n))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(uint64(n))
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(n) + 0.5)
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(float64(n) + 0.25)
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(string(fd.Name()) + "-value")
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(fd.Name()))
	}
	panic("unhandled kind " + fd.Kind().String())
}

// asJSON renders m with proto field names and enum numbers, so a mesh message
// and its public twin compare equal when every field was copied.
func asJSON(t *testing.T, m proto.Message) map[string]any {
	t.Helper()
	b, err := protojson.MarshalOptions{UseProtoNames: true, UseEnumNumbers: true}.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func full[M proto.Message](m M) M {
	fill(m.ProtoReflect(), 3)
	return m
}

// checkMapMapper runs the common cases for a map[string]*Mesh -> map[string]*Public mapper.
func checkMapMapper[M, P proto.Message](t *testing.T, mapper func(map[string]M) map[string]P, newMesh func() M) {
	t.Helper()
	if got := mapper(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := mapper(map[string]M{}); got == nil || len(got) != 0 {
		t.Errorf("empty input: got %v, want empty non-nil map", got)
	}

	var nilMesh M
	src := newMesh()
	got := mapper(map[string]M{"full": full(src), "empty": newMesh(), "nil": nilMesh})
	if _, ok := got["nil"]; ok {
		t.Error("nil entry should be skipped")
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if j := asJSON(t, got["empty"]); len(j) != 0 {
		t.Errorf("empty entry mapped to %v", j)
	}
	want, gotJSON := asJSON(t, src), asJSON(t, got["full"])
	if !reflect.DeepEqual(gotJSON, want) {
		t.Errorf("full entry not copied field for field:\n got  %v\n want %v", gotJSON, want)
	}
}

func TestMapZoning(t *testing.T) {
	checkMapMapper(t, mapZoning, func() *meshzoningv1.Zoning { return &meshzoningv1.Zoning{} })
}

func TestMapImprovementTypes(t *testing.T) {
	checkMapMapper(t, mapImprovementTypes, func() *meshimprovementsv1.ImprovementType { return &meshimprovementsv1.ImprovementType{} })
}

func TestMapImprovementConditions(t *testing.T) {
	checkMapMapper(t, mapImprovementConditions, func() *meshimprovementsv1.ImprovementCondition { return &meshimprovementsv1.ImprovementCondition{} })
}

func TestMapLandUses(t *testing.T) {
	checkMapMapper(t, mapLandUses, func() *meshlandusesv1.LandUse { return &meshlandusesv1.LandUse{} })
}

func TestMapLandUseTypes(t *testing.T) {
	checkMapMapper(t, mapLandUseTypes, func() *meshlandusesv1.LandUseType { return &meshlandusesv1.LandUseType{} })
}

func TestMapEquityComparableParcels(t *testing.T) {
	checkMapMapper(t, mapEquityComparableParcels, func() *meshparcelsv1.EquityComparableParcel { return &meshparcelsv1.EquityComparableParcel{} })
}

func TestMapSaleComparableParcels(t *testing.T) {
	checkMapMapper(t, mapSaleComparableParcels, func() *meshparcelsv1.SaleComparableParcel { return &meshparcelsv1.SaleComparableParcel{} })
}

func TestMapParcelWithImprovementSummary(t *testing.T) {
	checkMapMapper(t, mapParcelWithImprovementSummary, func() *meshparcelsv1.ParcelWithImprovementSummary {
		return &meshparcelsv1.ParcelWithImprovementSummary{}
	})

	// Partial: details without a summary stays partial.
	partial := map[string]*meshparcelsv1.ParcelWithImprovementSummary{
		"p": {ParcelDetails: full(&meshparcelsv1.ParcelDetails{})},
	}
	got := mapParcelWithImprovementSummary(partial)["p"]
	if got.ParcelDetails == nil || got.ImprovementSummary != nil {
		t.Errorf("partial entry: details=%v summary=%v; want details only", got.ParcelDetails, got.ImprovementSummary)
	}
}

func TestMapCriteria(t *testing.T) {
	if got := mapCriteria(nil); got != nil {
		t.Errorf("nil input: got %v", got)
	}
	src := full(&publicparcelsv1.ComparableCriteria{})
	got := mapCriteria([]*publicparcelsv1.ComparableCriteria{src, nil})
	if len(got) != 2 || got[1] != nil {
		t.Fatalf("got %v; want two entries with a nil second", got)
	}
	if g, w := asJSON(t, got[0]), asJSON(t, src); !reflect.DeepEqual(g, w) {
		t.Errorf("criteria not copied:\n got  %v\n want %v", g, w)
	}
}

func TestMapComparableAttributes(t *testing.T) {
	if got := mapComparableAttributes(nil); got != nil {
		t.Errorf("nil input: got %v", got)
	}
	src := full(&meshparcelsv1.ComparableAttribute{})
	got := mapComparableAttributes([]*meshparcelsv1.ComparableAttribute{src, nil})
	if len(got) != 2 || got[1] != nil {
		t.Fatalf("got %v; want two entries with a nil second", got)
	}
	if g, w := asJSON(t, got[0]), asJSON(t, src); !reflect.DeepEqual(g, w) {
		t.Errorf("attributes not copied:\n got  %v\n want %v", g, w)
	}
}
