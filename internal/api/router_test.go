package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

type fakeRunner struct {
	startedID string
	stoppedID string
	activeID  string
}

func (f *fakeRunner) Start(req runner.StartRequest) (string, error) {
	f.startedID = "run-123"
	return f.startedID, nil
}

func (f *fakeRunner) Stop(runID string) error {
	f.stoppedID = runID
	return nil
}

func (f *fakeRunner) ActiveRunID() (string, bool) {
	if f.activeID != "" {
		return f.activeID, true
	}
	return "", false
}

func testDependencies(tb testing.TB) (*storage.Store, *realtime.Hub, *runner.Registry, *fakeRunner) {
	tb.Helper()
	store, err := storage.Open(tb.TempDir() + "/router.db")
	if err != nil {
		tb.Fatalf("open store: %v", err)
	}
	if err := store.Migrate(); err != nil {
		store.Close()
		tb.Fatalf("migrate: %v", err)
	}
	tb.Cleanup(func() { store.Close() })
	return store, realtime.NewHub(), runner.NewRegistry(), &fakeRunner{}
}

func TestStartRunEndpoint(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteModels: []string{"shared-model", "dynamic-only", "static-only"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Remote) != 3 {
		t.Fatalf("expected 3 remote models after dedup, got %d: %v", len(resp.Remote), resp.Remote)
	}

	ids := make([]string, len(resp.Remote))
	for i, m := range resp.Remote {
		ids[i] = m.ID
	}
	want := []string{"shared-model", "dynamic-only", "static-only"}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("remote model[%d] = %q, want %q", i, ids[i], id)
		}
	}
}

func TestModelsEndpointRemoteDiscoveryFallsBackToStatic(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteModels: []string{"remote-fallback"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Remote) != 1 {
		t.Fatalf("expected 1 remote model, got %d", len(resp.Remote))
	}
	if resp.Remote[0].ID != "remote-fallback" {
		t.Errorf("remote model id = %q, want remote-fallback", resp.Remote[0].ID)
	}
}

func TestModelsEndpointReusesHTTPClientForLocalDiscovery(t *testing.T) {
	var mu sync.Mutex
	addrs := make(map[string]int)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		addrs[r.RemoteAddr]++
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "local-model-a", "object": "model"},
				{"id": "local-model-b", "object": "model"},
			},
		})
	}))
	defer local.Close()

	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		LocalEndpoint: local.URL,
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp modelsResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Local) != 2 {
			t.Fatalf("expected 2 local models, got %d", len(resp.Local))
		}
	}

	mu.Lock()
	distinct := len(addrs)
	mu.Unlock()
	if distinct != 1 {
		t.Fatalf("expected 1 reused TCP connection, got %d distinct RemoteAddr values", distinct)
	}
}

func TestModelsEndpointReusesHTTPClientForRemoteDiscovery(t *testing.T) {
	var mu sync.Mutex
	addrs := make(map[string]int)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		addrs[r.RemoteAddr]++
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "remote-model-a", "object": "model"},
			},
		})
	}))
	defer remote.Close()

	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteEndpoint: remote.URL,
		RemoteModels:   []string{"remote-model-b"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp modelsResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Remote) != 2 {
			t.Fatalf("expected 2 remote models, got %d", len(resp.Remote))
		}
	}

	mu.Lock()
	distinct := len(addrs)
	mu.Unlock()
	if distinct != 1 {
		t.Fatalf("expected 1 reused TCP connection, got %d distinct RemoteAddr values", distinct)
	}
}

func TestModelsEndpointDiscoversLocalAndRemote(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "local-model-a", "object": "model"},
				{"id": "local-model-b", "object": "model"},
			},
		})
	}))
	defer local.Close()

	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		LocalEndpoint:  local.URL,
		RemoteEndpoint: "https://api.remote.example.com",
		RemoteModels:   []string{"remote-model-1", "remote-model-2"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Local) != 2 {
		t.Fatalf("expected 2 local models, got %d", len(resp.Local))
	}
	if resp.Local[0].ID != "local-model-a" {
		t.Errorf("first local model id = %q, want local-model-a", resp.Local[0].ID)
	}
	if resp.Local[0].Source != "local" {
		t.Errorf("first local model source = %q, want local", resp.Local[0].Source)
	}
	if resp.Local[0].Endpoint != local.URL {
		t.Errorf("first local model endpoint = %q, want %s", resp.Local[0].Endpoint, local.URL)
	}

	if len(resp.Remote) != 2 {
		t.Fatalf("expected 2 remote models, got %d", len(resp.Remote))
	}
	if resp.Remote[0].ID != "remote-model-1" {
		t.Errorf("first remote model id = %q, want remote-model-1", resp.Remote[0].ID)
	}
	if resp.Remote[0].Source != "remote" {
		t.Errorf("first remote model source = %q, want remote", resp.Remote[0].Source)
	}
	if resp.Remote[0].Endpoint != "https://api.remote.example.com" {
		t.Errorf("first remote model endpoint = %q, want https://api.remote.example.com", resp.Remote[0].Endpoint)
	}
	if !resp.Remote[0].RequiresAPIKey {
		t.Errorf("first remote model should require API key")
	}
	if !resp.Remote[1].RequiresAPIKey {
		t.Errorf("second remote model should require API key")
	}
}

func TestRunEventsEndpointReturnsPersistedEvents(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteModels: []string{"local-model-a", "Local Model B", "alreadyfriend", "remote-model-1", "Friendly Remote"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantDisplay := map[string]string{
		"local-model-a":   "Local Model A",
		"Local Model B":   "",
		"alreadyfriend":   "Alreadyfriend",
		"remote-model-1":  "Remote Model 1",
		"Friendly Remote": "",
	}
	for _, m := range append(resp.Local, resp.Remote...) {
		want, ok := wantDisplay[m.ID]
		if !ok {
			continue
		}
		if m.DisplayName != want {
			t.Errorf("model %q displayName = %q, want %q", m.ID, m.DisplayName, want)
		}
		delete(wantDisplay, m.ID)
	}
	for id := range wantDisplay {
		t.Errorf("expected model %q in response", id)
	}
}

func TestModelsEndpointReturnsRemoteWhenLocalUnreachable(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteModels: []string{"remote-fallback"},
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Local) != 0 {
		t.Errorf("expected 0 local models when endpoint unreachable, got %d", len(resp.Local))
	}
	if len(resp.Remote) != 1 {
		t.Fatalf("expected 1 remote model, got %d", len(resp.Remote))
	}
	if resp.Remote[0].ID != "remote-fallback" {
		t.Errorf("remote model id = %q, want remote-fallback", resp.Remote[0].ID)
	}
	if resp.Remote[0].Source != "remote" {
		t.Errorf("remote model source = %q, want remote", resp.Remote[0].Source)
	}
	if !resp.Remote[0].RequiresAPIKey {
		t.Errorf("remote model should require API key")
	}
}

func TestModelsEndpointRemoteDoesNotRequireKeyWhenConfigured(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		RemoteModels: []string{"remote-model"},
		RemoteAPIKey: "some-api-key",
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Remote) != 1 {
		t.Fatalf("expected 1 remote model, got %d", len(resp.Remote))
	}
	if resp.Remote[0].RequiresAPIKey {
		t.Errorf("remote model should not require API key when configured")
	}
}

func TestListRunsEndpointReturnsRuns(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	if err := store.InsertRun(model.Run{
		ID:          "run-1",
		StartedAt:   1,
		Status:      model.RunDone,
		ScenarioIDs: []string{"demo"},
		Runtime:     "local",
		RuntimeKind: "llama.cpp",
		Model:       "test-model",
		TotalPoints: intPtr(5),
		MaxPoints:   intPtr(10),
	}); err != nil {
		t.Fatalf("insert run: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 run, got %d", len(resp))
	}
	if resp[0]["id"] != "run-1" {
		t.Errorf("run id = %v, want run-1", resp[0]["id"])
	}
	if resp[0]["status"] != "done" {
		t.Errorf("run status = %v, want done", resp[0]["status"])
	}
	if resp[0]["totalPoints"] != float64(5) {
		t.Errorf("run totalPoints = %v, want 5", resp[0]["totalPoints"])
	}
}

func TestGetRunEndpointReturnsRunWithScenarios(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	if err := store.InsertRun(model.Run{
		ID:          "run-1",
		StartedAt:   1,
		Status:      model.RunDone,
		ScenarioIDs: []string{"demo"},
		Runtime:     "local",
		RuntimeKind: "llama.cpp",
		Model:       "test-model",
		TotalPoints: intPtr(5),
		MaxPoints:   intPtr(10),
	}); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := store.UpsertScenarioRun(model.ScenarioRun{
		RunID:      "run-1",
		ScenarioID: "demo",
		Category:   "basic",
		Status:     model.ScenarioPass,
		Points:     intPtr(5),
		MaxPoints:  10,
		RubricKind: "10pt",
	}); err != nil {
		t.Fatalf("upsert scenario: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["id"] != "run-1" {
		t.Errorf("run id = %v, want run-1", resp["id"])
	}
	scenarios, ok := resp["scenarios"].([]any)
	if !ok || len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %v", resp["scenarios"])
	}
	scenario := scenarios[0].(map[string]any)
	if scenario["scenarioId"] != "demo" {
		t.Errorf("scenario id = %v, want demo", scenario["scenarioId"])
	}
	if scenario["points"] != float64(5) {
		t.Errorf("scenario points = %v, want 5", scenario["points"])
	}
}

func intPtr(v int) *int { return &v }

type mutableRuntimeConfig struct {
	data config.RuntimeConfigData
}

func newMutableRuntimeConfig(data config.RuntimeConfigData) config.RuntimeConfig {
	return &mutableRuntimeConfig{data: data}
}

func (m *mutableRuntimeConfig) LocalEndpoint() string                { return m.data.LocalEndpoint }
func (m *mutableRuntimeConfig) RemoteEndpoint() string               { return m.data.RemoteEndpoint }
func (m *mutableRuntimeConfig) RemoteAPIKey() string                 { return m.data.RemoteAPIKey }
func (m *mutableRuntimeConfig) RemoteModels() []string               { return m.data.RemoteModels }
func (m *mutableRuntimeConfig) RemoteModelCacheTTLSeconds() int      { return m.data.RemoteModelCacheTTLSeconds }
func (m *mutableRuntimeConfig) Snapshot() config.RuntimeConfigData   { return m.data }
func (m *mutableRuntimeConfig) Apply(update config.RuntimeConfigData) error {
	m.data = update
	return nil
}
func (m *mutableRuntimeConfig) ValidateEndpoints() error {
	if m.data.LocalEndpoint == "" && m.data.RemoteEndpoint == "" {
		return errors.New("at least one endpoint must be configured")
	}
	return nil
}

func TestGetConfigEndpoint(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := config.NewStaticRuntimeConfig(config.RuntimeConfigData{
		LocalEndpoint:              "http://localhost:8080",
		RemoteEndpoint:             "https://api.remote.example.com",
		RemoteAPIKey:               "test-key",
		RemoteModels:               []string{"model-a", "model-b"},
		RemoteModelCacheTTLSeconds: 30,
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp runtimeConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LocalEndpoint != "http://localhost:8080" {
		t.Errorf("localEndpoint = %q, want http://localhost:8080", resp.LocalEndpoint)
	}
	if resp.RemoteEndpoint != "https://api.remote.example.com" {
		t.Errorf("remoteEndpoint = %q, want https://api.remote.example.com", resp.RemoteEndpoint)
	}
	if resp.RemoteAPIKey != "test-key" {
		t.Errorf("remoteApiKey = %q, want test-key", resp.RemoteAPIKey)
	}
	if len(resp.RemoteModels) != 2 || resp.RemoteModels[0] != "model-a" || resp.RemoteModels[1] != "model-b" {
		t.Errorf("remoteModels = %v, want [model-a model-b]", resp.RemoteModels)
	}
	if resp.RemoteModelCacheTTLSeconds != 30 {
		t.Errorf("remoteModelCacheTTLSeconds = %d, want 30", resp.RemoteModelCacheTTLSeconds)
	}
}

func TestUpdateConfigEndpoint(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	rc := newMutableRuntimeConfig(config.RuntimeConfigData{
		LocalEndpoint: "http://localhost:8080",
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body := `{"localEndpoint":"http://new-local:9090","remoteEndpoint":"https://new-remote","remoteApiKey":"new-key","remoteModels":["new-model"],"remoteModelCacheTTLSeconds":60}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp runtimeConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LocalEndpoint != "http://new-local:9090" {
		t.Errorf("localEndpoint = %q, want http://new-local:9090", resp.LocalEndpoint)
	}
	if resp.RemoteEndpoint != "https://new-remote" {
		t.Errorf("remoteEndpoint = %q, want https://new-remote", resp.RemoteEndpoint)
	}
	if resp.RemoteAPIKey != "new-key" {
		t.Errorf("remoteApiKey = %q, want new-key", resp.RemoteAPIKey)
	}
	if len(resp.RemoteModels) != 1 || resp.RemoteModels[0] != "new-model" {
		t.Errorf("remoteModels = %v, want [new-model]", resp.RemoteModels)
	}
	if resp.RemoteModelCacheTTLSeconds != 60 {
		t.Errorf("remoteModelCacheTTLSeconds = %d, want 60", resp.RemoteModelCacheTTLSeconds)
	}
}

func TestUpdateConfigRejectedWhenActive(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	fr.activeID = "run-123"
	rc := newMutableRuntimeConfig(config.RuntimeConfigData{
		LocalEndpoint: "http://localhost:8080",
	})
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{Runtime: rc},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body := `{"localEndpoint":"http://new-local:9090"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(resp.Error, "active") {
		t.Errorf("error = %q, want it to mention active run", resp.Error)
	}
}
