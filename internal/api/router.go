package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
	"github.com/timmersuk/scaffold-bench-go/internal/web"
)

// Config holds the dependencies needed by the HTTP router.
type Config struct {
	Store    *storage.Store
	Events   *realtime.Hub
	Runner   Runner
	BuildID  string
	Frontend fs.FS
}

// Runner orchestrates benchmark runs.
type Runner interface {
	Start(req runner.StartRequest) (string, error)
	Stop(runID string) error
	ActiveRunID() (string, bool)
}

// NewRouter builds the application http.Handler.
func NewRouter(cfg Config) (http.Handler, error) {
	if cfg.Store == nil {
		return nil, errors.New("store is required")
	}
	if cfg.Events == nil {
		return nil, errors.New("events hub is required")
	}
	if cfg.Runner == nil {
		return nil, errors.New("runner is required")
	}

	frontend, err := fs.Sub(web.Files, "dist")
	if err != nil {
		return nil, fmt.Errorf("sub frontend fs: %w", err)
	}

	srv := &server{
		store:    cfg.Store,
		events:   cfg.Events,
		runner:   cfg.Runner,
		buildID:  cfg.BuildID,
		frontend: frontend,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/scenarios", srv.withMethod(http.MethodGet, srv.handleScenarios))
	apiMux.HandleFunc("/models", srv.withMethod(http.MethodGet, srv.handleModels))
	apiMux.HandleFunc("/runs/active", srv.withMethod(http.MethodGet, srv.handleActiveRun))
	apiMux.HandleFunc("/runs/clear", srv.withMethod(http.MethodPost, srv.handleClearRuns))
	apiMux.HandleFunc("/runs", srv.withMethod(http.MethodPost, srv.handleStartRun))
	apiMux.HandleFunc("/runs/", srv.handleRuns)
	apiMux.HandleFunc("/oneshot/tests", srv.withMethod(http.MethodGet, srv.handleOneshotTests))
	apiMux.HandleFunc("/oneshot/runs/", srv.withMethod(http.MethodGet, srv.handleOneshotRuns))
	apiMux.HandleFunc("/report/data", srv.withMethod(http.MethodGet, srv.handleReportData))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.withMethod(http.MethodGet, srv.handleHealth))
	mux.Handle("/api/", http.StripPrefix("/api", apiMux))
	mux.HandleFunc("/", srv.handleFrontend)

	return mux, nil
}

type server struct {
	store    *storage.Store
	events   *realtime.Hub
	runner   Runner
	buildID  string
	frontend fs.FS
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"build_id": s.buildID,
	})
}

func (s *server) handleScenarios(w http.ResponseWriter, r *http.Request) {
	// TODO: load from scenario registry.
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (s *server) handleModels(w http.ResponseWriter, r *http.Request) {
	// TODO: discover local + remote models.
	writeJSON(w, http.StatusOK, map[string]any{
		"local":  []any{},
		"remote": []any{},
	})
}

func (s *server) handleActiveRun(w http.ResponseWriter, r *http.Request) {
	if id, ok := s.runner.ActiveRunID(); ok {
		writeJSON(w, http.StatusOK, map[string]any{"runId": id})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runId": nil})
}

func (s *server) handleClearRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleRuns(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	// parts: ["runs", "id", ...]
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing run id"})
		return
	}
	runID := parts[1]
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing run id"})
		return
	}

	// /api/runs/{id}/stream
	if len(parts) == 3 && parts[2] == "stream" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		s.handleRunStream(w, r, runID)
		return
	}
	// /api/runs/{id}/stop
	if len(parts) == 3 && parts[2] == "stop" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		s.handleStopRun(w, r, runID)
		return
	}
	// /api/runs/{id}/scenarios/{scenarioId}/events
	if len(parts) == 5 && parts[2] == "scenarios" && parts[4] == "events" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	// /api/runs/{id}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": runID})
		return
	}

	writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
}

func (s *server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	var req runner.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if len(req.ScenarioIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "scenarioIds is required"})
		return
	}
	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "model is required"})
		return
	}
	runID, err := s.runner.Start(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"runId": runID})
}

func (s *server) handleStopRun(w http.ResponseWriter, _ *http.Request, runID string) {
	if err := s.runner.Stop(runID); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleRunStream(w http.ResponseWriter, r *http.Request, runID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	events, unsubscribe := s.events.Subscribe()
	defer unsubscribe()

	if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			if e.RunID != runID {
				continue
			}
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) handleOneshotTests(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (s *server) handleOneshotRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleReportData(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"models": []any{},
		"categories": []any{},
		"difficulty": []any{},
	})
}

func (s *server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}

	if serveStaticFile(w, r, s.frontend, name, true) {
		return
	}
	serveStaticFile(w, r, s.frontend, "index.html", true)
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, files fs.FS, name string, noStoreIndex bool) bool {
	file, err := files.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil || stat.IsDir() {
		return false
	}

	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("read static asset", "name", name, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "read frontend asset"})
		return true
	}

	if noStoreIndex && name == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeContent(w, r, name, stat.ModTime(), bytes.NewReader(data))
	return true
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *server) withMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
