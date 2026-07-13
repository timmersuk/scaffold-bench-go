package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body := strings.NewReader(`{"scenarioIds":["demo"],"modelId":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	req.Header.Set("content-type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d: %s", rr.Code, rr.Body.String())
	}
	if fr.startedID != "run-123" {
		t.Fatalf("runner.Start not called")
	}
}

func TestStopRunEndpoint(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/runs/run-42/stop", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}
	if fr.stoppedID != "run-42" {
		t.Fatalf("runner.Stop not called, got %q", fr.stoppedID)
	}
}

func TestScenariosEndpointReturnsDemo(t *testing.T) {
	store, events, registry, fr := testDependencies(t)
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/scenarios", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", rr.Code, rr.Body.String())
	}

	var infos []scenarioInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &infos); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	found := false
	for _, info := range infos {
		if info.ID == "demo" {
			found = true
			if info.Name != "create-hello" {
				t.Errorf("demo name = %q, want create-hello", info.Name)
			}
			if info.Category != "basic" {
				t.Errorf("demo category = %q, want basic", info.Category)
			}
			if info.Difficulty != "low" {
				t.Errorf("demo difficulty = %q, want low", info.Difficulty)
			}
			if info.MaxPoints != 10 {
				t.Errorf("demo maxPoints = %d, want 10", info.MaxPoints)
			}
			if info.Prompt == "" {
				t.Errorf("demo prompt should not be empty")
			}
			if info.Track != "execution" {
				t.Errorf("demo track = %q, want execution", info.Track)
			}
		}
	}
	if !found {
		t.Fatalf("demo scenario not found in response: %s", rr.Body.String())
	}
}

func TestModelsEndpointDiscoversRemoteModels(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "remote-discovered-a", "object": "model"},
				{"id": "remote-discovered-b", "object": "model"},
			},
		})
	}))
	defer remote.Close()

	store, events, registry, fr := testDependencies(t)
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:              "http://127.0.0.1:1",
			RemoteEndpoint:             remote.URL,
			RemoteAPIKey:               "secret",
			RemoteModels:               []string{"remote-static"},
			RemoteModelCacheTTLSeconds: 10,
		},
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
		t.Errorf("expected 0 local models, got %d", len(resp.Local))
	}
	if len(resp.Remote) != 3 {
		t.Fatalf("expected 3 remote models, got %d: %v", len(resp.Remote), resp.Remote)
	}

	ids := make([]string, len(resp.Remote))
	for i, m := range resp.Remote {
		ids[i] = m.ID
		if m.Source != "remote" {
			t.Errorf("model %q source = %q, want remote", m.ID, m.Source)
		}
		if m.Endpoint != remote.URL {
			t.Errorf("model %q endpoint = %q, want %s", m.ID, m.Endpoint, remote.URL)
		}
		if m.RequiresAPIKey {
			t.Errorf("model %q should not require API key when configured", m.ID)
		}
	}
	want := []string{"remote-discovered-a", "remote-discovered-b", "remote-static"}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("remote model[%d] = %q, want %q", i, ids[i], id)
		}
	}
}

func TestModelsEndpointRemoteDiscoversMergesWithStaticAndDedups(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "shared-model", "object": "model"},
				{"id": "dynamic-only", "object": "model"},
			},
		})
	}))
	defer remote.Close()

	store, events, registry, fr := testDependencies(t)
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:              "http://127.0.0.1:1",
			RemoteEndpoint:             remote.URL,
			RemoteModels:               []string{"shared-model", "static-only"},
			RemoteModelCacheTTLSeconds: 10,
		},
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
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:              "http://127.0.0.1:1",
			RemoteEndpoint:             "http://127.0.0.1:1",
			RemoteModels:               []string{"remote-fallback"},
			RemoteModelCacheTTLSeconds: 10,
		},
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
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:  local.URL,
			RemoteEndpoint: "https://api.remote.example.com",
			RemoteModels:   []string{"remote-model-1", "remote-model-2"},
		},
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
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	run := model.Run{
		ID:          "run-events",
		StartedAt:   1,
		Status:      model.RunRunning,
		ScenarioIDs: []string{"demo"},
		Runtime:     "local",
		RuntimeKind: "llama.cpp",
		Model:       "test-model",
	}
	if err := store.InsertRun(run); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := store.InsertEvent("run-events", "demo", 0, 1, model.EventScenarioStarted, map[string]any{"scenarioId": "demo"}); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-events/events", nil)
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
		t.Fatalf("expected 1 event, got %d", len(resp))
	}
	if resp[0]["type"] != model.EventScenarioStarted {
		t.Errorf("event type = %q, want %q", resp[0]["type"], model.EventScenarioStarted)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/runs/run-events/events?fromSeq=0", nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	var filtered []map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("decode fromSeq response: %v", err)
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 events with fromSeq=0, got %d", len(filtered))
	}
}

func TestModelsEndpointIncludesDisplayNames(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "local-model-a", "object": "model"},
				{"id": "Local Model B", "object": "model"},
				{"id": "alreadyfriend", "object": "model"},
			},
		})
	}))
	defer local.Close()

	store, events, registry, fr := testDependencies(t)
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:  local.URL,
			RemoteEndpoint: "https://api.remote.example.com",
			RemoteModels:   []string{"remote-model-1", "Friendly Remote"},
		},
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
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:  "http://127.0.0.1:1",
			RemoteEndpoint: "https://api.remote.example.com",
			RemoteModels:   []string{"remote-fallback"},
		},
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
	router, err := NewRouter(Config{
		Store:    store,
		Events:   events,
		Runner:   fr,
		Registry: registry,
		AppConfig: config.Config{
			LocalEndpoint:  "http://127.0.0.1:1",
			RemoteEndpoint: "https://api.remote.example.com",
			RemoteAPIKey:   "secret",
			RemoteModels:   []string{"remote-keyed"},
		},
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
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
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
	router, err := NewRouter(Config{
		Store:     store,
		Events:    events,
		Runner:    fr,
		Registry:  registry,
		AppConfig: config.Config{LocalEndpoint: "http://127.0.0.1:1"},
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

