-- +goose Up

ALTER TABLE oneshot_results RENAME TO oneshot_results_old;

CREATE TABLE oneshot_results (
  prompt_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  model TEXT,
  started_at INTEGER,
  finished_at INTEGER,
  status TEXT CHECK(status IN ('pending','running','done','failed','stopped')),
  output TEXT,
  finish_reason TEXT,
  wall_time_ms INTEGER,
  first_token_ms INTEGER,
  prompt_tokens INTEGER,
  completion_tokens INTEGER,
  artifact_path TEXT,
  error TEXT
);

INSERT INTO oneshot_results (prompt_id, run_id, model, started_at, finished_at, status, output, finish_reason, wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error)
SELECT prompt_id, run_id, NULL, started_at, finished_at, status, output, finish_reason, wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error
FROM oneshot_results_old;

DROP TABLE oneshot_results_old;

-- +goose Down

ALTER TABLE oneshot_results RENAME TO oneshot_results_new;

CREATE TABLE oneshot_results (
  run_id TEXT NOT NULL,
  prompt_id TEXT NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  status TEXT CHECK(status IN ('pending','running','done','failed','stopped')),
  output TEXT,
  finish_reason TEXT,
  wall_time_ms INTEGER,
  first_token_ms INTEGER,
  prompt_tokens INTEGER,
  completion_tokens INTEGER,
  artifact_path TEXT,
  error TEXT,
  PRIMARY KEY(run_id, prompt_id),
  FOREIGN KEY(run_id) REFERENCES oneshot_runs(id) ON DELETE CASCADE
);

INSERT INTO oneshot_results (run_id, prompt_id, started_at, finished_at, status, output, finish_reason, wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error)
SELECT run_id, prompt_id, started_at, finished_at, status, output, finish_reason, wall_time_ms, first_token_ms, prompt_tokens, completion_tokens, artifact_path, error
FROM oneshot_results_new;

DROP TABLE oneshot_results_new;
