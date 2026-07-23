package batch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

// StartRequest is the payload for POST /api/batch-runs.
type StartRequest struct {
	ModelIDs       []string `json:"modelIds"`
	ScenarioIDs    []string `json:"scenarioIds"`
	RunsPerModel   int      `json:"runsPerModel"`
	WarmupDuration int      `json:"warmupDuration"`
	Harness        string   `json:"harness"`
}

// Engine orchestrates batch runs.
type Engine struct {
	store      *storage.Store
	runner     *runner.Engine
	mu         sync.Mutex
	active     map[string]*activeBatch
	stopCh     map[string]chan struct{}
}

type activeBatch struct {
	batchID string
	cancel  context.CancelFunc
}

// NewEngine creates a batch engine.
func NewEngine(store *storage.Store, runner *runner.Engine) *Engine {
	return &Engine{
		store:  store,
		runner: runner,
		active: make(map[string]*activeBatch),
		stopCh: make(map[string]chan struct{}),
	}
}

// Start begins a new batch run and returns its ID.
func (e *Engine) Start(req StartRequest) (string, error) {
	if len(req.ModelIDs) == 0 {
		return "", fmt.Errorf("no models specified")
	}
	if len(req.ScenarioIDs) == 0 {
		return "", fmt.Errorf("no scenarios specified")
	}
	if req.RunsPerModel <= 0 {
		return "", fmt.Errorf("runs per model must be positive")
	}

	batchID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())

	batch := model.BatchRun{
		ID: batchID,
		Config: model.BatchRunConfig{
			ModelIDs:       req.ModelIDs,
			ScenarioIDs:    req.ScenarioIDs,
			RunsPerModel:   req.RunsPerModel,
			WarmupDuration: req.WarmupDuration,
			Harness:        req.Harness,
		},
		Status:    model.BatchRunRunning,
		StartedAt: time.Now().UnixMilli(),
	}

	if err := e.store.InsertBatchRun(batch); err != nil {
		cancel()
		return "", fmt.Errorf("insert batch run: %w", err)
	}

	e.mu.Lock()
	e.active[batchID] = &activeBatch{
		batchID: batchID,
		cancel:  cancel,
	}
	e.stopCh[batchID] = make(chan struct{})
	e.mu.Unlock()

	go e.runBatch(ctx, batch)

	return batchID, nil
}

// Stop halts a running batch.
func (e *Engine) Stop(batchID string) error {
	e.mu.Lock()
	ab, ok := e.active[batchID]
	stopCh := e.stopCh[batchID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("batch %s not active", batchID)
	}

	ab.cancel()
	close(stopCh)

	e.mu.Lock()
	delete(e.active, batchID)
	delete(e.stopCh, batchID)
	e.mu.Unlock()

	batch, err := e.store.GetBatchRun(batchID)
	if err != nil {
		return fmt.Errorf("get batch run: %w", err)
	}

	now := time.Now().UnixMilli()
	batch.Status = model.BatchRunInterrupted
	batch.FinishedAt = &now

	if err := e.store.UpdateBatchRun(batch); err != nil {
		return fmt.Errorf("update batch run: %w", err)
	}

	return nil
}

// ActiveBatchID returns the ID of the currently running batch, if any.
func (e *Engine) ActiveBatchID() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id := range e.active {
		return id, true
	}
	return "", false
}

func (e *Engine) runBatch(ctx context.Context, batch model.BatchRun) {
	defer func() {
		e.mu.Lock()
		delete(e.active, batch.ID)
		delete(e.stopCh, batch.ID)
		e.mu.Unlock()
	}()

	slog.Info("starting batch run", "batchId", batch.ID, "models", len(batch.Config.ModelIDs), "scenarios", len(batch.Config.ScenarioIDs), "runsPerModel", batch.Config.RunsPerModel)

	totalRuns := len(batch.Config.ModelIDs) * batch.Config.RunsPerModel
	completedRuns := 0

	for _, modelID := range batch.Config.ModelIDs {
		for runNum := 0; runNum < batch.Config.RunsPerModel; runNum++ {
			select {
			case <-ctx.Done():
				slog.Info("batch run stopped", "batchId", batch.ID)
				return
			default:
			}

			if batch.Config.WarmupDuration > 0 && (completedRuns > 0 || runNum > 0) {
				slog.Info("warmup wait", "batchId", batch.ID, "duration", batch.Config.WarmupDuration)
				select {
				case <-time.After(time.Duration(batch.Config.WarmupDuration) * time.Second):
				case <-ctx.Done():
					return
				}
			}

			slog.Info("starting run", "batchId", batch.ID, "model", modelID, "run", runNum+1, "of", batch.Config.RunsPerModel)

			req := runner.StartRequest{
				ScenarioIDs: batch.Config.ScenarioIDs,
				ModelID:     modelID,
				Harness:     batch.Config.Harness,
			}

			runID, err := e.runner.Start(req)
			if err != nil {
				slog.Error("failed to start run", "batchId", batch.ID, "model", modelID, "error", err)
				continue
			}

			if err := e.associateRunWithBatch(runID, batch.ID); err != nil {
				slog.Error("failed to associate run with batch", "runId", runID, "batchId", batch.ID, "error", err)
			}

			if err := e.waitForRun(ctx, runID); err != nil {
				slog.Error("run failed", "runId", runID, "batchId", batch.ID, "error", err)
			}

			completedRuns++
			slog.Info("run completed", "batchId", batch.ID, "runId", runID, "progress", fmt.Sprintf("%d/%d", completedRuns, totalRuns))
		}
	}

	now := time.Now().UnixMilli()
	batch.Status = model.BatchRunCompleted
	batch.FinishedAt = &now

	if err := e.store.UpdateBatchRun(batch); err != nil {
		slog.Error("failed to update batch run", "batchId", batch.ID, "error", err)
	}

	slog.Info("batch run completed", "batchId", batch.ID, "totalRuns", completedRuns)
}

func (e *Engine) associateRunWithBatch(runID, batchID string) error {
	return e.store.UpdateRunBatchID(runID, batchID)
}

func (e *Engine) waitForRun(ctx context.Context, runID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			run, err := e.store.GetRun(runID)
			if err != nil {
				return err
			}
			if run.Status == model.RunDone || run.Status == model.RunFailed || run.Status == model.RunStopped {
				return nil
			}
		}
	}
}
