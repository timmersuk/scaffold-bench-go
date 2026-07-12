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

func TestListEvents(t *testing.T) {
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
		t.Fatalf("insert event 0: %v", err)
	}
	if err := store.InsertEvent("run-events", "demo", 1, 2, model.EventAssistantDelta, map[string]any{"content": "hi"}); err != nil {
		t.Fatalf("insert event 1: %v", err)
	}

	events, err := store.ListEvents("run-events", -1)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Seq != 0 || events[0].Type != model.EventScenarioStarted {
		t.Errorf("event[0].seq/type = %d/%q", events[0].Seq, events[0].Type)
	}
	if events[1].Seq != 1 || events[1].Type != model.EventAssistantDelta {
		t.Errorf("event[1].seq/type = %d/%q", events[1].Seq, events[1].Type)
	}
	payload, ok := events[1].Payload.(map[string]any)
	if !ok || payload["content"] != "hi" {
		t.Errorf("event[1].payload = %v", events[1].Payload)
	}

	slice, err := store.ListEvents("run-events", 0)
	if err != nil {
		t.Fatalf("list events from 0: %v", err)
	}
	if len(slice) != 1 {
		t.Fatalf("expected 1 event after seq 0, got %d", len(slice))
	}

	empty, err := store.ListEvents("not-found", -1)
	if err != nil {
		t.Fatalf("list events not found: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty events, got %d", len(empty))
	}
}
