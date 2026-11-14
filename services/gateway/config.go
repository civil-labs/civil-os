package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/civil-labs/civil-os/internal/envconfig"
)

const service = envconfig.Gateway

// Config holds all the runtime configuration. See spec §5.5 / docs/architecture.md
// for the CIVIL_OS_GATEWAY_* variables.
type Config struct {
	LogLevel            slog.Level
	Port                uint16
	AuthServer          string
	IDPHost             string // Use local address here. Its where the gateway will make requests for JWKS
	DBReaderHost        string
	DBWriterHost        string // Optional. Empty disables parcel writes (UpdateParcel returns Unavailable).
	TileServerHost      string
	DexGrpcAddress      string // Optional. Empty disables the Dex admin API.
	AllowedClientsIds   []string
	InstanceMetadataUrl string
}

// LoadConfig reads the gateway's configuration from env. It fails on any
// legacy CIVIL_* variable, a missing required variable, or an invalid value.
func LoadConfig(env envconfig.Env) (*Config, error) {
	if err := env.RejectLegacy(); err != nil {
		return nil, err
	}

	key := func(setting string) string { return envconfig.Key(service, setting) }

	if err := env.Require(
		key("AUTH_SERVER"),
		key("IDP_HOST"),
		key("TILE_SERVER_HOST"),
		key("ALLOWED_CLIENT_IDS"),
		key("READER_HOST"),
		key("INSTANCE_METADATA_URL"),
	); err != nil {
		return nil, err
	}

	level, levelErr := env.LogLevel(service)
	port, portErr := env.Port(key("PORT"), 8080)
	clientIDs, clientIDsErr := parseAllowedClientIDs(key("ALLOWED_CLIENT_IDS"), env.Get(key("ALLOWED_CLIENT_IDS")))
	if err := errors.Join(levelErr, portErr, clientIDsErr); err != nil {
		return nil, err
	}

	return &Config{
		LogLevel:            level,
		Port:                port,
		AuthServer:          env.Get(key("AUTH_SERVER")),
		IDPHost:             env.Get(key("IDP_HOST")),
		TileServerHost:      env.Get(key("TILE_SERVER_HOST")),
		DBReaderHost:        env.Get(key("READER_HOST")),
		DBWriterHost:        env.Get(key("WRITER_HOST")),
		DexGrpcAddress:      env.Get(key("DEX_GRPC_ADDRESS")),
		AllowedClientsIds:   clientIDs,
		InstanceMetadataUrl: env.Get(key("INSTANCE_METADATA_URL")),
	}, nil
}

// parseAllowedClientIDs parses a JSON array of OIDC client IDs. An invalid or
// empty list is an error: previously it silently became an empty list, which
// rejects every token.
func parseAllowedClientIDs(name, value string) ([]string, error) {
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("invalid %s: want a JSON array of client IDs, e.g. [\"web\",\"cli\"]: %w", name, err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("invalid %s: the list of allowed client IDs is empty", name)
	}
	return ids, nil
}
