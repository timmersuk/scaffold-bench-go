package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestStartRunEndpoint(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/router.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	fr := &fakeRunner{}
	router, err := NewRouter(Config{
		Store:  store,
		Events: realtime.NewHub(),
		Runner: fr,
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
	store, err := storage.Open(t.TempDir() + "/router.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	fr := &fakeRunner{}
	router, err := NewRouter(Config{
		Store:  store,
		Events: realtime.NewHub(),
		Runner: fr,
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
