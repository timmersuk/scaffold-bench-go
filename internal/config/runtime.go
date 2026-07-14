package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultRemoteModelCacheTTLSeconds is the default cache lifetime for remote model list discovery.
const DefaultRemoteModelCacheTTLSeconds = 10

// RuntimeFileName is the name of the persisted runtime configuration file.
const RuntimeFileName = "runtime-config.json"

// RuntimeConfigData is the plain, serializable runtime configuration values.
type RuntimeConfigData struct {
	LocalEndpoint              string   `json:"localEndpoint,omitempty"`
	RemoteEndpoint             string   `json:"remoteEndpoint,omitempty"`
	RemoteAPIKey               string   `json:"remoteApiKey,omitempty"`
	RemoteModels               []string `json:"remoteModels,omitempty"`
	RemoteModelCacheTTLSeconds int      `json:"remoteModelCacheTTLSeconds,omitempty"`
}

type RuntimeConfig interface {
	LocalEndpoint() string
	RemoteEndpoint() string
	RemoteAPIKey() string
	RemoteModels() []string
	RemoteModelCacheTTLSeconds() int

	Apply(update RuntimeConfigData) error
	Snapshot() RuntimeConfigData
}

// RuntimeConfig holds configuration values that can be changed at runtime and
// persisted to a JSON file in the data directory.
//
// The underlying values are accessed via the exported methods. Values are
// protected by an internal mutex for safe concurrent use.
type fileRuntimeConfig struct {
	mu   sync.RWMutex
	path string
	data RuntimeConfigData
}

// NewRuntimeConfig creates an loaded runtime configuration store for the given
// data directory.
func NewFileRuntimeConfig(dataDir string) (RuntimeConfig, error) {
	cfg := &fileRuntimeConfig{
		path: filepath.Join(dataDir, RuntimeFileName),
	}

	err := cfg.load()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime configuration: %w", err)
	}

	return cfg, nil
}

// Path returns the absolute file path used for persistence.
func (r *fileRuntimeConfig) Path() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.path
}

// Load reads the runtime configuration file from disk, merges it with the
// default values, and updates the in-memory state. Missing fields keep their
// default values; values present in the file override defaults.
//
// If the file does not exist, Load initializes the store with defaults and
// returns nil. If the file exists but cannot be decoded, Load returns an
// error.
func (r *fileRuntimeConfig) load() error {
	data := RuntimeConfigData{
		RemoteModelCacheTTLSeconds: DefaultRemoteModelCacheTTLSeconds,
	}

	bytes, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			r.mu.Lock()
			r.data = data
			r.mu.Unlock()
			return nil
		}
		return err
	}

	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	if data.RemoteModelCacheTTLSeconds == 0 {
		data.RemoteModelCacheTTLSeconds = DefaultRemoteModelCacheTTLSeconds
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = data
	return nil
}

// Snapshot returns a defensive copy of the current runtime configuration data.
func (r *fileRuntimeConfig) Snapshot() RuntimeConfigData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshotLocked()
}

// LocalEndpoint returns the currently configured local endpoint.
func (r *fileRuntimeConfig) LocalEndpoint() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data.LocalEndpoint
}

// RemoteEndpoint returns the currently configured remote endpoint.
func (r *fileRuntimeConfig) RemoteEndpoint() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data.RemoteEndpoint
}

// RemoteAPIKey returns the currently configured remote API key.
func (r *fileRuntimeConfig) RemoteAPIKey() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data.RemoteAPIKey
}

// RemoteModels returns a defensive copy of the currently configured remote models list.
func (r *fileRuntimeConfig) RemoteModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.data.RemoteModels))
	copy(out, r.data.RemoteModels)
	return out
}

// RemoteModelCacheTTLSeconds returns the currently configured cache TTL.
func (r *fileRuntimeConfig) RemoteModelCacheTTLSeconds() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data.RemoteModelCacheTTLSeconds == 0 {
		return DefaultRemoteModelCacheTTLSeconds
	}
	return r.data.RemoteModelCacheTTLSeconds
}

// Apply updates the runtime configuration values from a snapshot and saves to disk
func (r *fileRuntimeConfig) Apply(update RuntimeConfigData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.LocalEndpoint = update.LocalEndpoint
	r.data.RemoteEndpoint = update.RemoteEndpoint
	r.data.RemoteAPIKey = update.RemoteAPIKey
	r.data.RemoteModelCacheTTLSeconds = update.RemoteModelCacheTTLSeconds

	if update.RemoteModels == nil {
		r.data.RemoteModels = nil
	} else {
		models := make([]string, 0, len(update.RemoteModels))
		for _, m := range update.RemoteModels {
			m = strings.TrimSpace(m)
			if m != "" {
				models = append(models, m)
			}
		}
		r.data.RemoteModels = models
	}

	bytes, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(r.path, bytes, 0o644)
}

// snapshotLocked returns a defensive copy of the current runtime configuration data.
// The caller must hold at least a read lock.
func (r *fileRuntimeConfig) snapshotLocked() RuntimeConfigData {
	models := make([]string, len(r.data.RemoteModels))
	copy(models, r.data.RemoteModels)
	return RuntimeConfigData{
		LocalEndpoint:              r.data.LocalEndpoint,
		RemoteEndpoint:             r.data.RemoteEndpoint,
		RemoteAPIKey:               r.data.RemoteAPIKey,
		RemoteModels:               models,
		RemoteModelCacheTTLSeconds: r.data.RemoteModelCacheTTLSeconds,
	}
}

type staticRuntimeConfig struct {
	data RuntimeConfigData
}

func NewStaticRuntimeConfig(data RuntimeConfigData) RuntimeConfig {
	return &staticRuntimeConfig{data: data}
}

func (s *staticRuntimeConfig) LocalEndpoint() string  { return s.data.LocalEndpoint }
func (s *staticRuntimeConfig) RemoteEndpoint() string { return s.data.RemoteEndpoint }
func (s *staticRuntimeConfig) RemoteAPIKey() string   { return s.data.RemoteAPIKey }
func (s *staticRuntimeConfig) RemoteModels() []string { return s.data.RemoteModels }
func (s *staticRuntimeConfig) RemoteModelCacheTTLSeconds() int {
	return s.data.RemoteModelCacheTTLSeconds
}
func (s *staticRuntimeConfig) Snapshot() RuntimeConfigData { return s.data }

func (s *staticRuntimeConfig) Apply(update RuntimeConfigData) error {
	return errors.New("cannot apply updates to static runtime config")
}
