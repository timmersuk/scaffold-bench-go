package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/oneshot"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
	"github.com/timmersuk/scaffold-bench-go/internal/web"
)

// Config holds the dependencies needed by the HTTP router.
type Config struct {
	Store        *storage.Store
	Events       *realtime.Hub
	Runner       Runner
	OneshotRunner OneshotRunner
	Registry     *runner.Registry
	AppConfig    config.Config
	BuildID      string
	Frontend     fs.FS
}

// Runner orchestrates benchmark runs.
type Runner interface {
	Start(req runner.StartRequest) (string, error)
	Stop(runID string) error
	ActiveRunID() (string, bool)
}

// OneshotRunner orchestrates one-shot lab runs.
type OneshotRunner interface {
	Start(req oneshot.StartRequest) (string, error)
	Stop(runID string) error
	ActiveRunID() (string, bool)
	ListPrompts() ([]oneshot.PromptSummary, error)
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
	if cfg.OneshotRunner == nil {
		return nil, errors.New("oneshot runner is required")
	}
	if cfg.Registry == nil {
		return nil, errors.New("registry is required")
	}
	if cfg.AppConfig.Runtime == nil {
		return nil, errors.New("runtime config is required")
	}

	frontend, err := fs.Sub(web.Files, "dist")
	if err != nil {
		return nil, fmt.Errorf("sub frontend fs: %w", err)
	}

	srv := &server{
		store:         cfg.Store,
		events:        cfg.Events,
		runner:        cfg.Runner,
		oneshotRunner: cfg.OneshotRunner,
		registry:      cfg.Registry,
		appConfig:     cfg.AppConfig,
		buildID:       cfg.BuildID,
		frontend:      frontend,
		modelsCache: &modelsCache{
			ttl: cfg.AppConfig.RemoteModelCacheTTLSeconds(),
		},
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/scenarios", srv.withMethod(http.MethodGet, srv.handleScenarios))
	apiMux.HandleFunc("/models", srv.withMethod(http.MethodGet, srv.handleModels))
	apiMux.HandleFunc("/config", srv.handleConfig)
	apiMux.HandleFunc("/runs/active", srv.withMethod(http.MethodGet, srv.handleActiveRun))
	apiMux.HandleFunc("/runs/clear", srv.withMethod(http.MethodPost, srv.handleClearRuns))
	apiMux.HandleFunc("/runs", srv.handleRuns)
	apiMux.HandleFunc("/runs/", srv.handleRuns)
	apiMux.HandleFunc("/oneshot/tests", srv.withMethod(http.MethodGet, srv.handleOneshotTests))
	apiMux.HandleFunc("/oneshot/runs/latest", srv.withMethod(http.MethodGet, srv.handleOneshotLatestRun))
	apiMux.HandleFunc("/oneshot/runs", srv.handleOneshotRuns)
	apiMux.HandleFunc("/oneshot/runs/", srv.handleOneshotRuns)
	apiMux.HandleFunc("/oneshot/artifacts/", srv.withMethod(http.MethodGet, srv.handleOneshotArtifact))
	apiMux.HandleFunc("/report/data", srv.withMethod(http.MethodGet, srv.handleReportData))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.withMethod(http.MethodGet, srv.handleHealth))
	mux.Handle("/api/", http.StripPrefix("/api", loggingMiddleware(apiMux)))
	mux.HandleFunc("/", srv.handleFrontend)

	return mux, nil
}

type server struct {
	store         *storage.Store
	events        *realtime.Hub
	runner        Runner
	oneshotRunner OneshotRunner
	registry      *runner.Registry
	appConfig     config.Config
	buildID       string
	frontend      fs.FS
	modelsCache   *modelsCache
	httpClient    *http.Client
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"build_id": s.buildID,
	})
}

func (s *server) handleScenarios(w http.ResponseWriter, r *http.Request) {
	ids := s.registry.IDs()
	infos := make([]scenarioInfo, 0, len(ids))
	for _, id := range ids {
		sc, ok := s.registry.Get(id)
		if !ok {
			continue
		}
		track := sc.Manifest.Meta.Track
		if track == "" {
			track = "execution"
		}
		infos = append(infos, scenarioInfo{
			ID:         sc.ID,
			Name:       sc.Name,
			Category:   sc.Category,
			Difficulty: sc.Manifest.Meta.Difficulty,
			MaxPoints:  sc.MaxPoints,
			Prompt:     sc.Prompt,
			Track:      track,
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handleUpdateConfig(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	}
}

func (s *server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.appConfig.Runtime.Snapshot()
	writeJSON(w, http.StatusOK, runtimeConfigResponse{
		LocalEndpoint:              cfg.LocalEndpoint,
		RemoteEndpoint:             cfg.RemoteEndpoint,
		RemoteAPIKey:               cfg.RemoteAPIKey,
		RemoteModels:               cfg.RemoteModels,
		RemoteModelCacheTTLSeconds: cfg.RemoteModelCacheTTLSeconds,
	})
}

func (s *server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.runner.ActiveRunID(); ok {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "cannot update runtime configuration while a run is active"})
		return
	}

	var req updateRuntimeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	update := config.RuntimeConfigData{
		LocalEndpoint:              req.LocalEndpoint,
		RemoteEndpoint:             req.RemoteEndpoint,
		RemoteAPIKey:               req.RemoteAPIKey,
		RemoteModels:               req.RemoteModels,
		RemoteModelCacheTTLSeconds: req.RemoteModelCacheTTLSeconds,
	}
	err := s.appConfig.Runtime.Apply(update)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to persist runtime configuration"})
		return
	}

	// Clear cached model lists so the next /api/models call reflects the new endpoints and keys.
	s.modelsCache.clear()

	writeJSON(w, http.StatusOK, runtimeConfigResponse{
		LocalEndpoint:              update.LocalEndpoint,
		RemoteEndpoint:             update.RemoteEndpoint,
		RemoteAPIKey:               update.RemoteAPIKey,
		RemoteModels:               update.RemoteModels,
		RemoteModelCacheTTLSeconds: update.RemoteModelCacheTTLSeconds,
	})
}

func (s *server) handleModels(w http.ResponseWriter, r *http.Request) {
	local := s.discoverLocalModels(r.Context())
	remote := s.discoverRemoteModels(r.Context())
	writeJSON(w, http.StatusOK, modelsResponse{
		Local:  local,
		Remote: remote,
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

	// /api/runs (collection)
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleListRuns(w, r)
		case http.MethodPost:
			s.handleStartRun(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		}
		return
	}

	// parts: ["runs", id, ...]
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
	// /api/runs/{id}/events
	if len(parts) == 3 && parts[2] == "events" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		s.handleRunEvents(w, r, runID)
		return
	}
	// /api/runs/{id}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		s.handleGetRun(w, r, runID)
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
	if req.ModelID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "modelId is required"})
		return
	}
	if !s.eitherEndpointConfigured() {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "either local or remote endpoint must be configured"})
		return
	}
	runID, err := s.runner.Start(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"runId": runID})
}

func (s *server) eitherEndpointConfigured() bool {
	rc := s.appConfig.Runtime.Snapshot()
	return rc.LocalEndpoint != "" || rc.RemoteEndpoint != ""
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

func (s *server) handleRunEvents(w http.ResponseWriter, r *http.Request, runID string) {
	fromSeq := int64(-1)
	if raw := r.URL.Query().Get("fromSeq"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid fromSeq"})
			return
		}
		fromSeq = v
	}

	events, err := s.store.ListEvents(runID, fromSeq)
	if err != nil {
		slog.Error("list run events", "run_id", runID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list events"})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListRuns()
	if err != nil {
		slog.Error("list runs", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list runs"})
		return
	}
	resp := make([]runSummary, 0, len(runs))
	for _, run := range runs {
		resp = append(resp, runSummary{
			ID:          run.ID,
			StartedAt:   run.StartedAt,
			FinishedAt:  run.FinishedAt,
			Status:      string(run.Status),
			Model:       run.Model,
			TotalPoints: zeroIfNil(run.TotalPoints),
			MaxPoints:   zeroIfNil(run.MaxPoints),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleGetRun(w http.ResponseWriter, r *http.Request, runID string) {
	run, scenarios, err := s.store.GetRunWithScenarios(runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "run not found"})
			return
		}
		slog.Error("get run with scenarios", "run_id", runID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to get run"})
		return
	}

	scenarioInfos := make([]scenarioResult, 0, len(scenarios))
	for _, sr := range scenarios {
		scenarioInfos = append(scenarioInfos, scenarioResult{
			ScenarioID:    sr.ScenarioID,
			Category:      sr.Category,
			Family:        sr.Family,
			Status:        string(sr.Status),
			Points:        zeroIfNil(sr.Points),
			MaxPoints:     sr.MaxPoints,
			WallTimeMs:    sr.WallTimeMs,
			FirstTokenMs:  sr.FirstTokenMs,
			ToolCallCount: zeroIfNil(sr.ToolCallCount),
			ModelMetrics:  jsonRaw(sr.ModelMetricsJSON),
			Evaluation:    jsonRaw(sr.EvaluationJSON),
			RubricKind:    sr.RubricKind,
			Breakdown:     breakdown(&sr),
			Error:         sr.Error,
		})
	}

	writeJSON(w, http.StatusOK, runDetail{
		ID:          run.ID,
		StartedAt:   run.StartedAt,
		FinishedAt:  run.FinishedAt,
		Status:      string(run.Status),
		Model:       run.Model,
		TotalPoints: zeroIfNil(run.TotalPoints),
		MaxPoints:   zeroIfNil(run.MaxPoints),
		Scenarios:   scenarioInfos,
	})
}

func (s *server) handleOneshotTests(w http.ResponseWriter, r *http.Request) {
	prompts, err := s.oneshotRunner.ListPrompts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "list prompts: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, prompts)
}

func (s *server) handleOneshotRuns(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/oneshot/runs")
	path = strings.TrimPrefix(path, "/")

	if r.Method == http.MethodPost && path == "" {
		s.startOneshotRun(w, r)
		return
	}

	if r.Method == http.MethodPost && strings.HasSuffix(path, "/stop") {
		runID := strings.TrimSuffix(path, "/stop")
		s.stopOneshotRun(w, r, runID)
		return
	}

	if r.Method == http.MethodGet && strings.HasSuffix(path, "/stream") {
		runID := strings.TrimSuffix(path, "/stream")
		s.streamOneshotRun(w, r, runID)
		return
	}

	writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
}

func (s *server) startOneshotRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID   string   `json:"modelId"`
		PromptIDs []string `json:"promptIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if req.ModelID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "modelId is required"})
		return
	}
	if len(req.PromptIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "promptIds is required"})
		return
	}

	oneshotReq := oneshot.StartRequest{
		ModelID:   req.ModelID,
		PromptIDs: req.PromptIDs,
	}
	runID, err := s.oneshotRunner.Start(oneshotReq)
	if err != nil {
		if strings.Contains(err.Error(), "already in progress") {
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"runId": runID})
}

func (s *server) stopOneshotRun(w http.ResponseWriter, r *http.Request, runID string) {
	if err := s.oneshotRunner.Stop(runID); err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "runId": runID, "status": "stopping"})
}

func (s *server) streamOneshotRun(w http.ResponseWriter, r *http.Request, runID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := s.events.Subscribe()
	defer unsub()

	fromSeq := int64(-1)
	if q := r.URL.Query().Get("fromSeq"); q != "" {
		if v, err := strconv.ParseInt(q, 10, 64); err == nil {
			fromSeq = v
		}
	}

	events, err := s.store.ListOneshotEvents(runID, fromSeq)
	if err != nil {
		slog.Error("failed to list oneshot events", "run_id", runID, "err", err)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	for _, e := range events {
		if e.RunID != runID {
			continue
		}
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if e.RunID != runID {
				continue
			}
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if e.Type == model.EventOneshotRunFinished || e.Type == model.EventOneshotRunFailed || e.Type == model.EventOneshotRunStopped {
				return
			}
		}
	}
}

func (s *server) handleOneshotLatestRun(w http.ResponseWriter, r *http.Request) {
	run, found, err := s.store.GetLatestOneshotRun()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	results, err := s.store.GetAllOneshotResults()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, oneshotLatestRunResponse{
		RunID:      run.ID,
		Status:     string(run.Status),
		Model:      run.Model,
		Endpoint:   run.Endpoint,
		PromptIDs:  run.PromptIDs,
		StartedAt:  run.StartedAt,
		FinishedAt: run.FinishedAt,
		Error:      run.Error,
		Results:    results,
	})
}

func (s *server) handleOneshotArtifact(w http.ResponseWriter, r *http.Request) {
	promptID := strings.TrimPrefix(r.URL.Path, "/oneshot/artifacts/")
	if promptID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "promptId is required"})
		return
	}

	artifactPath := filepath.Join(s.appConfig.DataDir, "artifacts", "oneshot", promptID+".html")
	http.ServeFile(w, r, artifactPath)
}

func (s *server) handleReportData(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"models":     []any{},
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

type scenarioInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
	MaxPoints  int    `json:"maxPoints"`
	Prompt     string `json:"prompt"`
	Track      string `json:"track"`
}

type modelsResponse struct {
	Local  []modelInfo `json:"local"`
	Remote []modelInfo `json:"remote"`
}

type modelInfo struct {
	ID             string `json:"id"`
	Source         string `json:"source"`
	Endpoint       string `json:"endpoint"`
	RequiresAPIKey bool   `json:"requiresApiKey,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
}

type modelListKey struct {
	id     string
	source string
}

type cachedModels struct {
	models []modelInfo
	ts     time.Time
}

type modelsCache struct {
	mu    sync.Mutex
	cache *cachedModels
	ttl   int
}

func (c *modelsCache) get() ([]modelInfo, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		return nil, false
	}
	if time.Since(c.cache.ts) > time.Duration(c.ttl)*time.Second {
		return nil, false
	}
	return c.cache.models, true
}

func (c *modelsCache) set(models []modelInfo) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = &cachedModels{models: models, ts: time.Now()}
}

func (c *modelsCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = nil
}

func (s *server) discoverRemoteModels(ctx context.Context) []modelInfo {
	endpoint := strings.TrimRight(s.appConfig.RemoteEndpoint(), "/")
	if endpoint == "" {
		return s.staticRemoteModels()
	}

	if cached, ok := s.modelsCache.get(); ok {
		return cached
	}

	data, err := s.fetchOpenAIModels(ctx, endpoint, s.appConfig.RemoteAPIKey())
	if err != nil {
		slog.Debug("remote models endpoint unavailable", "endpoint", endpoint, "err", err)
		return s.staticRemoteModels()
	}

	remoteModels := s.appConfig.RemoteModels()
	seen := make(map[modelListKey]struct{}, len(data)+len(remoteModels))
	models := make([]modelInfo, 0, len(data)+len(remoteModels))

	for _, m := range data {
		if m.ID == "" {
			continue
		}
		key := modelListKey{id: m.ID, source: "remote"}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, modelInfo{
			ID:             m.ID,
			Source:         "remote",
			Endpoint:       endpoint,
			RequiresAPIKey: s.appConfig.RemoteAPIKey() == "",
			DisplayName:    displayNameForModel(m.ID, m.Object),
		})
	}

	for _, id := range remoteModels {
		key := modelListKey{id: id, source: "remote"}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, modelInfo{
			ID:             id,
			Source:         "remote",
			Endpoint:       endpoint,
			RequiresAPIKey: s.appConfig.RemoteAPIKey() == "",
			DisplayName:    displayNameForModel(id, ""),
		})
	}

	s.modelsCache.set(models)
	return models
}

func (s *server) fetchOpenAIModels(ctx context.Context, endpoint, apiKey string) ([]openAIModel, error) {
	endpoint = strings.TrimRight(endpoint, "/")
	url := endpoint + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var parsed openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Data, nil
}

func (s *server) discoverLocalModels(ctx context.Context) []modelInfo {
	endpoint := strings.TrimRight(s.appConfig.LocalEndpoint(), "/")
	if endpoint == "" {
		return []modelInfo{}
	}

	data, err := s.fetchOpenAIModels(ctx, endpoint, "")
	if err != nil {
		slog.Debug("local models endpoint unavailable", "endpoint", endpoint, "err", err)
		return []modelInfo{}
	}

	models := make([]modelInfo, 0, len(data))
	for _, m := range data {
		if m.ID == "" {
			continue
		}
		models = append(models, modelInfo{
			ID:          m.ID,
			Source:      "local",
			Endpoint:    endpoint,
			DisplayName: displayNameForModel(m.ID, m.Object),
		})
	}
	return models
}

func (s *server) staticRemoteModels() []modelInfo {
	remoteModels := s.appConfig.RemoteModels()
	models := make([]modelInfo, 0, len(remoteModels))
	endpoint := strings.TrimRight(s.appConfig.RemoteEndpoint(), "/")
	for _, id := range remoteModels {
		models = append(models, modelInfo{
			ID:             id,
			Source:         "remote",
			Endpoint:       endpoint,
			RequiresAPIKey: s.appConfig.RemoteAPIKey() == "",
			DisplayName:    displayNameForModel(id, ""),
		})
	}
	return models
}

func displayNameForModel(id, object string) string {
	// Some providers include a human-readable name in the object field.
	// Prefer it only when it is a real name and not the OpenAI generic "model" marker.
	if object != "" && object != id && object != "model" {
		return object
	}
	return deriveDisplayNameFromID(id)
}

func deriveDisplayNameFromID(id string) string {
	// Replace common separators with spaces and title-case each word.
	separators := []string{"-", "_", "."}
	name := id
	for _, sep := range separators {
		name = strings.ReplaceAll(name, sep, " ")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	words := strings.Fields(name)
	for i, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		if len(runes) > 0 {
			runes[0] = unicode.ToTitle(runes[0])
			for j := 1; j < len(runes); j++ {
				runes[j] = unicode.ToLower(runes[j])
			}
			words[i] = string(runes)
		}
	}
	derived := strings.Join(words, " ")
	if derived == id {
		return ""
	}
	return derived
}

type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type runtimeConfigResponse struct {
	LocalEndpoint              string   `json:"localEndpoint"`
	RemoteEndpoint             string   `json:"remoteEndpoint"`
	RemoteAPIKey               string   `json:"remoteApiKey"`
	RemoteModels               []string `json:"remoteModels"`
	RemoteModelCacheTTLSeconds int      `json:"remoteModelCacheTTLSeconds"`
}

type oneshotLatestRunResponse struct {
	RunID      string                `json:"runId"`
	Status     string                `json:"status"`
	Model      string                `json:"model,omitempty"`
	Endpoint   string                `json:"endpoint,omitempty"`
	PromptIDs  []string              `json:"promptIds"`
	StartedAt  int64                 `json:"startedAt"`
	FinishedAt *int64                `json:"finishedAt,omitempty"`
	Error      string                `json:"error,omitempty"`
	Results    []model.OneshotResult `json:"results"`
}

type updateRuntimeConfigRequest struct {
	LocalEndpoint              string   `json:"localEndpoint,omitempty"`
	RemoteEndpoint             string   `json:"remoteEndpoint,omitempty"`
	RemoteAPIKey               string   `json:"remoteApiKey,omitempty"`
	RemoteModels               []string `json:"remoteModels,omitempty"`
	RemoteModelCacheTTLSeconds int      `json:"remoteModelCacheTTLSeconds,omitempty"`
}

type runSummary struct {
	ID          string `json:"id"`
	StartedAt   int64  `json:"startedAt"`
	FinishedAt  *int64 `json:"finishedAt,omitempty"`
	Status      string `json:"status"`
	Model       string `json:"model"`
	TotalPoints int    `json:"totalPoints"`
	MaxPoints   int    `json:"maxPoints"`
}

type runDetail struct {
	ID          string           `json:"id"`
	StartedAt   int64            `json:"startedAt"`
	FinishedAt  *int64           `json:"finishedAt,omitempty"`
	Status      string           `json:"status"`
	Model       string           `json:"model"`
	TotalPoints int              `json:"totalPoints"`
	MaxPoints   int              `json:"maxPoints"`
	Scenarios   []scenarioResult `json:"scenarios"`
}

type scenarioResult struct {
	ScenarioID    string `json:"scenarioId"`
	Category      string `json:"category,omitempty"`
	Family        string `json:"family,omitempty"`
	Status        string `json:"status"`
	Points        int    `json:"points"`
	MaxPoints     int    `json:"maxPoints"`
	WallTimeMs    *int64 `json:"wallTimeMs,omitempty"`
	FirstTokenMs  *int64 `json:"firstTokenMs,omitempty"`
	ToolCallCount int    `json:"toolCallCount"`
	ModelMetrics  any    `json:"modelMetrics,omitempty"`
	Evaluation    any    `json:"evaluation,omitempty"`
	RubricKind    string `json:"rubricKind,omitempty"`
	Breakdown     any    `json:"breakdown,omitempty"`
	Error         string `json:"error,omitempty"`
}

func zeroIfNil(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func jsonRaw(s string) any {
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

func breakdown(sr *model.ScenarioRun) any {
	return map[string]any{
		"correctness":  zeroIfNil(sr.Correctness),
		"scope":        zeroIfNil(sr.Scope),
		"pattern":      zeroIfNil(sr.Pattern),
		"verification": zeroIfNil(sr.Verification),
		"cleanup":      zeroIfNil(sr.Cleanup),
	}
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("api request started",
			"method", r.Method,
			"path", r.URL.Path,
		)
		start := time.Now()
		wrapped := &responseWriterWithStatus{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)
		slog.Info("api request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", duration,
		)
	})
}

type responseWriterWithStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWithStatus) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWithStatus) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
