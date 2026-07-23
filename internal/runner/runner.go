package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/timmersuk/scaffold-bench-go/internal/agent"
	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

// defaultBuildTimeout is the maximum duration allowed for a single build command.
const defaultBuildTimeout = 60 * time.Second

// StartRequest is the payload for POST /api/runs.
type StartRequest struct {
	ScenarioIDs   []string                `json:"scenarioIds"`
	ModelID       string                  `json:"modelId,omitempty"`
	Source        string                  `json:"source,omitempty"`
	Endpoint      string                  `json:"endpoint,omitempty"`
	APIKey        string                  `json:"apiKey,omitempty"`
	SystemPrompt  string                  `json:"systemPrompt,omitempty"`
	Harness       string                  `json:"harness,omitempty"`
	TimeoutMs     int                     `json:"timeoutMs,omitempty"`
	ToolExecution agent.ToolExecutionMode `json:"toolExecution,omitempty"`
}

// Engine orchestrates benchmark runs.
type Engine struct {
	store    *storage.Store
	hub      *realtime.Hub
	cfg      config.Config
	registry *Registry
	mu       sync.Mutex
	active   map[string]*activeRun
}

type activeRun struct {
	cancel context.CancelFunc
	seq    atomic.Int64
}

// NewEngine creates a run engine.
func NewEngine(store *storage.Store, hub *realtime.Hub, cfg config.Config, registry *Registry) *Engine {
	return &Engine{
		store:    store,
		hub:      hub,
		cfg:      cfg,
		registry: registry,
		active:   make(map[string]*activeRun),
	}
}

// Start begins a new run and returns its ID.
func (e *Engine) Start(req StartRequest) (string, error) {
	if len(req.ScenarioIDs) == 0 {
		return "", fmt.Errorf("no scenarios specified")
	}
	for _, id := range req.ScenarioIDs {
		if _, ok := e.registry.Get(id); !ok {
			return "", fmt.Errorf("unknown scenario %q", id)
		}
	}

	runID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{cancel: cancel}
	e.mu.Lock()
	e.active[runID] = ar
	e.mu.Unlock()

	now := time.Now().UnixMilli()
	run := model.Run{
		ID:          runID,
		StartedAt:   now,
		Status:      model.RunWarmingUp,
		ScenarioIDs: req.ScenarioIDs,
		Runtime:     "local",
		RuntimeKind: "llama.cpp",
		Endpoint:    req.Endpoint,
		Model:       req.ModelID,
		Source:      req.Source,
		Harness:     req.Harness,
	}
	if run.Endpoint == "" {
		run.Endpoint = e.cfg.LocalEndpoint()
	}
	if run.Source == "" {
		run.Source = "local"
	}

	if err := e.store.InsertRun(run); err != nil {
		cancel()
		e.mu.Lock()
		delete(e.active, runID)
		e.mu.Unlock()
		return "", fmt.Errorf("insert run: %w", err)
	}

	startPayload := map[string]any{
		"scenarioIds": req.ScenarioIDs,
		"model":       req.ModelID,
		"endpoint":    run.Endpoint,
		"harness":     req.Harness,
	}
	e.publish(runID, "", ar.nextSeq(), now, model.EventRunStarted, startPayload)

	for _, id := range req.ScenarioIDs {
		scenario, _ := e.registry.Get(id)
		e.store.UpsertScenarioRun(model.ScenarioRun{
			RunID:      runID,
			ScenarioID: id,
			Family:     scenario.Family,
			RubricKind: scenario.RubricKind,
			Status:     model.ScenarioPending,
			MaxPoints:  scenario.MaxPoints,
		})
	}

	go e.executeRun(ctx, runID, req, ar)
	return runID, nil
}

// Stop cancels an active run.
func (e *Engine) Stop(runID string) error {
	e.mu.Lock()
	ar, ok := e.active[runID]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("run %s not active", runID)
	}
	ar.cancel()
	return nil
}

// ActiveRunID returns an active run ID, if any.
func (e *Engine) ActiveRunID() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id := range e.active {
		return id, true
	}
	return "", false
}

func (e *Engine) executeRun(ctx context.Context, runID string, req StartRequest, ar *activeRun) {
	var scenarioResults []scenarioResult
	defer func() {
		e.mu.Lock()
		delete(e.active, runID)
		e.mu.Unlock()
	}()

	var runErr error
	defer func() {
		if r := recover(); r != nil {
			runErr = fmt.Errorf("panic: %v", r)
		}
		isAbort := ctx.Err() != nil
		ts := time.Now().UnixMilli()
		if runErr != nil {
			status := model.RunFailed
			if isAbort {
				status = model.RunStopped
			}
			eventType := model.EventRunFailed
			payload := map[string]any{"error": runErr.Error()}
			if isAbort {
				eventType = model.EventRunStopped
				payload = map[string]any{"reason": "user requested stop"}
			}
			e.publish(runID, "", ar.nextSeq(), ts, eventType, payload)
			e.store.UpdateRun(model.Run{ID: runID, Status: status, FinishedAt: &ts, Error: runErr.Error()})
			return
		}

		total, max := totalPoints(scenarioResults)
		reportPath, err := e.writeReport(runID, req, total, max, scenarioResults)
		if err != nil {
			slog.Error("write run report", "run_id", runID, "err", err)
		}
		e.publish(runID, "", ar.nextSeq(), ts, model.EventRunFinished, map[string]any{
			"totalPoints": total,
			"maxPoints":   max,
			"reportPath":  reportPath,
		})
		e.store.UpdateRun(model.Run{ID: runID, Status: model.RunDone, FinishedAt: &ts, TotalPoints: &total, MaxPoints: &max, ReportPath: reportPath})
	}()

	endpoint := req.Endpoint
	if endpoint == "" {
		endpoint = e.cfg.LocalEndpoint()
	}
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	// Perform warmup phase
	warmupStart := time.Now().UnixMilli()
	e.publish(runID, "", ar.nextSeq(), warmupStart, model.EventModelWarmupStarted, map[string]any{
		"model":    req.ModelID,
		"endpoint": endpoint,
	})

	warmupResult, err := PerformWarmup(ctx, endpoint, req.ModelID, req.APIKey, 5*time.Minute)
	if err != nil {
		runErr = fmt.Errorf("warmup failed: %w", err)
		return
	}

	warmupEnd := time.Now().UnixMilli()
	e.publish(runID, "", ar.nextSeq(), warmupEnd, model.EventModelWarmupFinished, map[string]any{
		"durationMs": warmupEnd - warmupStart,
		"modelFile":  warmupResult.ModelFile,
		"quant":      warmupResult.Quant,
		"gpuBackend": warmupResult.GPUBackend,
		"gpuModel":   warmupResult.GPUModel,
	})

	// Update run with metadata
	metadataUpdate := model.Run{
		ID:          runID,
		Status:      model.RunRunning,
		ModelFile:   warmupResult.ModelFile,
		Quant:       warmupResult.Quant,
		QuantTier:   warmupResult.QuantTier,
		QuantSource: warmupResult.QuantSource,
		ContextSize: warmupResult.ContextSize,
		GPUBackend:  warmupResult.GPUBackend,
		GPUModel:    warmupResult.GPUModel,
		GPUCount:    warmupResult.GPUCount,
		VRAMTotalMB: warmupResult.VRAMTotalMB,
	}
	if err := e.store.UpdateRunMetadata(metadataUpdate); err != nil {
		slog.Error("update run metadata", "run_id", runID, "err", err)
	}

	for _, scenarioID := range req.ScenarioIDs {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			return
		}
		scenario, ok := e.registry.Get(scenarioID)
		if !ok {
			continue
		}
		scenarioResults = append(scenarioResults, e.runScenario(ctx, runID, req, scenario, endpoint, timeout, ar))
	}
}

type scenarioResult struct {
	ScenarioID   string
	Category     string
	Family       string
	Status       model.ScenarioStatus
	Points       int
	MaxPoints    int
	WallTimeMs   int64
	FirstTokenMs *int64
	ToolCalls    int
	Evaluation   model.Evaluation
	ModelMetrics model.ModelMetrics
	Archive      *model.WorkspaceArchive
	ArtifactPath string
	Error        string
}

// missingRequirement returns the first manifest requirement that is not on PATH.
func missingRequirement(requires []string) string {
	for _, name := range requires {
		if name == "" {
			continue
		}
		if _, err := exec.LookPath(name); err != nil {
			return name
		}
	}
	return ""
}

func (e *Engine) runScenario(ctx context.Context, runID string, req StartRequest, scenario Scenario, endpoint string, timeout time.Duration, ar *activeRun) scenarioResult {
	res := scenarioResult{ScenarioID: scenario.ID, Category: scenario.Category, Family: scenario.Family, MaxPoints: scenario.MaxPoints}
	now := time.Now().UnixMilli()

	e.publish(runID, scenario.ID, ar.nextSeq(), now, model.EventScenarioStarted, map[string]any{
		"scenarioId": scenario.ID,
		"name":       scenario.Name,
		"category":   scenario.Category,
		"maxPoints":  scenario.MaxPoints,
		"family":     scenario.Family,
		"rubricKind": scenario.RubricKind,
		"prompt":     scenario.Prompt,
	})
	startedAt := now
	e.store.UpsertScenarioRun(model.ScenarioRun{
		RunID:      runID,
		ScenarioID: scenario.ID,
		Category:   scenario.Category,
		Family:     scenario.Family,
		StartedAt:  &startedAt,
		Status:     model.ScenarioRunning,
		MaxPoints:  scenario.MaxPoints,
		RubricKind: scenario.RubricKind,
	})

	if missing := missingRequirement(scenario.Manifest.Requires); missing != "" {
		msg := fmt.Sprintf("missing required tool: %s", missing)
		res.Status = model.ScenarioSkipped
		res.MaxPoints = 0
		res.Error = msg
		res.Evaluation = model.Evaluation{
			Status:    "skipped",
			Points:    0,
			MaxPoints: 0,
			Summary:   msg,
			Checks: []model.CheckResult{
				{Name: "requires", Pass: false, Weight: 0, Detail: msg},
			},
		}
		e.finishScenario(runID, scenario, ar, res)
		return res
	}

	workDir, err := os.MkdirTemp("", "sb-run-")
	if err != nil {
		res.Status = model.ScenarioFail
		res.Error = fmt.Sprintf("create workspace: %s", err)
		res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
		e.finishScenario(runID, scenario, ar, res)
		return res
	}
	defer os.RemoveAll(workDir)

	workspaceRoot := scenario.Manifest.Workspace.Root
	if workspaceRoot == "" {
		workspaceRoot = "playground"
	}

	if scenario.WorkspaceSource != "" {
		dest := filepath.Join(workDir, workspaceRoot)
		if err := copyDir(scenario.WorkspaceSource, dest); err != nil {
			res.Status = model.ScenarioFail
			res.Error = fmt.Sprintf("copy workspace: %s", err)
			res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
			e.finishScenario(runID, scenario, ar, res)
			return res
		}
	}
	// Ensure the workspace root directory exists.
	_ = os.MkdirAll(filepath.Join(workDir, workspaceRoot), 0o755)

	// Copy any setup files defined by the manifest.
	if scenario.Manifest.Setup != nil {
		for _, mapping := range scenario.Manifest.Setup.Files {
			src := filepath.Join(scenario.Dir, mapping.Src)
			dst := filepath.Join(workDir, mapping.Dest)
			if err := copyFile(src, dst); err != nil {
				res.Status = model.ScenarioFail
				res.Error = fmt.Sprintf("copy setup file: %s", err)
				res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
				e.finishScenario(runID, scenario, ar, res)
				return res
			}
		}
	}

	// Stage hidden fixtures in a directory adjacent to the workspace root.
	hiddenDir := ""
	if len(scenario.Manifest.HiddenFixtures) > 0 {
		hiddenDir = filepath.Join(workDir, "hidden")
		if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
			res.Status = model.ScenarioFail
			res.Error = fmt.Sprintf("create hidden dir: %s", err)
			res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
			e.finishScenario(runID, scenario, ar, res)
			return res
		}
		for _, hf := range scenario.Manifest.HiddenFixtures {
			src := filepath.Join(scenario.Dir, hf.Src)
			dst := filepath.Join(hiddenDir, hf.Dest)
			if err := copyFile(src, dst); err != nil {
				res.Status = model.ScenarioFail
				res.Error = fmt.Sprintf("copy hidden fixture %s: %s", hf.Src, err)
				res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
				e.finishScenario(runID, scenario, ar, res)
				return res
			}
		}
	}

	agentCfg := agent.Config{
		WorkDir:       workDir,
		Prompt:        scenario.Prompt,
		Endpoint:      endpoint,
		Model:         req.ModelID,
		APIKey:        req.APIKey,
		SystemPrompt:  req.SystemPrompt,
		Harness:       req.Harness,
		Timeout:       timeout,
		ToolExecution: req.ToolExecution,
		OnEvent: func(ev model.RuntimeEvent) {
			e.publish(runID, scenario.ID, ar.nextSeq(), time.Now().UnixMilli(), ev.Type, runtimeEventPayload(ev))
		},
	}
	output := agent.Run(ctx, agentCfg)

	// Run manifest build commands after the agent finishes and before checks.
	if err := runBuildCommands(ctx, workDir, scenario.Manifest.Build, defaultBuildTimeout); err != nil {
		res.Status = model.ScenarioFail
		res.Error = err.Error()
		res.Evaluation = errorEvaluation(res.Error, scenario.MaxPoints)
		e.finishScenario(runID, scenario, ar, res)
		return res
	}

	pristineDir := scenario.PristineDir
	if pristineDir == "" {
		empty, _ := os.MkdirTemp("", "sb-pristine-")
		defer os.RemoveAll(empty)
		pristineDir = empty
	}

	evaluator := NewEvaluator()
	eval := evaluator.Evaluate(ctx, Input{
		Manifest:    scenario.Manifest,
		WorkDir:     workDir,
		PristineDir: pristineDir,
		Dir:         scenario.Dir,
		HiddenDir:   hiddenDir,
		ToolCalls:   output.ToolCalls,
	})
	res.Evaluation = eval
	res.Points = eval.Points
	res.Status = model.ScenarioStatus(eval.Status)
	if res.Status == "" {
		res.Status = model.ScenarioFail
	}
	res.WallTimeMs = output.WallTimeMs
	res.FirstTokenMs = output.FirstTokenMs
	res.ToolCalls = len(output.ToolCalls)
	res.ModelMetrics = output.ModelMetrics
	if output.Error != "" {
		res.Error = output.Error
		if res.Status == model.ScenarioPass || res.Status == model.ScenarioPartial {
			res.Status = model.ScenarioFail
			res.Points = 0
		}
	}

	archive, err := captureWorkspace(filepath.Join(workDir, workspaceRoot), pristineDir, workspaceRoot)
	if err != nil {
		slog.Error("capture workspace", "run_id", runID, "scenario", scenario.ID, "err", err)
	} else {
		res.Archive = archive
		artifactPath := archiveArtifactPath(e.cfg.DataDir, runID, scenario.ID)
		if err := writeArtifact(artifactPath, archive); err != nil {
			slog.Error("write artifact", "run_id", runID, "scenario", scenario.ID, "err", err)
		} else {
			rel, _ := filepath.Rel(e.cfg.DataDir, artifactPath)
			res.ArtifactPath = rel
		}
	}

	e.finishScenario(runID, scenario, ar, res)
	return res
}

func (e *Engine) finishScenario(runID string, scenario Scenario, ar *activeRun, res scenarioResult) {
	ts := time.Now().UnixMilli()
	evaluationJSON, _ := json.Marshal(res.Evaluation)
	modelMetricsJSON, _ := json.Marshal(res.ModelMetrics)
	mutated := false
	if res.Archive != nil {
		mutated = len(res.Archive.Changed) > 0 || len(res.Archive.Deleted) > 0
	}
	e.store.UpsertScenarioRun(model.ScenarioRun{
		RunID:            runID,
		ScenarioID:       scenario.ID,
		Category:         scenario.Category,
		Family:           scenario.Family,
		FinishedAt:       &ts,
		Status:           res.Status,
		Points:           &res.Points,
		MaxPoints:        res.MaxPoints,
		RubricKind:       scenario.RubricKind,
		WallTimeMs:       &res.WallTimeMs,
		FirstTokenMs:     res.FirstTokenMs,
		ToolCallCount:    &res.ToolCalls,
		ModelMetricsJSON: string(modelMetricsJSON),
		EvaluationJSON:   string(evaluationJSON),
		Error:            res.Error,
		ArtifactPath:     res.ArtifactPath,
		Mutated:          &mutated,
	})

	e.publish(runID, scenario.ID, ar.nextSeq(), ts, model.EventScenarioFinished, map[string]any{
		"scenarioId":    scenario.ID,
		"status":        string(res.Status),
		"points":        res.Points,
		"wallTimeMs":    res.WallTimeMs,
		"toolCallCount": res.ToolCalls,
		"firstTokenMs":  res.FirstTokenMs,
		"evaluation":    res.Evaluation,
		"modelMetrics":  res.ModelMetrics,
		"artifactPath":  res.ArtifactPath,
	})
}

func (e *Engine) publish(runID, scenarioID string, seq, ts int64, typ string, payload any) {
	e.hub.Publish(model.Event{
		Seq:        seq,
		Ts:         ts,
		Type:       typ,
		Payload:    payload,
		RunID:      runID,
		ScenarioID: scenarioID,
	})
	if err := e.store.InsertEvent(runID, scenarioID, seq, ts, typ, payload); err != nil {
		slog.Error("insert event", "run_id", runID, "type", typ, "err", err)
	}
}

func (ar *activeRun) nextSeq() int64 {
	return ar.seq.Add(1) - 1
}

func runtimeEventPayload(ev model.RuntimeEvent) any {
	switch ev.Type {
	case model.EventAssistantDelta:
		return map[string]any{"content": ev.Delta}
	case model.EventReasoningDelta:
		return map[string]any{"content": ev.Delta}
	case model.EventAssistant:
		return map[string]any{"content": ev.Content}
	case model.EventToolCall:
		return map[string]any{"call": ev.Call}
	case model.EventToolResult:
		return map[string]any{"call": ev.Call, "result": ev.Result}
	case model.EventModelMetrics:
		return map[string]any{"metrics": ev.Metrics}
	}
	return map[string]any{}
}

func errorEvaluation(msg string, maxPoints int) model.Evaluation {
	return model.Evaluation{
		Status:    "fail",
		Points:    0,
		MaxPoints: maxPoints,
		Summary:   msg,
		Checks: []model.CheckResult{
			{Name: "execute", Pass: false, Weight: maxPoints, Detail: msg},
		},
	}
}

func totalPoints(results []scenarioResult) (int, int) {
	total := 0
	max := 0
	for _, r := range results {
		total += r.Points
		max += r.MaxPoints
	}
	return total, max
}

func copyDir(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	return os.CopyFS(dst, os.DirFS(src))
}

type runReport struct {
	Timestamp    string             `json:"timestamp"`
	Runtime      string             `json:"runtime"`
	TotalPoints  int                `json:"totalPoints"`
	MaxPoints    int                `json:"maxPoints"`
	Harness      string             `json:"harness,omitempty"`
	ModelMetrics model.ModelMetrics `json:"modelMetrics"`
	Results      []scenarioReport   `json:"results"`
}

type scenarioReport struct {
	ScenarioID    string              `json:"scenarioId"`
	Category      string              `json:"category"`
	Family        string              `json:"family,omitempty"`
	Status        string              `json:"status"`
	Points        int                 `json:"points"`
	MaxPoints     int                 `json:"maxPoints"`
	Summary       string              `json:"summary"`
	RubricKind    string              `json:"rubricKind,omitempty"`
	ToolCallCount int                 `json:"toolCallCount"`
	WallTimeMs    int64               `json:"wallTimeMs"`
	FirstTokenMs  *int64              `json:"firstTokenMs,omitempty"`
	Error         string              `json:"error,omitempty"`
	ModelMetrics  model.ModelMetrics  `json:"modelMetrics"`
	Checks        []model.CheckResult `json:"checks"`
}

func (e *Engine) writeReport(runID string, req StartRequest, total, max int, results []scenarioResult) (string, error) {
	dir := filepath.Join(e.cfg.DataDir, "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	timestamp := time.Now().UnixMilli()
	path := filepath.Join(dir, fmt.Sprintf("%d-local.json", timestamp))

	merged := model.ModelMetrics{Model: req.ModelID}
	for _, r := range results {
		merged.RequestCount += r.ModelMetrics.RequestCount
		merged.PromptTokens += r.ModelMetrics.PromptTokens
		merged.CompletionTokens += r.ModelMetrics.CompletionTokens
		merged.TotalTokens += r.ModelMetrics.TotalTokens
		merged.TotalRequestTimeMs += r.ModelMetrics.TotalRequestTimeMs
		merged.Requests = append(merged.Requests, r.ModelMetrics.Requests...)
	}

	report := runReport{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Runtime:      "local",
		TotalPoints:  total,
		MaxPoints:    max,
		Harness:      req.Harness,
		ModelMetrics: merged,
	}
	for _, r := range results {
		report.Results = append(report.Results, scenarioReport{
			ScenarioID:    r.ScenarioID,
			Category:      r.Category,
			Family:        r.Family,
			Status:        string(r.Status),
			Points:        r.Points,
			MaxPoints:     r.MaxPoints,
			Summary:       r.Evaluation.Summary,
			RubricKind:    r.Evaluation.RubricKind,
			ToolCallCount: r.ToolCalls,
			WallTimeMs:    r.WallTimeMs,
			FirstTokenMs:  r.FirstTokenMs,
			Error:         r.Error,
			ModelMetrics:  r.ModelMetrics,
			Checks:        r.Evaluation.Checks,
		})
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(e.cfg.DataDir, path)
	return rel, nil
}

// runBuildCommands executes the manifest build commands in order. Each command
// runs with the current process environment plus any variables declared in the
// manifest. The {{.WorkDir}} template variable is expanded to an absolute path.
func runBuildCommands(ctx context.Context, workDir string, build *Build, timeout time.Duration) error {
	if build == nil || len(build.Commands) == 0 {
		return nil
	}

	env := os.Environ()
	for k, v := range build.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	for _, command := range build.Commands {
		expanded, err := expandBuildCommand(command, workDir)
		if err != nil {
			return err
		}

		ok, output, err := executeCommand(ctx, workDir, expanded, env, timeout)
		if err != nil || !ok {
			if err == nil {
				err = fmt.Errorf("command exited non-zero")
			}
			return fmt.Errorf("build command %q failed: %w\n%s", expanded, err, output)
		}
	}
	return nil
}

func expandBuildCommand(command, workDir string) (string, error) {
	tmpl, err := template.New("build").Parse(command)
	if err != nil {
		return "", fmt.Errorf("parse build command template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ WorkDir string }{WorkDir: workDir}); err != nil {
		return "", fmt.Errorf("execute build command template: %w", err)
	}
	return buf.String(), nil
}

func executeCommand(ctx context.Context, cwd, command string, env []string, timeout time.Duration) (bool, string, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	shell, arg := getShell()
	cmd := exec.CommandContext(ctx, shell, arg, command)
	cmd.Dir = cwd
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, out, fmt.Errorf("timed out after %s", timeout)
		}
		return false, out, err
	}
	return true, out, nil
}

func getShell() (string, string) {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return shell, "/c"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, "-lc"
}
