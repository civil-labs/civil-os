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
	Verbose             bool
	Port                uint16
	AuthServer          string
	IDPHost             string // Use local address here. Its where the gateway will make requests for JWKS
	DBReaderHost        string
	TileServerHost      string
	DexGrpcAddress      string
	AllowedClientsIds   []string
	InstanceMetadataUri string
}

func LoadConfig(logger *slog.Logger) (*Config, error) {
	// Define the list of required environment variables
	required := []string{
		"CIVIL_AUTH_SERVER",
		"CIVIL_IDP_HOST",
		"CIVIL_TILE_SERVER_HOST",
		"CIVIL_ALLOWED_CLIENT_IDS",
		"CIVIL_DB_READER_HOST",
		"CIVIL_INSTANCE_METADATA_URI",
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
		Verbose:             getVerboseEnv(),
		Port:                getPortEnv("CIVIL_PORT", 8080, logger),
		AuthServer:          os.Getenv("CIVIL_AUTH_SERVER"),
		IDPHost:             os.Getenv("CIVIL_IDP_HOST"),
		TileServerHost:      os.Getenv("CIVIL_TILE_SERVER_HOST"),
		DBReaderHost:        os.Getenv("CIVIL_DB_READER_HOST"),
		DexGrpcAddress:      os.Getenv("CIVIL_DEX_GRPC_ADDRESS"),
		AllowedClientsIds:   getAllowedClientIdsEnv(),
		InstanceMetadataUri: os.Getenv("CIVIL_INSTANCE_METADATA_URI"),
	}, nil
}

// Helper for optional variables
// func getEnv(key, fallback string) string {
// 	if value, exists := os.LookupEnv(key); exists {
// 		return value
// 	}
// 	return fallback
// }

func getVerboseEnv() bool {
	if value, exists := os.LookupEnv("CIVIL_VERBOSE"); exists {
		verbose, err := strconv.ParseBool(value)

		if err != nil {
			slog.Error("Failure in parsing CIVIL_VERBOSE. Defaulting to false", slog.Any("error", err))
			return false
		}

		return verbose
	}

	return false
}

func getPortEnv(key string, fallback uint16, logger *slog.Logger) uint16 {
	if value, exists := os.LookupEnv(key); exists {
		var intValue, err = strconv.ParseUint(value, 10, 16)

		if err != nil {
			logger.Warn("Failure in parsing integer. Falling back to default", slog.Any("error", err), slog.Int("applied_default", int(fallback)))
			return fallback
		}

		return uint16(intValue)
	} else {
		return fallback
	}

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
