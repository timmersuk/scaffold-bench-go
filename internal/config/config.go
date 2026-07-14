package config

import (
	"errors"
	"os"
	"strings"
)

// EnvConfig holds configuration values that can only be provided by environment
// variables. They are not editable at runtime.
type EnvConfig struct {
	HTTPAddr string
	DBPath   string
	DataDir  string
}

// Config is the complete effective configuration for the process.
// It embeds EnvConfig for direct field access to environment-only values, and
// carries a pointer to the live RuntimeConfig for runtime-editable values.
//
// Read the runtime values via the forwarding methods such as LocalEndpoint()
// so that the current persisted state is returned under the runtime lock.
type Config struct {
	EnvConfig
	Runtime RuntimeConfig
}

// LocalEndpoint returns the current local endpoint from runtime configuration.
func (c Config) LocalEndpoint() string {
	return c.Runtime.LocalEndpoint()
}

// RemoteEndpoint returns the current remote endpoint from runtime configuration.
func (c Config) RemoteEndpoint() string {
	return c.Runtime.RemoteEndpoint()
}

// RemoteAPIKey returns the current remote API key from runtime configuration.
func (c Config) RemoteAPIKey() string {
	return c.Runtime.RemoteAPIKey()
}

// RemoteModels returns the current remote model list from runtime configuration.
func (c Config) RemoteModels() []string {
	return c.Runtime.RemoteModels()
}

// RemoteModelCacheTTLSeconds returns the current remote model cache TTL.
func (c Config) RemoteModelCacheTTLSeconds() int {
	return c.Runtime.RemoteModelCacheTTLSeconds()
}

// SnapshotRuntime returns a snapshot of the current runtime configuration data.
func (c Config) SnapshotRuntime() RuntimeConfigData {
	return c.Runtime.Snapshot()
}

// FromEnv loads environment-only configuration. Runtime values must be loaded
// separately via a RuntimeConfig created with NewRuntimeConfig.
func FromEnv() (Config, error) {
	cfg := Config{
		EnvConfig: EnvConfig{
			HTTPAddr: envDefault("BENCH_HTTP_ADDR", ":8080"),
			DBPath:   envDefault("BENCH_DB_PATH", "data/scaffold-bench.db"),
			DataDir:  envDefault("BENCH_DATA_DIR", "data"),
		},
	}

	if cfg.HTTPAddr == "" {
		return Config{}, errors.New("BENCH_HTTP_ADDR cannot be empty")
	}
	return cfg, nil
}

func envDefault(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return strings.TrimSpace(value)
}
