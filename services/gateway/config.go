package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all the runtime configuration
type Config struct {
	Verbose     string
	IngressPort string
	EgressPort  string
	AuthServer  string
	Namespace   string
	// TileServerLocalHostName string
	// IDPLocalPort            string
	// IDPLocalHostName        string
}

func LoadConfig() (*Config, error) {
	// Define the list of required environment variables
	required := []string{
		"CIVIL_AUTH_SERVER",
		"CIVIL_NAMESPACE",
	}

	// Loop through and check for missing ones
	var missing []string
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	// If any are missing, return a detailed error
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// Return the populated config struct
	// You can also set defaults here for optional vars (like Port)
	return &Config{
		Verbose:     getEnv("CIVIL_VERBOSE", "false"),
		IngressPort: getEnv("CIVIL_INGRESS_PORT", "8080"),
		EgressPort:  getEnv("CIVIL_EGRESS_PORT", "8082"),
		AuthServer:  os.Getenv("CIVIL_AUTH_SERVER"),
		Namespace:   os.Getenv("CIVIL_NAMESPACE"),
	}, nil
}

// Helper for optional variables
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
