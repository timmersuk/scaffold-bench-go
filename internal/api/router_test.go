package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
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

	body := strings.NewReader(`{"scenarioIds":["demo"],"model":"test"}`)
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
