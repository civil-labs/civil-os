package testutil

// SharedEnv returns one environment holding every service's variables, as a
// compose file or ConfigMap shared by all services would. Each service's
// config tests load from it to prove they read only their own names.
func SharedEnv() map[string]string {
	return map[string]string{
		"CIVIL_OS_LOG_LEVEL": "WARN",

		"CIVIL_OS_GATEWAY_PORT":                  "8081",
		"CIVIL_OS_GATEWAY_LOG_LEVEL":             "DEBUG",
		"CIVIL_OS_GATEWAY_AUTH_SERVER":           "http://dex:5556",
		"CIVIL_OS_GATEWAY_IDP_HOST":              "dex:5556",
		"CIVIL_OS_GATEWAY_READER_HOST":           "http://database-reader:50051",
		"CIVIL_OS_GATEWAY_WRITER_HOST":           "http://database-writer:50052",
		"CIVIL_OS_GATEWAY_TILE_SERVER_HOST":      "http://tile-server:3000",
		"CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS":      "dex:5557",
		"CIVIL_OS_GATEWAY_ALLOWED_CLIENT_IDS":    `["civil-os-dev","cli"]`,
		"CIVIL_OS_GATEWAY_INSTANCE_METADATA_URL": "file:///etc/civil-os/instance-metadata.json",

		"CIVIL_OS_READER_DB_HOST": "reader-db",
		"CIVIL_OS_READER_DB_PORT": "5433",
		"CIVIL_OS_READER_DB_USER": "reader",
		"CIVIL_OS_READER_DB_PASS": "reader-pass",
		"CIVIL_OS_READER_DB_NAME": "civil",

		"CIVIL_OS_WRITER_PORT":      "50062",
		"CIVIL_OS_WRITER_LOG_LEVEL": "error",
		"CIVIL_OS_WRITER_DB_HOST":   "writer-db",
		"CIVIL_OS_WRITER_DB_USER":   "writer",
		"CIVIL_OS_WRITER_DB_PASS":   "writer-pass",
		"CIVIL_OS_WRITER_DB_NAME":   "civil",

		"CIVIL_OS_TILE_SERVER_DATABASE_URL": "postgres://tiles@db/civil",
	}
}

// WithEnv returns a copy of base with overrides applied; an override with
// value "-" deletes the key.
func WithEnv(base map[string]string, overrides map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		if v == "-" {
			delete(out, k)
		} else {
			out[k] = v
		}
	}
	return out
}
