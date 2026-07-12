package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

func TestRunBuildCommands(t *testing.T) {
	tmp := t.TempDir()

	// Write a small helper script so that path quoting is handled by argument
	// passing rather than shell escaping.
	var command string
	if runtime.GOOS == "windows" {
		script := filepath.Join(tmp, "build.bat")
		if err := os.WriteFile(script, []byte("@echo off\ntype nul > %1\\marker.txt\n"), 0o644); err != nil {
			t.Fatalf("write build.bat: %v", err)
		}
		command = script + ` {{.WorkDir}}`
	} else {
		script := filepath.Join(tmp, "build.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch \"$1/marker.txt\"\n"), 0o755); err != nil {
			t.Fatalf("write build.sh: %v", err)
		}
		command = `sh ` + script + ` {{.WorkDir}}`
	}

	build := &Build{
		Commands: []string{command},
		Env:      map[string]string{"SB_BUILD_TEST": "1"},
	}

	if err := runBuildCommands(context.Background(), tmp, build, defaultBuildTimeout); err != nil {
		t.Fatalf("runBuildCommands: %v", err)
	}

	marker := filepath.Join(tmp, "marker.txt")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
}

func TestRunBuildCommandsFailsOnError(t *testing.T) {
	build := &Build{Commands: []string{"exit 7"}}
	err := runBuildCommands(context.Background(), t.TempDir(), build, defaultBuildTimeout)
	if err == nil {
		t.Fatal("expected build command failure")
	}
	if !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineEndToEnd(t *testing.T) {
	calls := 0
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		calls++
		if calls == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"write\",\"arguments\":\"{\\\"path\\\":\\\"playground/hello.txt\\\",\\\"content\\\":\\\"hello\\\"}\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"done\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer modelServer.Close()

	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hub := realtime.NewHub()
	cfg := config.Config{DataDir: dir, LocalEndpoint: modelServer.URL}
	engine := NewEngine(store, hub, cfg, NewRegistry())

	runID, err := engine.Start(StartRequest{
		ScenarioIDs: []string{"demo"},
		Model:       "fake",
		Endpoint:    modelServer.URL,
		TimeoutMs:   30000,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var run model.Run
	for i := 0; i < 50; i++ {
		r, err := store.GetRun(runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if r.Status == "done" || r.Status == "failed" || r.Status == "stopped" {
			run = r
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if run.Status != "done" {
		t.Fatalf("run did not finish: status=%s", run.Status)
	}
	if run.TotalPoints == nil || *run.TotalPoints != 10 {
		t.Fatalf("expected 10 points, got %v", run.TotalPoints)
	}
	if !strings.HasSuffix(run.ReportPath, "-local.json") {
		t.Fatalf("expected report path, got %s", run.ReportPath)
	}
}

func TestMissingRequirement(t *testing.T) {
	tests := []struct {
		name     string
		requires []string
		want     string
	}{
		{"empty", nil, ""},
		{"present tool", []string{"go"}, ""},
		{"missing tool", []string{"sb-tool-that-should-not-exist"}, "sb-tool-that-should-not-exist"},
		{"first missing", []string{"go", "sb-tool-that-should-not-exist"}, "sb-tool-that-should-not-exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingRequirement(tt.requires); got != tt.want {
				t.Errorf("missingRequirement(%v) = %q, want %q", tt.requires, got, tt.want)
			}
		})
	}
}

func TestRunScenarioSkipsOnMissingRequirement(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hub := realtime.NewHub()
	cfg := config.Config{DataDir: dir}
	registry := &Registry{
		scenarios: map[string]Scenario{
			"missing-tool": {
				ID:         "missing-tool",
				Category:   "test",
				Family:     "test",
				MaxPoints:  10,
				RubricKind: "10pt",
				Manifest: Manifest{
					Requires: []string{"sb-tool-that-should-not-exist"},
				},
			},
		},
	}
	engine := NewEngine(store, hub, cfg, registry)
	scenario, _ := registry.Get("missing-tool")

	res := engine.runScenario(context.Background(), "run-1", StartRequest{Model: "fake"}, scenario, "", 0, &activeRun{})

	if res.Status != model.ScenarioSkipped {
		t.Errorf("status = %q, want %q", res.Status, model.ScenarioSkipped)
	}
	if res.Points != 0 {
		t.Errorf("points = %d, want 0", res.Points)
	}
	if res.MaxPoints != 0 {
		t.Errorf("maxPoints = %d, want 0", res.MaxPoints)
	}
	if !strings.Contains(res.Error, "sb-tool-that-should-not-exist") {
		t.Errorf("error = %q, want it to name missing tool", res.Error)
	}
	if !strings.Contains(res.Evaluation.Summary, "sb-tool-that-should-not-exist") {
		t.Errorf("evaluation summary = %q, want it to name missing tool", res.Evaluation.Summary)
	}
}
