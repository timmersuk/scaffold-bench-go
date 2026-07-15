package storage

import (
	"path/filepath"
	"strings"
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
		Source:      "local",
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

func TestListRuns(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, id := range []string{"run-a", "run-b"} {
		if err := store.InsertRun(model.Run{
			ID:          id,
			StartedAt:   int64(strings.Compare(id, "run-a")+1) * 100,
			Status:      model.RunDone,
			ScenarioIDs: []string{"demo"},
			Runtime:     "local",
			RuntimeKind: "llama.cpp",
			Model:       "test-model",
			Source:      "local",
			TotalPoints: intPtr(5),
			MaxPoints:   intPtr(10),
		}); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	runs, err := store.ListRuns()
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	// Most recent first: run-b started at 200, run-a at 100.
	if runs[0].ID != "run-b" || runs[1].ID != "run-a" {
		t.Errorf("unexpected order: %q, %q", runs[0].ID, runs[1].ID)
	}
	if runs[0].TotalPoints == nil || *runs[0].TotalPoints != 5 {
		t.Errorf("run-b total points = %v, want 5", runs[0].TotalPoints)
	}
}

func TestGetRunWithScenarios(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := store.InsertRun(model.Run{
		ID:          "run-1",
		StartedAt:   1,
		Status:      model.RunDone,
		ScenarioIDs: []string{"demo"},
		Runtime:     "local",
		RuntimeKind: "llama.cpp",
		Model:       "test-model",
		Source:      "local",
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

	_, scenarios, err := store.GetRunWithScenarios("run-1")
	if err != nil {
		t.Fatalf("get run with scenarios: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	if scenarios[0].ScenarioID != "demo" {
		t.Errorf("scenario id = %q, want demo", scenarios[0].ScenarioID)
	}
	if scenarios[0].Points == nil || *scenarios[0].Points != 5 {
		t.Errorf("scenario points = %v, want 5", scenarios[0].Points)
	}
}

func intPtr(v int) *int { return &v }

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
		Source:      "local",
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
