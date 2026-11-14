package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/civil-labs/civil-os/internal/testutil"
)

// authed wraps a handler that echoes the injected claims' subject.
func authed(t *testing.T, iss *testutil.Issuer, clientIDs ...string) http.Handler {
	t.Helper()
	mw, err := RequireAuth(iss.URL, iss.Host, clientIDs, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	return mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := r.Context().Value(userContextKey).(Claims)
		_, _ = w.Write([]byte(claims.Subject + "|" + claims.Email))
	}))
}

func TestRequireAuth(t *testing.T) {
	iss := testutil.NewIssuer(t)
	other := testutil.NewIssuer(t)
	h := authed(t, iss, "web", "cli")

	tests := []struct {
		name   string
		header string
		status int
		body   string
	}{
		{"valid", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"web"}, Subject: "alice", Email: "a@example.com"}), 200, "alice|a@example.com"},
		{"second allowed client", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"cli"}}), 200, "test-user|"},
		{"one of several audiences allowed", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"other", "cli"}}), 200, "test-user|"},
		{"wrong client", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"mobile"}}), 401, "Unrecognized client"},
		{"expired", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"web"}, Expiry: time.Now().Add(-time.Minute)}), 401, "Invalid or expired"},
		{"bad signature", "Bearer " + iss.Token(t, testutil.TokenOptions{Audience: []string{"web"}, BadSignature: true}), 401, "Invalid or expired"},
		{"other issuer", "Bearer " + other.Token(t, testutil.TokenOptions{Audience: []string{"web"}}), 401, "Invalid or expired"},
		{"garbage token", "Bearer not.a.jwt", 401, "Invalid or expired"},
		{"missing", "", 401, "Missing or invalid"},
		{"not bearer", "Basic dXNlcjpwYXNz", 401, "Missing or invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("got %d %q, want %d containing %q", rec.Code, rec.Body.String(), tt.status, tt.body)
			}
		})
	}
}

func TestRequireAuthConfig(t *testing.T) {
	log := discardLogger()
	for name, call := range map[string]func() error{
		"no auth server": func() error { _, err := RequireAuth("", "idp:5556", []string{"web"}, log); return err },
		"no IdP host":    func() error { _, err := RequireAuth("dex", "", []string{"web"}, log); return err },
		"no client IDs":  func() error { _, err := RequireAuth("dex", "idp:5556", nil, log); return err },
	} {
		if call() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestIssuerURL(t *testing.T) {
	for in, want := range map[string]string{
		"auth.example.com":         "https://auth.example.com",
		"auth.example.com/dex":     "https://auth.example.com/dex",
		"https://auth.example.com": "https://auth.example.com",
		"http://dex:5556":          "http://dex:5556",
		"http://dex:5556/":         "http://dex:5556",
	} {
		if got := issuerURL(in); got != want {
			t.Errorf("issuerURL(%q) = %q, want %q", in, got, want)
		}
	}
}
