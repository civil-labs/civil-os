package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const issuerKeyID = "civil-os-test"

// Issuer is a fake OIDC provider on a local httptest server. It serves
// discovery, a JWKS at /keys (where the gateway looks for Dex's keys) and
// mints RS256 tokens.
type Issuer struct {
	// URL is the issuer, e.g. http://127.0.0.1:41234; use it as
	// CIVIL_OS_GATEWAY_AUTH_SERVER.
	URL string
	// Host is host:port; use it as CIVIL_OS_GATEWAY_IDP_HOST.
	Host string

	key *rsa.PrivateKey
}

// NewIssuer starts a fake issuer that stops when the test ends.
func NewIssuer(t testing.TB) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	iss := &Issuer{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                iss.URL,
			"authorization_endpoint":                iss.URL + "/auth",
			"token_endpoint":                        iss.URL + "/token",
			"jwks_uri":                              iss.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: &key.PublicKey, KeyID: issuerKeyID, Algorithm: string(jose.RS256), Use: "sig",
		}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	iss.URL = srv.URL
	iss.Host = strings.TrimPrefix(srv.URL, "http://")
	return iss
}

// TokenOptions shapes a minted token. Zero values give a valid token for
// subject "test-user" that expires in an hour.
type TokenOptions struct {
	Audience     []string
	Subject      string
	Email        string
	Groups       []string
	Expiry       time.Time // zero: one hour from now
	Issuer       string    // empty: this issuer
	BadSignature bool      // sign with an unrelated key that reuses this key ID
}

// Token mints a signed ID token for the given audience.
func (i *Issuer) Token(t testing.TB, opts TokenOptions) string {
	t.Helper()
	key := i.key
	if opts.BadSignature {
		var err error
		if key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			t.Fatal(err)
		}
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: issuerKeyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if opts.Expiry.IsZero() {
		opts.Expiry = now.Add(time.Hour)
	}
	if opts.Issuer == "" {
		opts.Issuer = i.URL
	}
	if opts.Subject == "" {
		opts.Subject = "test-user"
	}
	std := jwt.Claims{
		Issuer:   opts.Issuer,
		Subject:  opts.Subject,
		Audience: jwt.Audience(opts.Audience),
		IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:   jwt.NewNumericDate(opts.Expiry),
	}
	extra := map[string]any{
		"email":          opts.Email,
		"email_verified": opts.Email != "",
		"groups":         opts.Groups,
	}
	raw, err := jwt.Signed(signer).Claims(std).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
