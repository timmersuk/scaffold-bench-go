package oneshot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/timmersuk/scaffold-bench-go/internal/agent"
	"github.com/timmersuk/scaffold-bench-go/internal/config"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/realtime"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

const oneshotTimeout = 10 * time.Minute

type StartRequest struct {
	ModelID  string   `json:"modelId"`
	PromptIDs []string `json:"promptIds"`
}

type Engine struct {
	store  *storage.Store
	hub    *realtime.Hub
	cfg    config.Config
	caller agent.Caller
	mu     sync.Mutex
	active map[string]*activeRun
}

type activeRun struct {
	cancel context.CancelFunc
	seq    atomic.Int64
}

func NewEngine(store *storage.Store, hub *realtime.Hub, cfg config.Config, caller agent.Caller) *Engine {
	return &Engine{
		store:  store,
		hub:    hub,
		cfg:    cfg,
		caller: caller,
		active: make(map[string]*activeRun),
	}
}

func (e *Engine) Start(req StartRequest) (string, error) {
	if len(req.PromptIDs) == 0 {
		return "", fmt.Errorf("no prompts specified")
	}

	prompts, err := LoadLabPrompts("lab_prompts")
	if err != nil {
		return "", fmt.Errorf("load prompts: %w", err)
	}
	promptMap := make(map[string]model.LabPrompt, len(prompts))
	for _, p := range prompts {
		promptMap[p.ID] = p
	}
	for _, id := range req.PromptIDs {
		if _, ok := promptMap[id]; !ok {
			return "", fmt.Errorf("unknown prompt %q", id)
		}
	}

	runID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{cancel: cancel}
	e.mu.Lock()
	if len(e.active) > 0 {
		e.mu.Unlock()
		cancel()
		return "", fmt.Errorf("a one-shot run is already in progress")
	}
	e.active[runID] = ar
	e.mu.Unlock()

	now := time.Now().UnixMilli()
	run := model.OneshotRun{
		ID:        runID,
		StartedAt: now,
		Status:    model.OneshotRunRunning,
		Model:     req.ModelID,
		Endpoint:  e.cfg.LocalEndpoint(),
		PromptIDs: req.PromptIDs,
	}
	if err := e.store.InsertOneshotRun(run); err != nil {
		cancel()
		e.mu.Lock()
		delete(e.active, runID)
		e.mu.Unlock()
		return "", fmt.Errorf("insert oneshot run: %w", err)
	}

	e.publish(runID, ar.nextSeq(), now, model.EventOneshotRunStarted, map[string]any{
		"runId":     runID,
		"promptIds": req.PromptIDs,
		"model":     req.ModelID,
	})

	if err := e.store.ResetOneshotPrompts(req.PromptIDs); err != nil {
		slog.Error("reset oneshot prompts", "run_id", runID, "err", err)
	}

	go e.executeRun(ctx, runID, req, promptMap, ar)
	return runID, nil
}

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

func (e *Engine) ActiveRunID() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id := range e.active {
		return id, true
	}
	return "", false
}

func (e *Engine) executeRun(ctx context.Context, runID string, req StartRequest, promptMap map[string]model.LabPrompt, ar *activeRun) {
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
			status := model.OneshotRunFailed
			eventType := model.EventOneshotRunFailed
			payload := map[string]any{"error": runErr.Error()}
			if isAbort {
				status = model.OneshotRunStopped
				eventType = model.EventOneshotRunStopped
				payload = map[string]any{"reason": "user requested stop"}
			}
			e.publish(runID, ar.nextSeq(), ts, eventType, payload)
			e.store.UpdateOneshotRun(model.OneshotRun{ID: runID, Status: status, FinishedAt: &ts, Error: runErr.Error()})
			return
		}
		e.publish(runID, ar.nextSeq(), ts, model.EventOneshotRunFinished, map[string]any{"runId": runID})
		e.store.UpdateOneshotRun(model.OneshotRun{ID: runID, Status: model.OneshotRunDone, FinishedAt: &ts})
	}()

	endpoint := e.cfg.LocalEndpoint()

	// Perform warmup phase
	warmupStart := time.Now().UnixMilli()
	e.publish(runID, ar.nextSeq(), warmupStart, model.EventOneshotWarmupStarted, map[string]any{
		"model":    req.ModelID,
		"endpoint": endpoint,
	})

	warmupResult, err := runner.PerformWarmup(ctx, endpoint, req.ModelID, "", 5*time.Minute)
	if err != nil {
		runErr = fmt.Errorf("warmup failed: %w", err)
		return
	}

	warmupEnd := time.Now().UnixMilli()
	e.publish(runID, ar.nextSeq(), warmupEnd, model.EventOneshotWarmupFinished, map[string]any{
		"durationMs": warmupEnd - warmupStart,
		"modelFile":  warmupResult.ModelFile,
		"quant":      warmupResult.Quant,
		"gpuBackend": warmupResult.GPUBackend,
		"gpuModel":   warmupResult.GPUModel,
	})

	for i, promptID := range req.PromptIDs {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			return
		}
		prompt := promptMap[promptID]
		e.runPrompt(ctx, runID, req, prompt, endpoint, i, len(req.PromptIDs), ar)
	}
}

func (e *Engine) runPrompt(ctx context.Context, runID string, req StartRequest, prompt model.LabPrompt, endpoint string, index, total int, ar *activeRun) {
	now := time.Now().UnixMilli()

	result := model.OneshotResult{
		PromptID: prompt.ID,
		RunID:    runID,
		Model:    req.ModelID,
		Status:   model.OneshotPromptRunning,
	}

	e.publish(runID, ar.nextSeq(), now, model.EventOneshotTestStarted, map[string]any{
		"runId":   runID,
		"promptId": prompt.ID,
		"index":   index,
		"total":   total,
	})

	e.store.UpsertOneshotResult(result)

	messages := []agent.ChatMessage{{Role: "user", Content: prompt.Prompt}}

	var output string
	var firstTokenMs *int64
	startTime := time.Now()

	resp, err := e.caller.Call(ctx, endpoint, req.ModelID, "", messages, nil, func(delta string) {
		if firstTokenMs == nil {
			elapsed := time.Since(startTime).Milliseconds()
			firstTokenMs = &elapsed
		}
		output += delta
		e.publish(runID, ar.nextSeq(), time.Now().UnixMilli(), model.EventOneshotDelta, map[string]any{
			"runId":    runID,
			"promptId": prompt.ID,
			"content":  delta,
		})
	})

	wallTimeMs := time.Since(startTime).Milliseconds()
	finishTime := time.Now().UnixMilli()

	if err != nil {
		if ctx.Err() != nil {
			result.Status = model.OneshotPromptStopped
			result.Error = "stopped"
		} else {
			result.Status = model.OneshotPromptFailed
			result.Error = err.Error()
		}
		result.FinishedAt = &finishTime
		result.WallTimeMs = &wallTimeMs
		e.store.UpsertOneshotResult(result)
		e.publish(runID, ar.nextSeq(), finishTime, model.EventOneshotTestFinished, map[string]any{
			"runId":      runID,
			"promptId":   prompt.ID,
			"output":     "",
			"finishReason": "",
			"wallTimeMs": wallTimeMs,
			"error":      result.Error,
		})
		return
	}

	result.Output = resp.Message.Content
	result.FinishReason = string(resp.FinishReason)
	result.FinishedAt = &finishTime
	result.WallTimeMs = &wallTimeMs
	result.FirstTokenMs = firstTokenMs
	promptTokens := resp.Metrics.PromptTokens
	completionTokens := resp.Metrics.CompletionTokens
	result.PromptTokens = &promptTokens
	result.CompletionTokens = &completionTokens

	html, hasArtifact := ExtractHTML(resp.Message.Content)
	if hasArtifact {
		artifactPath := e.saveArtifact(prompt.ID, html)
		result.ArtifactPath = artifactPath
		result.HasArtifact = true
	}

	result.Status = model.OneshotPromptDone
	e.store.UpsertOneshotResult(result)

	e.publish(runID, ar.nextSeq(), finishTime, model.EventOneshotTestFinished, map[string]any{
		"runId":        runID,
		"promptId":     prompt.ID,
		"output":       resp.Message.Content,
		"finishReason": string(resp.FinishReason),
		"wallTimeMs":   wallTimeMs,
		"firstTokenMs": firstTokenMs,
		"metrics": map[string]any{
			"promptTokens":     promptTokens,
			"completionTokens": completionTokens,
		},
		"artifact": hasArtifact,
	})
}

func (e *Engine) saveArtifact(promptID, html string) string {
	dir := filepath.Join(e.cfg.DataDir, "artifacts", "oneshot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("create artifacts dir", "err", err)
		return ""
	}
	path := filepath.Join(dir, promptID+".html")
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		slog.Error("write artifact", "prompt_id", promptID, "err", err)
		return ""
	}
	rel, _ := filepath.Rel(e.cfg.DataDir, path)
	return rel
}

func (e *Engine) publish(runID string, seq, ts int64, typ string, payload any) {
	event := model.Event{
		Seq:     seq,
		Ts:      ts,
		Type:    typ,
		Payload: payload,
		RunID:   runID,
	}
	e.hub.Publish(event)
	
	// Store event in database for SSE replay
	if err := e.store.InsertOneshotEvent(runID, seq, ts, typ, payload); err != nil {
		slog.Error("failed to store oneshot event", "run_id", runID, "type", typ, "err", err)
	}
}

func (ar *activeRun) nextSeq() int64 {
	return ar.seq.Add(1) - 1
}

// PromptSummary is a summary of a LabPrompt for the API.
type PromptSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
}

// ListPrompts returns summaries of all available LabPrompts.
func (e *Engine) ListPrompts() ([]PromptSummary, error) {
	prompts, err := LoadLabPrompts("lab_prompts")
	if err != nil {
		return nil, err
	}
	summaries := make([]PromptSummary, len(prompts))
	for i, p := range prompts {
		summaries[i] = PromptSummary{
			ID:       p.ID,
			Title:    p.Title,
			Category: p.Category,
			Prompt:   p.Prompt,
		}
	}
	return summaries, nil
}
