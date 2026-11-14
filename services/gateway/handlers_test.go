package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/civil-labs/civil-os/api/gen/go/civil/public/improvements/v1/improvementsv1connect"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/instance/v1/instancev1connect"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/landuses/v1/landusesv1connect"
	publicneighborhoodsv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/neighborhoods/v1/neighborhoodsv1connect"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/parcels/v1/parcelsv1connect"
	publicvaluationsv1connect "github.com/civil-labs/civil-os/api/gen/go/civil/public/valuations/v1/valuationsv1connect"
	"github.com/civil-labs/civil-os/api/gen/go/civil/public/zoning/v1/zoningv1connect"
	"github.com/civil-labs/civil-os/internal/testutil"
)

const testClientID = "test-client"

// meshCall is one request the fake mesh received.
type meshCall struct {
	procedure string
	req       *dynamicpb.Message
	header    http.Header
}

// fakeMesh serves every mesh procedure (reader and writer). It records each
// request and answers with a fully populated response, or with errCode.
type fakeMesh struct {
	mu      sync.Mutex
	calls   []meshCall
	resps   map[string]*dynamicpb.Message
	errCode connect.Code // 0: succeed
}

func newFakeMesh(t *testing.T) (*fakeMesh, string) {
	f := &fakeMesh{resps: map[string]*dynamicpb.Message{}}
	mux := http.NewServeMux()
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "civil.mesh.") {
			return true
		}
		for i := 0; i < fd.Services().Len(); i++ {
			sd := fd.Services().Get(i)
			for j := 0; j < sd.Methods().Len(); j++ {
				f.register(mux, sd.Methods().Get(j))
			}
		}
		return true
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, strings.TrimPrefix(srv.URL, "http://")
}

func (f *fakeMesh) register(mux *http.ServeMux, md protoreflect.MethodDescriptor) {
	procedure := fmt.Sprintf("/%s/%s", md.Parent().FullName(), md.Name())
	resp := dynamicpb.NewMessage(md.Output())
	fillValid(resp, 3)
	f.resps[procedure] = resp

	mux.Handle(procedure, connect.NewUnaryHandler(procedure,
		func(_ context.Context, req *connect.Request[dynamicpb.Message]) (*connect.Response[dynamicpb.Message], error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.calls = append(f.calls, meshCall{procedure, req.Msg, req.Header().Clone()})
			if f.errCode != 0 {
				return nil, connect.NewError(f.errCode, errors.New("fake mesh error"))
			}
			return connect.NewResponse(resp), nil
		},
		connect.WithRequestInitializer(func(_ connect.Spec, msg any) error {
			*msg.(*dynamicpb.Message) = *dynamicpb.NewMessage(md.Input())
			return nil
		}),
	))
}

func (f *fakeMesh) reset(code connect.Code) []meshCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := f.calls
	f.calls, f.errCode = nil, code
	return calls
}

// fillValid fills m like fill, then rewrites string fields so they satisfy
// the public protovalidate rules: IDs become UUIDs and money becomes decimals.
func fillValid(m protoreflect.Message, depth int) {
	fill(m, depth)
	fixStrings(m)
}

func fixStrings(m protoreflect.Message) {
	if m.Descriptor().FullName() == "google.type.Interval" {
		// Validation requires start_time < end_time.
		end := m.Descriptor().Fields().ByName("end_time")
		ts := m.Mutable(end).Message()
		ts.Set(ts.Descriptor().Fields().ByName("seconds"), protoreflect.ValueOfInt64(1_000_000))
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool { fixStrings(mv.Message()); return true })
			}
		case fd.IsList():
			for i := 0; i < v.List().Len(); i++ {
				if fd.Message() != nil {
					fixStrings(v.List().Get(i).Message())
				} else if fd.Kind() == protoreflect.StringKind {
					v.List().Set(i, protoreflect.ValueOfString(validString(fd)))
				}
			}
		case fd.Message() != nil:
			fixStrings(v.Message())
		case fd.Kind() == protoreflect.StringKind:
			m.Set(fd, protoreflect.ValueOfString(validString(fd)))
		}
		return true
	})
}

func validString(fd protoreflect.FieldDescriptor) string {
	name := string(fd.Name())
	switch {
	case name == "id" || strings.HasSuffix(name, "_id") || strings.HasSuffix(name, "_ids"):
		return testutil.ParcelID(int(fd.Number()))
	case strings.Contains(name, "value") || strings.Contains(name, "price"):
		return "1234.5"
	case strings.Contains(name, "wkt"):
		return "POLYGON((0 0, 1 0, 1 1, 0 1, 0 0))"
	case name == "properties":
		return `{"k":"v"}`
	}
	return name + "-value"
}

type gateway struct {
	url    string
	mesh   *fakeMesh
	issuer *testutil.Issuer
	token  string
}

// startGateway runs the real gateway handler against a fake mesh, a fake
// tile server and a fake OIDC issuer. mutate may adjust the config.
func startGateway(t *testing.T, mutate func(*Config)) *gateway {
	t.Helper()
	mesh, meshHost := newFakeMesh(t)
	issuer := testutil.NewIssuer(t)

	tiles := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://tile-server.invalid")
		w.Header().Set("X-Tile-Path", r.URL.Path)
		w.Header().Set("X-Seen-Forwarded-Host", r.Header.Get("X-Forwarded-Host"))
		_, _ = w.Write([]byte("tile"))
	}))
	t.Cleanup(tiles.Close)

	metadata := filepath.Join(t.TempDir(), "instance-metadata.json")
	if err := os.WriteFile(metadata, []byte(`{"name":"Fixture County","center":[-140,-40]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Port:                8080,
		AuthServer:          issuer.URL,
		IDPHost:             issuer.Host,
		DBReaderHost:        meshHost,
		DBWriterHost:        meshHost,
		TileServerHost:      strings.TrimPrefix(tiles.URL, "http://"),
		AllowedClientsIds:   []string{testClientID},
		InstanceMetadataUrl: metadata,
	}
	if mutate != nil {
		mutate(cfg)
	}
	h, err := newHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &gateway{
		url:    srv.URL,
		mesh:   mesh,
		issuer: issuer,
		token:  issuer.Token(t, testutil.TokenOptions{Audience: []string{testClientID}}),
	}
}

// call invokes a public procedure with a dynamic request, authenticated.
func (g *gateway) call(t *testing.T, md protoreflect.MethodDescriptor, req *dynamicpb.Message, header http.Header) (*dynamicpb.Message, error) {
	t.Helper()
	procedure := fmt.Sprintf("/%s/%s", md.Parent().FullName(), md.Name())
	client := connect.NewClient[dynamicpb.Message, dynamicpb.Message](http.DefaultClient, g.url+procedure,
		connect.WithResponseInitializer(func(_ connect.Spec, msg any) error {
			*msg.(*dynamicpb.Message) = *dynamicpb.NewMessage(md.Output())
			return nil
		}))
	r := connect.NewRequest(req)
	r.Header().Set("Authorization", "Bearer "+g.token)
	for k, v := range header {
		r.Header()[k] = v
	}
	res, err := client.CallUnary(context.Background(), r)
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// publicServices are the services the gateway serves from the mesh.
var publicServices = []string{
	parcelsv1connect.ParcelsServiceName,
	improvementsv1connect.ImprovementsServiceName,
	landusesv1connect.LandUsesServiceName,
	zoningv1connect.ZoningServiceName,
	publicvaluationsv1connect.ValuationServiceName,
	publicneighborhoodsv1connect.NeighborhoodServiceName,
}

// localMethods answer without calling the mesh.
var localMethods = map[string]bool{
	"GetNumericalParcelAttributeStatsById":   true, // placeholder data
	"GetCategoricalParcelAttributeStatsById": true, // placeholder data
}

func methods(t *testing.T, service string) []protoreflect.MethodDescriptor {
	t.Helper()
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		t.Fatal(err)
	}
	sd := d.(protoreflect.ServiceDescriptor)
	var out []protoreflect.MethodDescriptor
	for i := 0; i < sd.Methods().Len(); i++ {
		out = append(out, sd.Methods().Get(i))
	}
	return out
}

// TestPublicRPCs checks every public RPC end to end through the gateway:
// the request reaches the mesh method of the same name with every field
// copied, the response comes back with every field copied, and a mesh error
// code reaches the caller unchanged.
func TestPublicRPCs(t *testing.T) {
	g := startGateway(t, nil)
	for _, service := range publicServices {
		for _, md := range methods(t, service) {
			t.Run(string(md.Name()), func(t *testing.T) {
				req := dynamicpb.NewMessage(md.Input())
				fillValid(req, 3)

				g.mesh.reset(0)
				res, err := g.call(t, md, req, nil)
				if err != nil {
					t.Fatalf("call: %v", err)
				}
				calls := g.mesh.reset(connect.CodeNotFound)

				if localMethods[string(md.Name())] {
					if len(calls) != 0 {
						t.Errorf("expected no mesh call, got %d", len(calls))
					}
					return
				}
				if len(calls) != 1 {
					t.Fatalf("expected one mesh call, got %d", len(calls))
				}
				if !strings.HasSuffix(calls[0].procedure, "/"+string(md.Name())) {
					t.Errorf("routed to %s", calls[0].procedure)
				}
				if got, want := asJSON(t, calls[0].req), asJSON(t, req); !reflect.DeepEqual(got, want) {
					t.Errorf("mesh request differs from public request:\n mesh   %v\n public %v", got, want)
				}
				if got, want := asJSON(t, res), asJSON(t, g.mesh.resps[calls[0].procedure]); !reflect.DeepEqual(got, want) {
					t.Errorf("public response differs from mesh response:\n public %v\n mesh   %v", got, want)
				}

				_, err = g.call(t, md, req, nil)
				if code := connect.CodeOf(err); code != connect.CodeNotFound {
					t.Errorf("mesh NotFound reached the caller as %v (%v)", code, err)
				}
				g.mesh.reset(0)
			})
		}
	}
}

// TestPublicRPCValidation sends each RPC that takes IDs a malformed ID and
// expects InvalidArgument from protovalidate before the mesh is called.
func TestPublicRPCValidation(t *testing.T) {
	g := startGateway(t, nil)
	tested := 0
	for _, service := range publicServices {
		for _, md := range methods(t, service) {
			req := dynamicpb.NewMessage(md.Input())
			fillValid(req, 3)
			if !breakIDs(req) {
				continue
			}
			tested++
			t.Run(string(md.Name()), func(t *testing.T) {
				g.mesh.reset(0)
				_, err := g.call(t, md, req, nil)
				if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
					t.Errorf("got %v (%v), want InvalidArgument", code, err)
				}
				if calls := g.mesh.reset(0); len(calls) != 0 {
					t.Errorf("invalid request reached the mesh")
				}
			})
		}
	}
	if tested == 0 {
		t.Fatal("no RPC with ID fields found")
	}
}

// hasUUIDRule reports whether fd (or each item of repeated fd) carries a
// protovalidate string.uuid rule.
func hasUUIDRule(fd protoreflect.FieldDescriptor) bool {
	rules, ok := proto.GetExtension(fd.Options(), validate.E_Field).(*validate.FieldRules)
	if !ok || rules == nil {
		return false
	}
	return rules.GetString().GetUuid() || rules.GetRepeated().GetItems().GetString().GetUuid()
}

// breakIDs replaces each top-level UUID-validated string in req with a
// non-UUID. It reports whether the request has such a field.
func breakIDs(req *dynamicpb.Message) bool {
	broke := false
	req.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsMap() || !hasUUIDRule(fd) {
			return true
		}
		if fd.IsList() {
			v.List().Set(0, protoreflect.ValueOfString("not-a-uuid"))
		} else {
			req.Set(fd, protoreflect.ValueOfString("not-a-uuid"))
		}
		broke = true
		return true
	})
	return broke
}

func updateParcelMethod(t *testing.T) protoreflect.MethodDescriptor {
	for _, md := range methods(t, parcelsv1connect.ParcelsServiceName) {
		if md.Name() == "UpdateParcel" {
			return md
		}
	}
	t.Fatal("UpdateParcel not found")
	return nil
}

func TestUpdateParcelForwardsTraceparent(t *testing.T) {
	g := startGateway(t, nil)
	md := updateParcelMethod(t)
	req := dynamicpb.NewMessage(md.Input())
	fillValid(req, 3)

	const tp = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if _, err := g.call(t, md, req, http.Header{"Traceparent": {tp}}); err != nil {
		t.Fatal(err)
	}
	calls := g.mesh.reset(0)
	if len(calls) != 1 || !strings.Contains(calls[0].procedure, "ParcelWriteService") {
		t.Fatalf("expected one ParcelWriteService call, got %+v", calls)
	}
	if got := calls[0].header.Get("Traceparent"); got != tp {
		t.Errorf("traceparent = %q, want %q", got, tp)
	}
}

func TestUpdateParcelWithoutWriter(t *testing.T) {
	g := startGateway(t, func(c *Config) { c.DBWriterHost = "" })
	md := updateParcelMethod(t)
	req := dynamicpb.NewMessage(md.Input())
	fillValid(req, 3)

	_, err := g.call(t, md, req, nil)
	if code := connect.CodeOf(err); code != connect.CodeUnavailable {
		t.Errorf("got %v (%v), want Unavailable", code, err)
	}
	if calls := g.mesh.reset(0); len(calls) != 0 {
		t.Errorf("mesh was called %d times", len(calls))
	}
}

func TestInstanceMetadata(t *testing.T) {
	g := startGateway(t, nil)
	md := methods(t, instancev1connect.InstanceServiceName)[0]

	// No Authorization header: the instance service is public so clients can
	// discover the issuer before signing in.
	g.token = ""
	res, err := g.call(t, md, dynamicpb.NewMessage(md.Input()), nil)
	if err != nil {
		t.Fatal(err)
	}
	j := asJSON(t, res)
	if j["auth_issuer_url"] != g.issuer.URL {
		t.Errorf("auth_issuer_url = %v, want %s", j["auth_issuer_url"], g.issuer.URL)
	}
	if meta, _ := j["metadata"].(map[string]any); meta["name"] != "Fixture County" {
		t.Errorf("metadata = %v", j["metadata"])
	}
}

func TestInstanceMetadataMissingFile(t *testing.T) {
	g := startGateway(t, func(c *Config) { c.InstanceMetadataUrl = filepath.Join(t.TempDir(), "missing.json") })
	md := methods(t, instancev1connect.InstanceServiceName)[0]
	_, err := g.call(t, md, dynamicpb.NewMessage(md.Input()), nil)
	if code := connect.CodeOf(err); code != connect.CodeInternal {
		t.Errorf("got %v, want Internal", code)
	}
}

func TestTileProxy(t *testing.T) {
	g := startGateway(t, nil)

	req, _ := http.NewRequest(http.MethodGet, g.url+"/tiles/parcels/14/1820/10181", nil)
	if res, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	} else if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated tile request: status %d, want 401", res.StatusCode)
	}

	req.Header.Set("Authorization", "Bearer "+g.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "tile" {
		t.Fatalf("status %d body %q", res.StatusCode, body)
	}
	if got := res.Header.Get("X-Tile-Path"); got != "/tiles/parcels/14/1820/10181" {
		t.Errorf("tile server saw path %q", got)
	}
	if got := res.Header.Get("X-Seen-Forwarded-Host"); got == "" {
		t.Error("X-Forwarded-Host not set")
	}
	if got := res.Header.Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the gateway's single *", got)
	}
}

func TestHealth(t *testing.T) {
	g := startGateway(t, nil)
	res, err := http.Get(g.url + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"OK"`) {
		t.Errorf("status %d body %s", res.StatusCode, body)
	}
}
