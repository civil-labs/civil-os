package main

import (
	"errors"
	"log/slog"

	"github.com/civil-labs/civil-os/internal/envconfig"
)

const service = envconfig.Reader

// Config holds all the runtime configuration. See spec §5.5 / docs/architecture.md
// for the CIVIL_OS_READER_* variables.
type Config struct {
	LogLevel         slog.Level
	Port             uint16
	DatabaseHost     string
	DatabasePort     uint16
	DatabaseUsername string
	DatabasePassword string
	DatabaseName     string
}

// LoadConfig reads the reader's configuration from env. It fails on any
// legacy CIVIL_* variable, a missing required variable, or an invalid value.
func LoadConfig(env envconfig.Env) (*Config, error) {
	if err := env.RejectLegacy(); err != nil {
		return nil, err
	}

	key := func(setting string) string { return envconfig.Key(service, setting) }

	if err := env.Require(key("DB_HOST"), key("DB_USER"), key("DB_PASS"), key("DB_NAME")); err != nil {
		return nil, err
	}

	level, levelErr := env.LogLevel(service)
	port, portErr := env.Port(key("PORT"), 50051)
	dbPort, dbPortErr := env.Port(key("DB_PORT"), 5432)
	if err := errors.Join(levelErr, portErr, dbPortErr); err != nil {
		return nil, err
	}

	return &Config{
		LogLevel:         level,
		Port:             port,
		DatabaseHost:     env.Get(key("DB_HOST")),
		DatabasePort:     dbPort,
		DatabaseUsername: env.Get(key("DB_USER")),
		DatabasePassword: env.Get(key("DB_PASS")),
		DatabaseName:     env.Get(key("DB_NAME")),
	}, nil
}
