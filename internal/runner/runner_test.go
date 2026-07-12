package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

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
