// Package envconfig reads civil-os service configuration from environment
// variables named CIVIL_OS_<SERVICE>_<SETTING> (spec §5.5), so every service
// can share one environment without collisions.
package envconfig

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Prefix is the namespace for every civil-os environment variable.
const Prefix = "CIVIL_OS_"

// legacyPrefix is the pre-v0.2.0 namespace. Any variable using it is rejected.
const legacyPrefix = "CIVIL_"

// Service names, used as the <SERVICE> segment of variable names.
const (
	Gateway    = "GATEWAY"
	Reader     = "READER"
	Writer     = "WRITER"
	TileServer = "TILE_SERVER"
)

// Env is a view of the process environment. Use FromOS in services and
// FromMap in tests.
type Env struct {
	lookup  func(string) (string, bool)
	environ func() []string
}

// FromOS returns the real process environment.
func FromOS() Env {
	return Env{lookup: os.LookupEnv, environ: os.Environ}
}

// FromMap returns an environment backed by m (for tests).
func FromMap(m map[string]string) Env {
	return Env{
		lookup: func(k string) (string, bool) { v, ok := m[k]; return v, ok },
		environ: func() []string {
			out := make([]string, 0, len(m))
			for k, v := range m {
				out = append(out, k+"="+v)
			}
			return out
		},
	}
}

// Key returns the variable name for a service setting, e.g.
// Key(Reader, "DB_HOST") == "CIVIL_OS_READER_DB_HOST".
func Key(service, setting string) string {
	return Prefix + service + "_" + setting
}

// Get returns the value of key, or "" when unset.
func (e Env) Get(key string) string {
	v, _ := e.lookup(key)
	return v
}

// RejectLegacy returns an error naming every set variable that uses the old
// CIVIL_ namespace (anything starting with CIVIL_ but not CIVIL_OS_). Services
// call it first, so a half-migrated deployment fails at startup instead of
// silently running with defaults.
func (e Env) RejectLegacy() error {
	var legacy []string
	for _, kv := range e.environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, legacyPrefix) && !strings.HasPrefix(name, Prefix) {
			legacy = append(legacy, name)
		}
	}
	if len(legacy) == 0 {
		return nil
	}
	slices.Sort(legacy)
	return fmt.Errorf("unsupported legacy environment variables %s: civil-os now reads CIVIL_OS_<SERVICE>_<SETTING> names; see the configuration table in docs/architecture.md",
		strings.Join(legacy, ", "))
}

// Require returns an error listing every key that is unset or empty.
func (e Env) Require(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if e.Get(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

// LogLevel returns the service's log level: CIVIL_OS_<SERVICE>_LOG_LEVEL if
// set, else CIVIL_OS_LOG_LEVEL, else INFO. Values follow slog.Level's text
// form: DEBUG, INFO, WARN, ERROR in any case, optionally with an offset such
// as INFO+2 or DEBUG-4.
func (e Env) LogLevel(service string) (slog.Level, error) {
	for _, key := range []string{Key(service, "LOG_LEVEL"), Prefix + "LOG_LEVEL"} {
		value := e.Get(key)
		if value == "" {
			continue
		}
		var level slog.Level
		if err := level.UnmarshalText([]byte(value)); err != nil {
			return 0, fmt.Errorf("invalid %s %q: want DEBUG, INFO, WARN or ERROR, optionally with an offset like INFO+2", key, value)
		}
		return level, nil
	}
	return slog.LevelInfo, nil
}

// Port returns key parsed as a TCP port, or def when unset.
func (e Env) Port(key string, def uint16) (uint16, error) {
	value := e.Get(key)
	if value == "" {
		return def, nil
	}
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return 0, fmt.Errorf("invalid %s %q: want a port number between 1 and 65535", key, value)
	}
	return uint16(port), nil
}
