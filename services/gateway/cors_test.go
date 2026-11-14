package main

import (
	"net/http"
	"strings"
	"testing"
)

func checkCORSHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if got := h.Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	for _, want := range []string{"Authorization", "Content-Type", "Connect-Protocol-Version"} {
		if !strings.Contains(h.Get("Access-Control-Allow-Headers"), want) {
			t.Errorf("Access-Control-Allow-Headers missing %s", want)
		}
	}
	if !strings.Contains(h.Get("Access-Control-Expose-Headers"), "Grpc-Status") {
		t.Error("Access-Control-Expose-Headers missing Grpc-Status")
	}
	if !strings.Contains(h.Get("Access-Control-Allow-Methods"), "POST") {
		t.Error("Access-Control-Allow-Methods missing POST")
	}
}

// TestCORS goes through the real gateway: a preflight needs no token, and
// CORS headers are present on success and on an auth failure, so browsers
// can read the error.
func TestCORS(t *testing.T) {
	g := startGateway(t, nil)
	path := g.url + "/civil.public.zoning.v1.ZoningService/GetZoning"

	pre, _ := http.NewRequest(http.MethodOptions, path, nil)
	pre.Header.Set("Origin", "https://app.example.com")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	res, err := http.DefaultClient.Do(pre)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("preflight status %d, want 200", res.StatusCode)
	}
	checkCORSHeaders(t, res.Header)
	if calls := g.mesh.reset(0); len(calls) != 0 {
		t.Error("preflight reached the mesh")
	}

	for name, token := range map[string]string{"unauthenticated": "", "authenticated": g.token} {
		req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := map[string]int{"unauthenticated": 401, "authenticated": 200}[name]
		if res.StatusCode != want {
			t.Errorf("%s: status %d, want %d", name, res.StatusCode, want)
		}
		checkCORSHeaders(t, res.Header)
	}
}
