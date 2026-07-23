package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// InsertBatchRun persists a new batch run.
func (s *Store) InsertBatchRun(b model.BatchRun) error {
	configJSON, err := json.Marshal(b.Config)
	if err != nil {
		return fmt.Errorf("marshal batch config: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO batch_runs (id, config_json, status, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)
	`, b.ID, string(configJSON), string(b.Status), b.StartedAt, nullInt64(b.FinishedAt))
	if err != nil {
		return fmt.Errorf("insert batch run: %w", err)
	}
	return nil
}

// GetBatchRun returns a batch run by ID.
func (s *Store) GetBatchRun(id string) (model.BatchRun, error) {
	var b model.BatchRun
	var configJSON string
	var finishedAt sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, config_json, status, started_at, finished_at
		FROM batch_runs WHERE id = ?
	`, id).Scan(&b.ID, &configJSON, &b.Status, &b.StartedAt, &finishedAt)
	if err != nil {
		return model.BatchRun{}, fmt.Errorf("get batch run: %w", err)
	}
	if err := json.Unmarshal([]byte(configJSON), &b.Config); err != nil {
		return model.BatchRun{}, fmt.Errorf("unmarshal batch config: %w", err)
	}
	if finishedAt.Valid {
		b.FinishedAt = &finishedAt.Int64
	}
	return b, nil
}

// ListBatchRuns returns all batch runs ordered by most recent first.
func (s *Store) ListBatchRuns() ([]model.BatchRun, error) {
	rows, err := s.db.Query(`
		SELECT id, config_json, status, started_at, finished_at
		FROM batch_runs
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list batch runs: %w", err)
	}
	defer rows.Close()

	var batches []model.BatchRun
	for rows.Next() {
		var b model.BatchRun
		var configJSON string
		var finishedAt sql.NullInt64
		if err := rows.Scan(&b.ID, &configJSON, &b.Status, &b.StartedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan batch run: %w", err)
		}
		if err := json.Unmarshal([]byte(configJSON), &b.Config); err != nil {
			return nil, fmt.Errorf("unmarshal batch config: %w", err)
		}
		if finishedAt.Valid {
			b.FinishedAt = &finishedAt.Int64
		}
		batches = append(batches, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch runs: %w", err)
	}
	return batches, nil
}

// UpdateBatchRun updates batch run status and finished_at.
func (s *Store) UpdateBatchRun(b model.BatchRun) error {
	_, err := s.db.Exec(`
		UPDATE batch_runs SET
			status = ?,
			finished_at = ?
		WHERE id = ?
	`, string(b.Status), nullInt64(b.FinishedAt), b.ID)
	if err != nil {
		return fmt.Errorf("update batch run: %w", err)
	}
	return nil
}

// ListRunsByBatch returns all runs associated with a batch.
func (s *Store) ListRunsByBatch(batchID string) ([]model.Run, error) {
	rows, err := s.db.Query(`
		SELECT id, started_at, finished_at, status, scenario_ids, runtime, runtime_kind,
			endpoint, model, source, model_file, quant, quant_tier, quant_source, context_size,
			harness, gpu_backend, gpu_model, gpu_count, vram_total_mb, host_thermal_note,
			total_points, max_points, report_path, error, batch_run_id
		FROM runs
		WHERE batch_run_id = ?
		ORDER BY started_at ASC
	`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list runs by batch: %w", err)
	}
	defer rows.Close()

	var runs []model.Run
	for rows.Next() {
		var r model.Run
		var finishedAt sql.NullInt64
		var totalPoints, maxPoints sql.NullInt64
		var scenarioIDs string
		var endpoint, modelFile, quant, quantSource, harness, gpuBackend, gpuModel, hostThermal, reportPath, errMsg, batchRunID sql.NullString
		var quantTier sql.NullFloat64
		var contextSize, gpuCount, vramTotal sql.NullInt64
		if err := rows.Scan(
			&r.ID, &r.StartedAt, &finishedAt, &r.Status, &scenarioIDs, &r.Runtime, &r.RuntimeKind,
			&endpoint, &r.Model, &r.Source, &modelFile, &quant, &quantTier, &quantSource, &contextSize,
			&harness, &gpuBackend, &gpuModel, &gpuCount, &vramTotal, &hostThermal,
			&totalPoints, &maxPoints, &reportPath, &errMsg, &batchRunID,
		); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Int64
		}
		if totalPoints.Valid {
			v := int(totalPoints.Int64)
			r.TotalPoints = &v
		}
		if maxPoints.Valid {
			v := int(maxPoints.Int64)
			r.MaxPoints = &v
		}
		if contextSize.Valid {
			v := int(contextSize.Int64)
			r.ContextSize = &v
		}
		if gpuCount.Valid {
			v := int(gpuCount.Int64)
			r.GPUCount = &v
		}
		if vramTotal.Valid {
			v := int(vramTotal.Int64)
			r.VRAMTotalMB = &v
		}
		if quantTier.Valid {
			r.QuantTier = &quantTier.Float64
		}
		r.Endpoint = endpoint.String
		r.ModelFile = modelFile.String
		r.Quant = quant.String
		r.QuantSource = quantSource.String
		r.Harness = harness.String
		r.GPUBackend = gpuBackend.String
		r.GPUModel = gpuModel.String
		r.HostThermalNote = hostThermal.String
		r.ReportPath = reportPath.String
		r.Error = errMsg.String
		if batchRunID.Valid {
			r.BatchRunID = batchRunID.String
		}
		_ = json.Unmarshal([]byte(scenarioIDs), &r.ScenarioIDs)
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

// MarkRunningBatchesInterrupted marks all batch runs with status "running" as "interrupted".
// This should be called on server startup to clean up stale state from previous crashes.
func (s *Store) MarkRunningBatchesInterrupted() error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`
		UPDATE batch_runs
		SET status = ?, finished_at = ?
		WHERE status = ?
	`, string(model.BatchRunInterrupted), now, string(model.BatchRunRunning))
	if err != nil {
		return fmt.Errorf("mark running batches interrupted: %w", err)
	}
	return nil
}
