package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all the runtime configuration
type Config struct {
	Namespace             string
	TileServerServiceName string
	Port                  string
}

// LoadConfig reads and validates all environment variables
func LoadConfig() (*Config, error) {
	// 1. Define the list of required environment variables
	required := []string{
		"CLOUD_MAP_NAMESPACE",
		"TILE_SERVER_CLOUD_MAP_SERVICE_NAME",
		// Add future variables here, e.g., "AWS_REGION", "API_KEY", etc.
	}

	// 2. Loop through and check for missing ones
	var missing []string
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	// 3. If any are missing, return a detailed error
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// 4. Return the populated config struct
	// You can also set defaults here for optional vars (like Port)
	return &Config{
		Namespace:             os.Getenv("CLOUD_MAP_NAMESPACE"),
		TileServerServiceName: os.Getenv("TILE_SERVER_CLOUD_MAP_SERVICE_NAME"),
		Port:                  getEnv("PORT", "8080"), // Optional with default
	}, nil
}

// Helper for optional variables
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
