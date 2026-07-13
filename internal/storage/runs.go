package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

// ListRuns returns all persisted benchmark runs ordered by most recent first.
func (s *Store) ListRuns() ([]model.Run, error) {
	rows, err := s.db.Query(`
		SELECT id, started_at, finished_at, status, scenario_ids, runtime, runtime_kind,
			endpoint, model, model_file, quant, quant_tier, quant_source, context_size,
			harness, gpu_backend, gpu_model, gpu_count, vram_total_mb, host_thermal_note,
			total_points, max_points, report_path, error
		FROM runs
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []model.Run
	for rows.Next() {
		var r model.Run
		var finishedAt sql.NullInt64
		var totalPoints, maxPoints sql.NullInt64
		var scenarioIDs string
		var endpoint, modelFile, quant, quantSource, harness, gpuBackend, gpuModel, hostThermal, reportPath, errMsg sql.NullString
		var quantTier sql.NullFloat64
		var contextSize, gpuCount, vramTotal sql.NullInt64
		if err := rows.Scan(
			&r.ID, &r.StartedAt, &finishedAt, &r.Status, &scenarioIDs, &r.Runtime, &r.RuntimeKind,
			&endpoint, &r.Model, &modelFile, &quant, &quantTier, &quantSource, &contextSize,
			&harness, &gpuBackend, &gpuModel, &gpuCount, &vramTotal, &hostThermal,
			&totalPoints, &maxPoints, &reportPath, &errMsg,
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
		_ = json.Unmarshal([]byte(scenarioIDs), &r.ScenarioIDs)
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

// GetRunWithScenarios returns a run by ID along with its scenario runs.
func (s *Store) GetRunWithScenarios(id string) (model.Run, []model.ScenarioRun, error) {
	r, err := s.GetRun(id)
	if err != nil {
		return model.Run{}, nil, err
	}

	rows, err := s.db.Query(`
		SELECT run_id, scenario_id, category, family, started_at, finished_at, status,
			points, max_points, rubric_kind, correctness, scope, pattern, verification, cleanup,
			wall_time_ms, first_token_ms, tool_call_count, bash_calls, post_change_bash_calls,
			verify_passes, mutated, model_metrics_json, evaluation_json, error_kind, error, artifact_path
		FROM scenario_runs
		WHERE run_id = ?
		ORDER BY started_at ASC
	`, id)
	if err != nil {
		return model.Run{}, nil, fmt.Errorf("query scenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []model.ScenarioRun
	for rows.Next() {
		var sr model.ScenarioRun
		var startedAt, finishedAt, wallTimeMs, firstTokenMs sql.NullInt64
		var points, correctness, scope, pattern, verification, cleanup, maxPoints sql.NullInt64
		var toolCallCount, bashCalls, postChangeBashCalls, verifyPasses sql.NullInt64
		var category, family, rubricKind, modelMetricsJSON, evaluationJSON, errorKind, errorMsg, artifactPath sql.NullString
		var mutated sql.NullBool
		if err := rows.Scan(
			&sr.RunID, &sr.ScenarioID, &category, &family, &startedAt, &finishedAt, &sr.Status,
			&points, &maxPoints, &rubricKind, &correctness, &scope, &pattern, &verification, &cleanup,
			&wallTimeMs, &firstTokenMs, &toolCallCount, &bashCalls, &postChangeBashCalls, &verifyPasses,
			&mutated, &modelMetricsJSON, &evaluationJSON, &errorKind, &errorMsg, &artifactPath,
		); err != nil {
			return model.Run{}, nil, fmt.Errorf("scan scenario run: %w", err)
		}
		if startedAt.Valid {
			sr.StartedAt = &startedAt.Int64
		}
		if finishedAt.Valid {
			sr.FinishedAt = &finishedAt.Int64
		}
		if points.Valid {
			v := int(points.Int64)
			sr.Points = &v
		}
		if maxPoints.Valid {
			sr.MaxPoints = int(maxPoints.Int64)
		}
		if correctness.Valid {
			v := int(correctness.Int64)
			sr.Correctness = &v
		}
		if scope.Valid {
			v := int(scope.Int64)
			sr.Scope = &v
		}
		if pattern.Valid {
			v := int(pattern.Int64)
			sr.Pattern = &v
		}
		if verification.Valid {
			v := int(verification.Int64)
			sr.Verification = &v
		}
		if cleanup.Valid {
			v := int(cleanup.Int64)
			sr.Cleanup = &v
		}
		if wallTimeMs.Valid {
			sr.WallTimeMs = &wallTimeMs.Int64
		}
		if firstTokenMs.Valid {
			sr.FirstTokenMs = &firstTokenMs.Int64
		}
		if toolCallCount.Valid {
			v := int(toolCallCount.Int64)
			sr.ToolCallCount = &v
		}
		if bashCalls.Valid {
			v := int(bashCalls.Int64)
			sr.BashCalls = &v
		}
		if postChangeBashCalls.Valid {
			v := int(postChangeBashCalls.Int64)
			sr.PostChangeBashCalls = &v
		}
		if verifyPasses.Valid {
			v := int(verifyPasses.Int64)
			sr.VerifyPasses = &v
		}
		if mutated.Valid {
			v := mutated.Bool
			sr.Mutated = &v
		}
		sr.Category = category.String
		sr.Family = family.String
		sr.RubricKind = rubricKind.String
		sr.ModelMetricsJSON = modelMetricsJSON.String
		sr.EvaluationJSON = evaluationJSON.String
		sr.ErrorKind = errorKind.String
		sr.Error = errorMsg.String
		sr.ArtifactPath = artifactPath.String
		scenarios = append(scenarios, sr)
	}
	if err := rows.Err(); err != nil {
		return model.Run{}, nil, fmt.Errorf("iterate scenarios: %w", err)
	}
	return r, scenarios, nil
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
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

// ListEvents returns persisted events for a run with sequence greater than fromSeq.
// When fromSeq is -1 all events are returned.
func (s *Store) ListEvents(runID string, fromSeq int64) ([]model.Event, error) {
	rows, err := s.db.Query(`
		SELECT seq, ts, type, payload_json, run_id, scenario_id
		FROM run_events
		WHERE run_id = ? AND seq > ?
		ORDER BY seq ASC
	`, runID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		var payloadJSON string
		var scenarioID sql.NullString
		if err := rows.Scan(&e.Seq, &e.Ts, &e.Type, &payloadJSON, &e.RunID, &scenarioID); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if scenarioID.Valid {
			e.ScenarioID = scenarioID.String
		}
		if err := json.Unmarshal([]byte(payloadJSON), &e.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal event payload: %w", err)
		}
		if e.Payload == nil {
			e.Payload = map[string]any{}
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
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
