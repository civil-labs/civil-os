package main

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/civil-labs/civil-os/internal/envconfig"
	"github.com/civil-labs/civil-os/internal/testutil"
)

func TestLoadConfigSharedEnv(t *testing.T) {
	cfg, err := LoadConfig(envconfig.FromMap(testutil.SharedEnv()))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		LogLevel:         slog.LevelWarn,
		Port:             50051, // default
		DatabaseHost:     "reader-db",
		DatabasePort:     5433,
		DatabaseUsername: "reader",
		DatabasePassword: "reader-pass",
		DatabaseName:     "civil",
	}
	if *cfg != want {
		t.Errorf("LoadConfig = %+v, want %+v", *cfg, want)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{"legacy variable", map[string]string{"CIVIL_DB_HOST": "db"}, "CIVIL_DB_HOST"},
		{"missing host", map[string]string{"CIVIL_OS_READER_DB_HOST": "-"}, "CIVIL_OS_READER_DB_HOST"},
		{"empty password", map[string]string{"CIVIL_OS_READER_DB_PASS": ""}, "CIVIL_OS_READER_DB_PASS"},
		{"bad port", map[string]string{"CIVIL_OS_READER_PORT": "http"}, "CIVIL_OS_READER_PORT"},
		{"bad db port", map[string]string{"CIVIL_OS_READER_DB_PORT": "99999"}, "CIVIL_OS_READER_DB_PORT"},
		{"bad log level", map[string]string{"CIVIL_OS_READER_LOG_LEVEL": "loud"}, "CIVIL_OS_READER_LOG_LEVEL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(envconfig.FromMap(testutil.WithEnv(testutil.SharedEnv(), tt.overrides)))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want one naming %s", err, tt.wantErr)
			}
		})
	}
}
