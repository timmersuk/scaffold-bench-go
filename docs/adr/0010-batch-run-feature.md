# Batch Run Feature: Persisted Orchestration

We decided to persist batch runs in the database with full drill-down to individual run details, rather than using in-memory orchestration. This gives us reproducibility, audit trails, and the ability to review historical batch configurations and results.

## Context

Batch runs execute multiple models across multiple scenarios with repeated runs for statistical significance. The upstream CLI script `run-all-models.ts` manages this client-side, but we need a web UI that survives browser refreshes and provides full visibility into batch progress and results.

## Decision

**Persisted orchestration**: Batch runs are stored in a `batch_runs` table with configuration, status, and timestamps. Individual runs reference their batch via `batch_run_id`.

**Schema**:
- `batch_runs` table: id, config JSON (models, scenarios, runs per model, warmup, harness), status, started_at, finished_at
- `runs` table gets nullable `batch_run_id` foreign key

**API**:
- `POST /api/batch-runs` - start batch
- `GET /api/batch-runs` - list all batches
- `GET /api/batch-runs/:id` - batch details with per-model summary
- `POST /api/batch-runs/:id/stop` - halt batch
- Existing `/api/runs/:id` unchanged, just has `batch_run_id` if part of batch

**UI**:
- New "Batches" nav item
- Batches page: list of batches with status, date, models, scenarios
- Start batch form: model selection, scenario selection, runs per model, warmup duration, harness
- Progress page: real-time status, completed runs, stop button
- Drill-down: click batch → per-model summary → individual runs → full Dashboard view

**Edge cases**:
- Server restart: mark batch as "interrupted", require manual restart
- Model endpoint down: skip and continue, mark model as failed
- Partial runs: count as failed, no auto-retry

**Lifecycle**: running → completed/interrupted/failed

## Consequences

- **Reproducibility**: Users can review exact batch configurations and results from any point in time
- **Audit trail**: Full history of which models were tested, when, and with what parameters
- **Schema complexity**: New table and foreign key relationship, but individual runs remain the source of truth for execution details
- **Recovery**: Interrupted batches require manual restart, but this prevents accidental re-execution of expensive runs

## Future Work

- **Configurable timeouts**: The idle timeout (5 minutes) and per-scenario hard timeout (10 minutes) are currently hardcoded in `internal/agent/model.go` and `internal/agent/agent.go`. These should be made configurable via the batch run config or runtime settings to accommodate different model speeds and scenario complexities. Consider exposing these in the batch start form and storing them in the batch config JSON.
