package main

import (
	"context"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/dexidp/dex/api/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/civil-labs/civil-os/api/gen/go/civil/public/dex/v1/dexv1connect"
	"github.com/civil-labs/civil-os/internal/testutil"
)

// fakeDex is an in-process Dex gRPC API. It records each request and
// answers with a fully populated response, or with errCode.
type fakeDex struct {
	api.UnimplementedDexServer

	mu      sync.Mutex
	reqs    map[string]proto.Message
	resps   map[string]proto.Message
	errCode codes.Code
}

func handle[Req, Res proto.Message](f *fakeDex, method string, req Req) (Res, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs[method] = req
	var zero Res
	if f.errCode != codes.OK {
		return zero, status.Error(f.errCode, "fake dex error")
	}
	return f.resps[method].(Res), nil
}

func (f *fakeDex) GetClient(_ context.Context, r *api.GetClientReq) (*api.GetClientResp, error) {
	return handle[*api.GetClientReq, *api.GetClientResp](f, "GetClient", r)
}
func (f *fakeDex) CreateClient(_ context.Context, r *api.CreateClientReq) (*api.CreateClientResp, error) {
	return handle[*api.CreateClientReq, *api.CreateClientResp](f, "CreateClient", r)
}
func (f *fakeDex) UpdateClient(_ context.Context, r *api.UpdateClientReq) (*api.UpdateClientResp, error) {
	return handle[*api.UpdateClientReq, *api.UpdateClientResp](f, "UpdateClient", r)
}
func (f *fakeDex) DeleteClient(_ context.Context, r *api.DeleteClientReq) (*api.DeleteClientResp, error) {
	return handle[*api.DeleteClientReq, *api.DeleteClientResp](f, "DeleteClient", r)
}
func (f *fakeDex) ListClients(_ context.Context, r *api.ListClientReq) (*api.ListClientResp, error) {
	return handle[*api.ListClientReq, *api.ListClientResp](f, "ListClients", r)
}
func (f *fakeDex) ListRefresh(_ context.Context, r *api.ListRefreshReq) (*api.ListRefreshResp, error) {
	return handle[*api.ListRefreshReq, *api.ListRefreshResp](f, "ListRefresh", r)
}
func (f *fakeDex) RevokeRefresh(_ context.Context, r *api.RevokeRefreshReq) (*api.RevokeRefreshResp, error) {
	return handle[*api.RevokeRefreshReq, *api.RevokeRefreshResp](f, "RevokeRefresh", r)
}

func startFakeDex(t *testing.T) (*fakeDex, string) {
	t.Helper()
	f := &fakeDex{
		reqs: map[string]proto.Message{},
		resps: map[string]proto.Message{
			"GetClient":     full(&api.GetClientResp{}),
			"CreateClient":  full(&api.CreateClientResp{}),
			"UpdateClient":  full(&api.UpdateClientResp{}),
			"DeleteClient":  full(&api.DeleteClientResp{}),
			"ListClients":   full(&api.ListClientResp{}),
			"ListRefresh":   full(&api.ListRefreshResp{}),
			"RevokeRefresh": full(&api.RevokeRefreshResp{}),
		},
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	api.RegisterDexServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return f, lis.Addr().String()
}

// without deletes the named fields from j: "field", "obj.field" or
// "list[].field" (the field in every list item).
func without(j map[string]any, paths ...string) map[string]any {
	for _, p := range paths {
		if list, field, ok := strings.Cut(p, "[]."); ok {
			items, _ := j[list].([]any)
			for _, it := range items {
				delete(it.(map[string]any), field)
			}
		} else if obj, field, ok := strings.Cut(p, "."); ok {
			if m, ok := j[obj].(map[string]any); ok {
				delete(m, field)
			}
		} else {
			delete(j, p)
		}
	}
	return j
}

// TestDexRPCs runs every DexService RPC through the gateway against the fake
// Dex: every request field must reach Dex and every response field must come
// back, except the documented omissions.
func TestDexRPCs(t *testing.T) {
	dex, addr := startFakeDex(t)
	g := startGateway(t, func(c *Config) { c.DexGrpcAddress = addr })

	// Fields the gateway intentionally does not copy.
	omitted := map[string][]string{
		// ListClients returns ClientInfo, which has no secret.
		"ListClients": {"clients[].secret"},
	}

	for _, md := range methods(t, dexv1connect.DexServiceName) {
		name := string(md.Name())
		t.Run(name, func(t *testing.T) {
			req := dynamicpb.NewMessage(md.Input())
			fill(req, 3)

			dex.mu.Lock()
			dex.errCode = codes.OK
			dex.mu.Unlock()
			res, err := g.call(t, md, req, nil)
			if err != nil {
				t.Fatal(err)
			}

			dex.mu.Lock()
			upstreamReq, upstreamRes := dex.reqs[name], dex.resps[name]
			dex.mu.Unlock()
			if upstreamReq == nil {
				t.Fatal("request did not reach Dex")
			}
			if got, want := asJSON(t, upstreamReq), asJSON(t, req); !reflect.DeepEqual(got, want) {
				t.Errorf("Dex request differs from public request:\n dex    %v\n public %v", got, want)
			}
			want := without(asJSON(t, upstreamRes), omitted[name]...)
			if got := asJSON(t, res); !reflect.DeepEqual(got, want) {
				t.Errorf("public response differs from Dex response:\n public %v\n dex    %v", got, want)
			}

			for _, tc := range []struct {
				upstream codes.Code
				want     connect.Code
			}{
				{codes.Unavailable, connect.CodeUnavailable},
				{codes.NotFound, connect.CodeInternal},
			} {
				dex.mu.Lock()
				dex.errCode = tc.upstream
				dex.mu.Unlock()
				_, err := g.call(t, md, req, nil)
				if code := connect.CodeOf(err); code != tc.want {
					t.Errorf("Dex %v reached the caller as %v, want %v", tc.upstream, code, tc.want)
				}
			}
		})
	}
}

func TestDexDisabled(t *testing.T) {
	g := startGateway(t, nil) // no DexGrpcAddress
	md := methods(t, dexv1connect.DexServiceName)[0]
	_, err := g.call(t, md, dynamicpb.NewMessage(md.Input()), nil)
	if code := connect.CodeOf(err); code != connect.CodeUnimplemented {
		t.Errorf("got %v (%v), want Unimplemented", code, err)
	}
}

func TestDexUnreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close() // nothing listens here now

	g := startGateway(t, func(c *Config) { c.DexGrpcAddress = addr })
	for _, md := range methods(t, dexv1connect.DexServiceName) {
		req := dynamicpb.NewMessage(md.Input())
		fill(req, 3)
		_, err := g.call(t, md, req, nil)
		if code := connect.CodeOf(err); code != connect.CodeUnavailable {
			t.Errorf("%s: got %v (%v), want Unavailable", md.Name(), code, err)
		}
	}
}

func TestDexInvalidAddress(t *testing.T) {
	iss := testutil.NewIssuer(t)
	cfg := &Config{
		AuthServer: iss.URL, IDPHost: iss.Host, AllowedClientsIds: []string{testClientID},
		DexGrpcAddress: "%zz", // grpc.NewClient rejects little beyond malformed URLs
	}
	if _, err := newHandler(cfg, discardLogger()); err == nil {
		t.Error("expected newHandler to reject an invalid Dex address")
	}
}
