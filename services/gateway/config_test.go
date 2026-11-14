package main

import (
	"log/slog"
	"reflect"
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
	want := &Config{
		LogLevel:            slog.LevelDebug,
		Port:                8081,
		AuthServer:          "http://dex:5556",
		IDPHost:             "dex:5556",
		DBReaderHost:        "http://database-reader:50051",
		DBWriterHost:        "http://database-writer:50052",
		TileServerHost:      "http://tile-server:3000",
		DexGrpcAddress:      "dex:5557",
		AllowedClientsIds:   []string{"civil-os-dev", "cli"},
		InstanceMetadataUrl: "file:///etc/civil-os/instance-metadata.json",
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("LoadConfig = %+v\nwant %+v", cfg, want)
	}
}

func TestLoadConfigOptionalAndDefaults(t *testing.T) {
	env := testutil.WithEnv(testutil.SharedEnv(), map[string]string{
		"CIVIL_OS_GATEWAY_PORT":             "-",
		"CIVIL_OS_GATEWAY_LOG_LEVEL":        "-",
		"CIVIL_OS_GATEWAY_WRITER_HOST":      "-",
		"CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS": "-",
	})
	cfg, err := LoadConfig(envconfig.FromMap(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 || cfg.LogLevel != slog.LevelWarn || cfg.DBWriterHost != "" || cfg.DexGrpcAddress != "" {
		t.Errorf("got port %d, level %v, writer %q, dex %q; want 8080, WARN (shared), empty, empty",
			cfg.Port, cfg.LogLevel, cfg.DBWriterHost, cfg.DexGrpcAddress)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{"legacy variable", map[string]string{"CIVIL_AUTH_SERVER": "dex"}, "CIVIL_AUTH_SERVER"},
		{"missing auth server", map[string]string{"CIVIL_OS_GATEWAY_AUTH_SERVER": "-"}, "CIVIL_OS_GATEWAY_AUTH_SERVER"},
		{"missing reader", map[string]string{"CIVIL_OS_GATEWAY_READER_HOST": ""}, "CIVIL_OS_GATEWAY_READER_HOST"},
		{"missing metadata", map[string]string{"CIVIL_OS_GATEWAY_INSTANCE_METADATA_URL": "-"}, "CIVIL_OS_GATEWAY_INSTANCE_METADATA_URL"},
		{"client IDs not JSON", map[string]string{"CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS": "web,cli"}, "JSON array"},
		{"client IDs empty", map[string]string{"CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS": "[]"}, "empty"},
		{"bad port", map[string]string{"CIVIL_OS_GATEWAY_PORT": "0"}, "CIVIL_OS_GATEWAY_PORT"},
		{"bad log level", map[string]string{"CIVIL_OS_GATEWAY_LOG_LEVEL": "trace"}, "CIVIL_OS_GATEWAY_LOG_LEVEL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(envconfig.FromMap(testutil.WithEnv(testutil.SharedEnv(), tt.overrides)))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}
