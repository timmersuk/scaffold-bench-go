package storage

import (
	"path/filepath"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

func TestRunLifecycle(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	run := model.Run{
		ID:          "run-1",
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

	finishedAt := int64(2)
	run.Status = model.RunDone
	run.FinishedAt = &finishedAt
	if err := store.UpdateRun(run); err != nil {
		t.Fatalf("update run: %v", err)
	}

	sr := model.ScenarioRun{
		RunID:      "run-1",
		ScenarioID: "demo",
		Status:     model.ScenarioPass,
		MaxPoints:  10,
		RubricKind: "10pt",
	}
	if err := store.UpsertScenarioRun(sr); err != nil {
		t.Fatalf("upsert scenario run: %v", err)
	}

	if err := store.InsertEvent("run-1", "demo", 0, 1, model.EventScenarioStarted, map[string]any{"scenarioId": "demo"}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
}
