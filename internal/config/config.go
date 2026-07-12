package config

import (
	"errors"
	"os"
	"strings"
)

// Config is populated exclusively from environment variables.
type Config struct {
	HTTPAddr string
	DBPath   string
	DataDir  string

	// Model endpoints (see scope decision: generic OpenAI-compatible).
	LocalEndpoint  string
	RemoteEndpoint string
	RemoteAPIKey   string
	RemoteModels   []string
}

func FromEnv() (Config, error) {
	local := strings.TrimSpace(os.Getenv("BENCH_LOCAL_ENDPOINT"))
	if local == "" {
		local = "http://127.0.0.1:8082"
	}

	remote := strings.TrimSpace(os.Getenv("BENCH_REMOTE_ENDPOINT"))
	remoteKey := os.Getenv("BENCH_REMOTE_API_KEY")

	rawRemoteModels := os.Getenv("BENCH_REMOTE_MODELS")
	var remoteModels []string
	if rawRemoteModels != "" {
		for _, m := range strings.Split(rawRemoteModels, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				remoteModels = append(remoteModels, m)
			}
		}
	}

	cfg := Config{
		HTTPAddr:       envDefault("BENCH_HTTP_ADDR", ":8080"),
		DBPath:         envDefault("BENCH_DB_PATH", "data/scaffold-bench.db"),
		DataDir:        envDefault("BENCH_DATA_DIR", "data"),
		LocalEndpoint:  local,
		RemoteEndpoint: remote,
		RemoteAPIKey:   remoteKey,
		RemoteModels:   remoteModels,
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
