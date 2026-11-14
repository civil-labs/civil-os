package envconfig

import (
	"log/slog"
	"strings"
	"testing"
)

func TestKey(t *testing.T) {
	if got := Key(TileServer, "DATABASE_URL"); got != "CIVIL_OS_TILE_SERVER_DATABASE_URL" {
		t.Errorf("Key = %q", got)
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want slog.Level
	}{
		{"default", nil, slog.LevelInfo},
		{"shared", map[string]string{"CIVIL_OS_LOG_LEVEL": "WARN"}, slog.LevelWarn},
		{"lower case", map[string]string{"CIVIL_OS_LOG_LEVEL": "debug"}, slog.LevelDebug},
		{"mixed case", map[string]string{"CIVIL_OS_LOG_LEVEL": "Error"}, slog.LevelError},
		{"positive offset", map[string]string{"CIVIL_OS_LOG_LEVEL": "INFO+2"}, slog.LevelInfo + 2},
		{"negative offset", map[string]string{"CIVIL_OS_LOG_LEVEL": "DEBUG-4"}, slog.LevelDebug - 4},
		{"service override", map[string]string{"CIVIL_OS_LOG_LEVEL": "ERROR", "CIVIL_OS_READER_LOG_LEVEL": "DEBUG"}, slog.LevelDebug},
		{"other service's override ignored", map[string]string{"CIVIL_OS_LOG_LEVEL": "WARN", "CIVIL_OS_GATEWAY_LOG_LEVEL": "DEBUG"}, slog.LevelWarn},
		{"empty override falls through", map[string]string{"CIVIL_OS_LOG_LEVEL": "WARN", "CIVIL_OS_READER_LOG_LEVEL": ""}, slog.LevelWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromMap(tt.env).LogLevel(Reader)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("LogLevel = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogLevelInvalid(t *testing.T) {
	for _, env := range []map[string]string{
		{"CIVIL_OS_LOG_LEVEL": "verbose"},
		{"CIVIL_OS_LOG_LEVEL": "INFO", "CIVIL_OS_WRITER_LOG_LEVEL": "true"},
	} {
		_, err := FromMap(env).LogLevel(Writer)
		if err == nil {
			t.Errorf("%v: expected an error", env)
			continue
		}
		if !strings.Contains(err.Error(), "LOG_LEVEL") {
			t.Errorf("error %q does not name the variable", err)
		}
	}
}

func TestRejectLegacy(t *testing.T) {
	ok := FromMap(map[string]string{"CIVIL_OS_READER_DB_HOST": "db", "PATH": "/bin", "CIVILIAN": "x"})
	if err := ok.RejectLegacy(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bad := FromMap(map[string]string{"CIVIL_OS_READER_DB_HOST": "db", "CIVIL_DB_HOST": "db", "CIVIL_VERBOSE": "1"})
	err := bad.RejectLegacy()
	if err == nil {
		t.Fatal("expected an error for legacy variables")
	}
	if !strings.Contains(err.Error(), "CIVIL_DB_HOST, CIVIL_VERBOSE") {
		t.Errorf("error should list legacy names sorted: %v", err)
	}
	if strings.Contains(err.Error(), "CIVIL_OS_READER_DB_HOST") {
		t.Errorf("error should not list CIVIL_OS_ names: %v", err)
	}
}

func TestRequire(t *testing.T) {
	env := FromMap(map[string]string{"A": "1", "B": ""})
	if err := env.Require("A"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := env.Require("A", "B", "C")
	if err == nil || !strings.Contains(err.Error(), "B, C") {
		t.Errorf("want error listing B, C; got %v", err)
	}
}

func TestPort(t *testing.T) {
	env := FromMap(map[string]string{"P": "8081", "ZERO": "0", "BIG": "65536", "TEXT": "http", "NEG": "-1"})
	if got, err := env.Port("P", 1); err != nil || got != 8081 {
		t.Errorf("Port(P) = %d, %v", got, err)
	}
	if got, err := env.Port("UNSET", 50051); err != nil || got != 50051 {
		t.Errorf("Port(UNSET) = %d, %v; want default", got, err)
	}
	for _, k := range []string{"ZERO", "BIG", "TEXT", "NEG"} {
		if _, err := env.Port(k, 1); err == nil {
			t.Errorf("Port(%s): expected an error", k)
		}
	}
}
