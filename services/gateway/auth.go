package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

const userContextKey contextKey = "userClaims"

// Claims defines the exact data you expect Dex/LLDAP to inject into the token
type Claims struct {
	Subject           string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

// RequireAuth is the middleware wrapper
func RequireAuth(verbose bool, authServer string, idpHost string, allowedClientIDs []string, logger *slog.Logger) (func(http.Handler) http.Handler, error) {

	providerConfig := oidc.ProviderConfig{
		IssuerURL:   "https://" + authServer,
		AuthURL:     "https://" + authServer,
		TokenURL:    "https://" + authServer + "/token",
		UserInfoURL: "https://" + authServer + "/userinfo",
		JWKSURL:     "http://" + idpHost + "/keys",
		Algorithms:  []string{"RS256"}, // Dex uses RS256 by default
	}

	if verbose {
		DumpRawJWKS(providerConfig.JWKSURL, logger)
	}

	// Initialize the Provider to securely fetch the JWKS keys from Dex
	provider := providerConfig.NewProvider(context.Background())

	// Configure the verifier to not run the clientID check
	// We'll need to do it manually as we'll have a list of acceptable
	// client IDs
	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	// Return the actual middleware function
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Unauthorized: Missing or invalid Bearer token", http.StatusUnauthorized)

				if verbose {
					logger.Debug("Unauthorized: Missing or invalid Bearer token")
				}

				return
			}
			rawIDToken := strings.TrimPrefix(authHeader, "Bearer ")

			if verbose {
				log.Println("ID Token: " + rawIDToken)
			}

			// Verify the cryptographic signature and expiration
			idToken, err := verifier.Verify(r.Context(), rawIDToken)
			if err != nil {
				http.Error(w, "Unauthorized: Invalid or expired token", http.StatusUnauthorized)

				if verbose {
					logger.Debug("Unauthorized: Invalid or expired token", slog.Any("error", err))
				}

				return
			}

			// Manually check if the audience is one of the allowed clients
			// We have to iterate over aud, as coreos/oidc normalizes it to
			// an array no matter what to handle an edge case in the spec
			isValidAudience := false
			for _, aud := range idToken.Audience {
				for _, allowed := range allowedClientIDs {
					if aud == allowed {
						isValidAudience = true
						break
					}
				}
			}

			if !isValidAudience {
				http.Error(w, "Unauthorized: Unrecognized client application", http.StatusUnauthorized)

				if verbose {
					logger.Debug("Unauthorized: Unrecognized client application")
				}

				return
			}

			// 3. Parse the LLDAP claims
			var claims Claims
			if err := idToken.Claims(&claims); err != nil {
				http.Error(w, "Internal Error: Failed to parse identity claims", http.StatusInternalServerError)

				if verbose {
					logger.Debug("Unauthorized: Failed to parse identity claims", slog.Any("error", err))
				}

				return
			}

			// 4. Inject the claims into the request context
			ctx := context.WithValue(r.Context(), userContextKey, claims)

			if verbose {
				slog.Debug("authentication successful")
			}

			// Pass the request down the chain with the newly populated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// DumpRawJWKS makes a raw HTTP request to the IDP and prints the exact response body.
func DumpRawJWKS(jwksURL string, logger *slog.Logger) {
	logger.Debug("attempting to fetch raw keys from %s", jwksURL)

	resp, err := http.Get(jwksURL)
	if err != nil {
		logger.Debug("network request failed", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("failed to read JWKS response body", slog.Any("error", err))
		return
	}

	logger.Debug("JWKS response", slog.Any("status", resp.StatusCode), slog.Any("payload", string(body)))
}
