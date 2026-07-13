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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
	"github.com/timmersuk/scaffold-bench-go/internal/web"
)

// Config holds the dependencies needed by the HTTP router.
type Config struct {
	Store     *storage.Store
	Events    *realtime.Hub
	Runner    Runner
	Registry  *runner.Registry
	AppConfig config.Config
	BuildID   string
	Frontend  fs.FS
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
	if cfg.Registry == nil {
		return nil, errors.New("registry is required")
	}

	frontend, err := fs.Sub(web.Files, "dist")
	if err != nil {
		return nil, fmt.Errorf("sub frontend fs: %w", err)
	}

	srv := &server{
		store:       cfg.Store,
		events:      cfg.Events,
		runner:      cfg.Runner,
		registry:    cfg.Registry,
		appConfig:   cfg.AppConfig,
		buildID:     cfg.BuildID,
		frontend:    frontend,
		modelsCache: &modelsCache{
			ttl: cfg.AppConfig.RemoteModelCacheTTLSeconds,
		},
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/scenarios", srv.withMethod(http.MethodGet, srv.handleScenarios))
	apiMux.HandleFunc("/models", srv.withMethod(http.MethodGet, srv.handleModels))
	apiMux.HandleFunc("/runs/active", srv.withMethod(http.MethodGet, srv.handleActiveRun))
	apiMux.HandleFunc("/runs/clear", srv.withMethod(http.MethodPost, srv.handleClearRuns))
	apiMux.HandleFunc("/runs", srv.handleRuns)
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
	store       *storage.Store
	events      *realtime.Hub
	runner      Runner
	registry    *runner.Registry
	appConfig   config.Config
	buildID     string
	frontend    fs.FS
	modelsCache *modelsCache
	httpClient  *http.Client
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
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (s *server) handleOneshotRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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

func (s *server) discoverRemoteModels(ctx context.Context) []modelInfo {
	endpoint := strings.TrimRight(s.appConfig.RemoteEndpoint, "/")
	if endpoint == "" {
		return s.staticRemoteModels()
	}

	if cached, ok := s.modelsCache.get(); ok {
		return cached
	}

	url := endpoint + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("build remote models request", "err", err)
		return s.staticRemoteModels()
	}
	req.Header.Set("Accept", "application/json")
	if s.appConfig.RemoteAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.appConfig.RemoteAPIKey)
	}

	client := s.httpClient
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("remote models endpoint unreachable", "endpoint", endpoint, "err", err)
		return s.staticRemoteModels()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("remote models endpoint returned non-ok status", "status", resp.StatusCode)
		return s.staticRemoteModels()
	}

	var parsed openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		slog.Error("decode remote models response", "err", err)
		return s.staticRemoteModels()
	}

	seen := make(map[modelListKey]struct{}, len(parsed.Data)+len(s.appConfig.RemoteModels))
	models := make([]modelInfo, 0, len(parsed.Data)+len(s.appConfig.RemoteModels))

	for _, m := range parsed.Data {
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
			RequiresAPIKey: s.appConfig.RemoteAPIKey == "",
			DisplayName:    displayNameForModel(m.ID, m.Object),
		})
	}

	for _, id := range s.appConfig.RemoteModels {
		key := modelListKey{id: id, source: "remote"}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, modelInfo{
			ID:             id,
			Source:         "remote",
			Endpoint:       endpoint,
			RequiresAPIKey: s.appConfig.RemoteAPIKey == "",
			DisplayName:    displayNameForModel(id, ""),
		})
	}

	s.modelsCache.set(models)
	return models
}

func (s *server) staticRemoteModels() []modelInfo {
	models := make([]modelInfo, 0, len(s.appConfig.RemoteModels))
	endpoint := strings.TrimRight(s.appConfig.RemoteEndpoint, "/")
	for _, id := range s.appConfig.RemoteModels {
		models = append(models, modelInfo{
			ID:             id,
			Source:         "remote",
			Endpoint:       endpoint,
			RequiresAPIKey: s.appConfig.RemoteAPIKey == "",
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

func (s *server) discoverLocalModels(ctx context.Context) []modelInfo {
	endpoint := strings.TrimRight(s.appConfig.LocalEndpoint, "/")
	if endpoint == "" {
		return []modelInfo{}
	}

	url := endpoint + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("build local models request", "err", err)
		return []modelInfo{}
	}
	req.Header.Set("Accept", "application/json")

	client := s.httpClient
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("local models endpoint unreachable", "endpoint", endpoint, "err", err)
		return []modelInfo{}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("local models endpoint returned non-ok status", "status", resp.StatusCode)
		return []modelInfo{}
	}

	var parsed openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		slog.Error("decode local models response", "err", err)
		return []modelInfo{}
	}

	models := make([]modelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
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

type errorResponse struct {
	Error string `json:"error"`
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
