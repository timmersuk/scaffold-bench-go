package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// GetRun returns a run by ID.
func (s *Store) GetRun(id string) (model.Run, error) {
	var r model.Run
	var finishedAt sql.NullInt64
	var totalPoints, maxPoints sql.NullInt64
	var scenarioIDs string
	var endpoint, modelFile, quant, quantSource, harness, gpuBackend, gpuModel, hostThermal, reportPath, errMsg sql.NullString
	var quantTier sql.NullFloat64
	var contextSize, gpuCount, vramTotal sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, started_at, finished_at, status, scenario_ids, runtime, runtime_kind,
			endpoint, model, model_file, quant, quant_tier, quant_source, context_size,
			harness, gpu_backend, gpu_model, gpu_count, vram_total_mb, host_thermal_note,
			total_points, max_points, report_path, error
		FROM runs WHERE id = ?
	`, id).Scan(
		&r.ID, &r.StartedAt, &finishedAt, &r.Status, &scenarioIDs, &r.Runtime, &r.RuntimeKind,
		&endpoint, &r.Model, &modelFile, &quant, &quantTier, &quantSource, &contextSize,
		&harness, &gpuBackend, &gpuModel, &gpuCount, &vramTotal, &hostThermal,
		&totalPoints, &maxPoints, &reportPath, &errMsg,
	)
	if err != nil {
		return model.Run{}, fmt.Errorf("get run: %w", err)
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
	_ = json.Unmarshal([]byte(scenarioIDs), &r.ScenarioIDs)
	return r, nil
}

// InsertRun persists a new benchmark run.
func (s *Store) InsertRun(r model.Run) error {
	_, err := s.db.Exec(`
		INSERT INTO runs (
			id, started_at, finished_at, status, scenario_ids, runtime, runtime_kind,
			endpoint, model, model_file, quant, quant_tier, quant_source, context_size,
			harness, gpu_backend, gpu_model, gpu_count, vram_total_mb, host_thermal_note,
			total_points, max_points, report_path, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.ID, r.StartedAt, nullInt64(r.FinishedAt), string(r.Status), jsonString(r.ScenarioIDs),
		r.Runtime, r.RuntimeKind, nullString(r.Endpoint), r.Model, nullString(r.ModelFile),
		nullString(r.Quant), nullFloat64(r.QuantTier), nullString(r.QuantSource), nullInt(r.ContextSize),
		nullString(r.Harness), nullString(r.GPUBackend), nullString(r.GPUModel), nullInt(r.GPUCount),
		nullInt(r.VRAMTotalMB), nullString(r.HostThermalNote), nullInt(r.TotalPoints),
		nullInt(r.MaxPoints), nullString(r.ReportPath), nullString(r.Error),
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

// UpdateRun updates mutable run fields at the end of a run.
func (s *Store) UpdateRun(r model.Run) error {
	_, err := s.db.Exec(`
		UPDATE runs SET
			status = ?,
			finished_at = ?,
			total_points = ?,
			max_points = ?,
			report_path = ?,
			error = ?
		WHERE id = ?
	`,
		string(r.Status), nullInt64(r.FinishedAt), nullInt(r.TotalPoints), nullInt(r.MaxPoints),
		nullString(r.ReportPath), nullString(r.Error), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update run: %w", err)
	}
	return nil
}

// UpsertScenarioRun inserts or replaces a scenario run row.
func (s *Store) UpsertScenarioRun(sr model.ScenarioRun) error {
	mutated := sql.NullBool{Bool: false, Valid: false}
	if sr.Mutated != nil {
		mutated = sql.NullBool{Bool: *sr.Mutated, Valid: true}
	}
	_, err := s.db.Exec(`
		INSERT INTO scenario_runs (
			run_id, scenario_id, category, family, started_at, finished_at, status,
			points, max_points, rubric_kind, correctness, scope, pattern, verification, cleanup,
			wall_time_ms, first_token_ms, tool_call_count, bash_calls, post_change_bash_calls,
			verify_passes, mutated, model_metrics_json, evaluation_json, error_kind, error, artifact_path
		) 		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, scenario_id) DO UPDATE SET
			category = excluded.category,
			family = excluded.family,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			status = excluded.status,
			points = excluded.points,
			max_points = excluded.max_points,
			rubric_kind = excluded.rubric_kind,
			correctness = excluded.correctness,
			scope = excluded.scope,
			pattern = excluded.pattern,
			verification = excluded.verification,
			cleanup = excluded.cleanup,
			wall_time_ms = excluded.wall_time_ms,
			first_token_ms = excluded.first_token_ms,
			tool_call_count = excluded.tool_call_count,
			bash_calls = excluded.bash_calls,
			post_change_bash_calls = excluded.post_change_bash_calls,
			verify_passes = excluded.verify_passes,
			mutated = excluded.mutated,
			model_metrics_json = excluded.model_metrics_json,
			evaluation_json = excluded.evaluation_json,
			error_kind = excluded.error_kind,
			error = excluded.error,
			artifact_path = excluded.artifact_path
	`,
		sr.RunID, sr.ScenarioID, nullString(sr.Category), sr.Family,
		nullInt64(sr.StartedAt), nullInt64(sr.FinishedAt), string(sr.Status),
		nullInt(sr.Points), sr.MaxPoints, sr.RubricKind,
		nullInt(sr.Correctness), nullInt(sr.Scope), nullInt(sr.Pattern),
		nullInt(sr.Verification), nullInt(sr.Cleanup),
		nullInt64(sr.WallTimeMs), nullInt64(sr.FirstTokenMs), nullInt(sr.ToolCallCount),
		nullInt(sr.BashCalls), nullInt(sr.PostChangeBashCalls), nullInt(sr.VerifyPasses),
		mutated, nullString(sr.ModelMetricsJSON), nullString(sr.EvaluationJSON),
		nullString(sr.ErrorKind), nullString(sr.Error), nullString(sr.ArtifactPath),
	)
	if err != nil {
		return fmt.Errorf("upsert scenario run: %w", err)
	}
	return nil
}

// InsertEvent persists a run event.
func (s *Store) InsertEvent(runID, scenarioID string, seq, ts int64, typ string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO run_events (run_id, scenario_id, seq, ts, type, payload_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, runID, nullString(scenarioID), seq, ts, typ, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func nullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
