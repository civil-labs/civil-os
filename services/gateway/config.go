package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all the runtime configuration
type Config struct {
	Verbose           bool
	HttpPort          string
	AuthServer        string
	IDPAddress        string // Use local address here. Its where the gateway will make requests for JWKS
	TileServerAddress string
	AllowedClientsIds []string
}

func LoadConfig() (*Config, error) {
	// Define the list of required environment variables
	required := []string{
		"CIVIL_AUTH_SERVER",
		"CIVIL_IDP_ADDRESS",
		"CIVIL_TILE_SERVER_ADDRESS",
		"CIVIL_ALLOWED_CLIENT_IDS",
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
		Verbose:           getVerboseEnv(),
		HttpPort:          getEnv("CIVIL_HTTP_PORT", "8080"),
		AuthServer:        os.Getenv("CIVIL_AUTH_SERVER"),
		IDPAddress:        os.Getenv("CIVIL_IDP_ADDRESS"),
		TileServerAddress: os.Getenv("CIVIL_TILE_SERVER_ADDRESS"),
		AllowedClientsIds: getAllowedClientIdsEnv(),
	}, nil
}

// Helper for optional variables
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getVerboseEnv() bool {
	if _, exists := os.LookupEnv("CIVIL_VERBOSE"); exists {
		verbose, err := strconv.ParseBool("CIVIL_VERBOSE")

		if err != nil {
			slog.Error("Failure in parsing CIVIL_VERBOSE. Defaulting to false", slog.Any("error", err))
			return false
		}

		return verbose
	}

	return false
}

func getAllowedClientIdsEnv() []string {
	if value, exists := os.LookupEnv("CIVIL_ALLOWED_CLIENT_IDS"); exists {
		var clientIds []string

		if value != "" {
			// Unmarshal the JSON string directly into the slice
			err := json.Unmarshal([]byte(value), &clientIds)
			if err != nil {
				slog.Error("Failed to parse CIVIL_ALLOWED_CLIENT_IDS. Defaulting to empty slice", slog.Any("error", err))
				return []string{}
			}
		}

		return clientIds
	}

	slog.Error("Can't find required environment variable CIVIL_ALLOWED_CLIENT_IDS. Defaulting to empty slice")
	return []string{}
}
