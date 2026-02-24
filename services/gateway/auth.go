package main

import (
	"context"
	"log"
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
func RequireAuth(localHostName string, localPort string, namespace string, allowedClientIDs []string) (func(http.Handler) http.Handler, error) {

	providerConfig := oidc.ProviderConfig{
		IssuerURL:   "https://auth.civillabs.app",
		AuthURL:     "https://auth.civillabs.app",
		TokenURL:    "https://auth.civillabs.app/token",
		UserInfoURL: "https://auth.civillabs.app/userinfo",
		JWKSURL:     "http://" + localHostName + "." + namespace + ":" + localPort + "/keys",
		Algorithms:  []string{"RS256"}, // Dex uses RS256 by default
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
				log.Println("Unauthorized: Missing or invalid Bearer token")
				return
			}
			rawIDToken := strings.TrimPrefix(authHeader, "Bearer ")

			log.Println("rawIDToken: " + rawIDToken)

			keySet := oidc.NewRemoteKeySet(r.Context(), "http://"+localHostName+"."+namespace+":"+localPort+"/keys")
			log.Printf("🚨 GATEWAY LOADED KEYS. Found key set: %v", keySet) // This will fail but print debug info
			keys, _ := keySet.VerifySignature(r.Context(), rawIDToken)
			log.Printf("🚨 GATEWAY LOADED KEYS. Token requires kid: %s", keys) // This will fail but print debug info

			// Verify the cryptographic signature and expiration
			idToken, err := verifier.Verify(r.Context(), rawIDToken)
			if err != nil {
				log.Printf("Unauthorized: Invalid or expired token: %v", err)

				http.Error(w, "Unauthorized: Invalid or expired token", http.StatusUnauthorized)
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
				log.Println("Unauthorized: Unrecognized client application")
				return
			}

			// 3. Parse the LLDAP claims
			var claims Claims
			if err := idToken.Claims(&claims); err != nil {
				http.Error(w, "Internal Error: Failed to parse identity claims", http.StatusInternalServerError)
				log.Println("Unauthorized: Failed to parse identity claims")
				return
			}

			// 4. Inject the claims into the request context
			ctx := context.WithValue(r.Context(), userContextKey, claims)

			// Pass the request down the chain with the newly populated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}
