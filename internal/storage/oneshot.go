package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// InsertOneshotRun persists a new one-shot run.
func (s *Store) InsertOneshotRun(r model.OneshotRun) error {
	_, err := s.db.Exec(`
		INSERT INTO oneshot_runs (id, started_at, finished_at, status, model, endpoint, prompt_ids, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.ID, r.StartedAt, nullInt64(r.FinishedAt), string(r.Status),
		nullString(r.Model), nullString(r.Endpoint), jsonString(r.PromptIDs), nullString(r.Error),
	)
	if err != nil {
		return fmt.Errorf("insert oneshot run: %w", err)
	}
	return nil
}

// UpdateOneshotRun updates mutable fields of a one-shot run.
func (s *Store) UpdateOneshotRun(r model.OneshotRun) error {
	_, err := s.db.Exec(`
		UPDATE oneshot_runs SET
			status = ?,
			finished_at = ?,
			error = ?
		WHERE id = ?
	`,
		string(r.Status), nullInt64(r.FinishedAt), nullString(r.Error), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update oneshot run: %w", err)
	}
	return nil
}

// GetOneshotRun returns a one-shot run by ID.
func (s *Store) GetOneshotRun(id string) (model.OneshotRun, error) {
	var r model.OneshotRun
	var finishedAt sql.NullInt64
	var promptIDs string
	var modelStr, endpoint, errMsg sql.NullString
	err := s.db.QueryRow(`
		SELECT id, started_at, finished_at, status, model, endpoint, prompt_ids, error
		FROM oneshot_runs WHERE id = ?
	`, id).Scan(&r.ID, &r.StartedAt, &finishedAt, &r.Status, &modelStr, &endpoint, &promptIDs, &errMsg)
	if err != nil {
		return model.OneshotRun{}, fmt.Errorf("get oneshot run: %w", err)
	}
	if finishedAt.Valid {
		r.FinishedAt = &finishedAt.Int64
	}
	r.Model = modelStr.String
	r.Endpoint = endpoint.String
	r.Error = errMsg.String
	_ = json.Unmarshal([]byte(promptIDs), &r.PromptIDs)
	return r, nil
}

// GetLatestOneshotRun returns the most recent one-shot run, or false if none exist.
func (s *Store) GetLatestOneshotRun() (model.OneshotRun, bool, error) {
	var r model.OneshotRun
	var finishedAt sql.NullInt64
	var promptIDs string
	var modelStr, endpoint, errMsg sql.NullString
	err := s.db.QueryRow(`
		SELECT id, started_at, finished_at, status, model, endpoint, prompt_ids, error
		FROM oneshot_runs ORDER BY started_at DESC LIMIT 1
	`).Scan(&r.ID, &r.StartedAt, &finishedAt, &r.Status, &modelStr, &endpoint, &promptIDs, &errMsg)
	if err == sql.ErrNoRows {
		return model.OneshotRun{}, false, nil
	}
	if err != nil {
		return model.OneshotRun{}, false, fmt.Errorf("get latest oneshot run: %w", err)
	}
	if finishedAt.Valid {
		r.FinishedAt = &finishedAt.Int64
	}
	r.Model = modelStr.String
	r.Endpoint = endpoint.String
	r.Error = errMsg.String
	_ = json.Unmarshal([]byte(promptIDs), &r.PromptIDs)
	return r, true, nil
}

// UpsertOneshotResult inserts or replaces a one-shot result (latest-per-prompt).
func (s *Store) UpsertOneshotResult(r model.OneshotResult) error {
	_, err := s.db.Exec(`
		INSERT INTO oneshot_results (
			prompt_id, run_id, model, started_at, finished_at, status, output, finish_reason,
			wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(prompt_id) DO UPDATE SET
			run_id = excluded.run_id,
			model = excluded.model,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			status = excluded.status,
			output = excluded.output,
			finish_reason = excluded.finish_reason,
			wall_time_ms = excluded.wall_time_ms,
			first_token_ms = excluded.first_token_ms,
			prompt_tokens = excluded.prompt_tokens,
			completion_tokens = excluded.completion_tokens,
			artifact_path = excluded.artifact_path,
			error = excluded.error
	`,
		r.PromptID, r.RunID, nullString(r.Model),
		nullInt64(r.StartedAt), nullInt64(r.FinishedAt), string(r.Status),
		nullString(r.Output), nullString(r.FinishReason),
		nullInt64(r.WallTimeMs), nullInt64(r.FirstTokenMs),
		nullInt(r.PromptTokens), nullInt(r.CompletionTokens),
		nullString(r.ArtifactPath), nullString(r.Error),
	)
	if err != nil {
		return fmt.Errorf("upsert oneshot result: %w", err)
	}
	return nil
}

// ResetOneshotPrompts resets results for the given prompt IDs to pending.
func (s *Store) ResetOneshotPrompts(promptIDs []string) error {
	for _, id := range promptIDs {
		_, err := s.db.Exec(`
			DELETE FROM oneshot_results WHERE prompt_id = ?
		`, id)
		if err != nil {
			return fmt.Errorf("reset oneshot prompt %s: %w", id, err)
		}
	}
	return nil
}

// GetOneshotResultsForRun returns all results for a given run ID.
func (s *Store) GetOneshotResultsForRun(runID string) ([]model.OneshotResult, error) {
	rows, err := s.db.Query(`
		SELECT prompt_id, run_id, model, started_at, finished_at, status, output, finish_reason,
			wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error
		FROM oneshot_results
		WHERE run_id = ?
		ORDER BY prompt_id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("query oneshot results: %w", err)
	}
	defer rows.Close()

	var results []model.OneshotResult
	for rows.Next() {
		var r model.OneshotResult
		var startedAt, finishedAt, wallTimeMs, firstTokenMs sql.NullInt64
		var promptTokens, completionTokens sql.NullInt64
		var modelStr, output, finishReason, artifactPath, errMsg sql.NullString
		if err := rows.Scan(
			&r.PromptID, &r.RunID, &modelStr, &startedAt, &finishedAt, &r.Status,
			&output, &finishReason, &wallTimeMs, &firstTokenMs,
			&promptTokens, &completionTokens, &artifactPath, &errMsg,
		); err != nil {
			return nil, fmt.Errorf("scan oneshot result: %w", err)
		}
		r.Model = modelStr.String
		if startedAt.Valid {
			r.StartedAt = &startedAt.Int64
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Int64
		}
		r.Output = output.String
		r.FinishReason = finishReason.String
		if wallTimeMs.Valid {
			r.WallTimeMs = &wallTimeMs.Int64
		}
		if firstTokenMs.Valid {
			r.FirstTokenMs = &firstTokenMs.Int64
		}
		if promptTokens.Valid {
			v := int(promptTokens.Int64)
			r.PromptTokens = &v
		}
		if completionTokens.Valid {
			v := int(completionTokens.Int64)
			r.CompletionTokens = &v
		}
		r.ArtifactPath = artifactPath.String
		r.Error = errMsg.String
		r.HasArtifact = r.ArtifactPath != ""
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oneshot results: %w", err)
	}
	return results, nil
}

// InsertOneshotEvent persists a one-shot run event.
func (s *Store) InsertOneshotEvent(runID string, seq, ts int64, typ string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal oneshot event payload: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO oneshot_run_events (run_id, seq, ts, type, payload_json)
		VALUES (?, ?, ?, ?, ?)
	`, runID, seq, ts, typ, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("insert oneshot event: %w", err)
	}
	return nil
}

// ListOneshotEvents returns persisted events for a one-shot run with sequence greater than fromSeq.
// When fromSeq is -1 all events are returned.
func (s *Store) ListOneshotEvents(runID string, fromSeq int64) ([]model.Event, error) {
	rows, err := s.db.Query(`
		SELECT seq, ts, type, payload_json, run_id
		FROM oneshot_run_events
		WHERE run_id = ? AND seq > ?
		ORDER BY seq ASC
	`, runID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("query oneshot events: %w", err)
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		var payloadJSON string
		if err := rows.Scan(&e.Seq, &e.Ts, &e.Type, &payloadJSON, &e.RunID); err != nil {
			return nil, fmt.Errorf("scan oneshot event: %w", err)
		}
		if err := json.Unmarshal([]byte(payloadJSON), &e.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal oneshot event payload: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oneshot events: %w", err)
	}
	return events, nil
}

// GetAllOneshotResults returns all current results (latest per prompt).
func (s *Store) GetAllOneshotResults() ([]model.OneshotResult, error) {
	rows, err := s.db.Query(`
		SELECT prompt_id, run_id, model, started_at, finished_at, status, output, finish_reason,
			wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error
		FROM oneshot_results
		ORDER BY prompt_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all oneshot results: %w", err)
	}
	defer rows.Close()

	var results []model.OneshotResult
	for rows.Next() {
		var r model.OneshotResult
		var startedAt, finishedAt, wallTimeMs, firstTokenMs sql.NullInt64
		var promptTokens, completionTokens sql.NullInt64
		var modelStr, output, finishReason, artifactPath, errMsg sql.NullString
		if err := rows.Scan(
			&r.PromptID, &r.RunID, &modelStr, &startedAt, &finishedAt, &r.Status,
			&output, &finishReason, &wallTimeMs, &firstTokenMs,
			&promptTokens, &completionTokens, &artifactPath, &errMsg,
		); err != nil {
			return nil, fmt.Errorf("scan oneshot result: %w", err)
		}
		r.Model = modelStr.String
		if startedAt.Valid {
			r.StartedAt = &startedAt.Int64
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Int64
		}
		r.Output = output.String
		r.FinishReason = finishReason.String
		if wallTimeMs.Valid {
			r.WallTimeMs = &wallTimeMs.Int64
		}
		if firstTokenMs.Valid {
			r.FirstTokenMs = &firstTokenMs.Int64
		}
		if promptTokens.Valid {
			v := int(promptTokens.Int64)
			r.PromptTokens = &v
		}
		if completionTokens.Valid {
			v := int(completionTokens.Int64)
			r.CompletionTokens = &v
		}
		r.ArtifactPath = artifactPath.String
		r.Error = errMsg.String
		r.HasArtifact = r.ArtifactPath != ""
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oneshot results: %w", err)
	}
	return results, nil
}
