-- +goose Up
-- Allow scenario runs to be recorded as skipped when a manifest requirement is missing.

ALTER TABLE scenario_runs RENAME TO _scenario_runs_old;

CREATE TABLE scenario_runs (
  run_id TEXT NOT NULL REFERENCES runs(id),
  scenario_id TEXT NOT NULL,
  category TEXT,
  family TEXT NOT NULL DEFAULT 'regex-style',
  started_at INTEGER,
  finished_at INTEGER,
  status TEXT CHECK(status IN ('pending','running','pass','partial','fail','stopped','skipped')),

  points INTEGER,
  max_points INTEGER,
  rubric_kind TEXT NOT NULL DEFAULT '10pt' CHECK(rubric_kind IN ('10pt','custom-5pt','custom-3pt')),

  correctness INTEGER,
  scope INTEGER,
  pattern INTEGER,
  verification INTEGER,
  cleanup INTEGER,

  wall_time_ms INTEGER,
  first_token_ms INTEGER,
  tool_call_count INTEGER,
  bash_calls INTEGER,
  post_change_bash_calls INTEGER,
  verify_passes INTEGER,
  mutated INTEGER,
  model_metrics_json TEXT,
  evaluation_json TEXT,
  error_kind TEXT CHECK(error_kind IN ('infra','timeout','aborted','runtime')),
  error TEXT,
  artifact_path TEXT,
  PRIMARY KEY(run_id, scenario_id)
);

INSERT INTO scenario_runs SELECT
  run_id,
  scenario_id,
  category,
  family,
  started_at,
  finished_at,
  status,
  points,
  max_points,
  rubric_kind,
  correctness,
  scope,
  pattern,
  verification,
  cleanup,
  wall_time_ms,
  first_token_ms,
  tool_call_count,
  bash_calls,
  post_change_bash_calls,
  verify_passes,
  mutated,
  model_metrics_json,
  evaluation_json,
  error_kind,
  error,
  artifact_path
FROM _scenario_runs_old;

DROP TABLE _scenario_runs_old;

-- +goose Down

ALTER TABLE scenario_runs RENAME TO _scenario_runs_new;

CREATE TABLE scenario_runs (
  run_id TEXT NOT NULL REFERENCES runs(id),
  scenario_id TEXT NOT NULL,
  category TEXT,
  family TEXT NOT NULL DEFAULT 'regex-style',
  started_at INTEGER,
  finished_at INTEGER,
  status TEXT CHECK(status IN ('pending','running','pass','partial','fail','stopped')),

  points INTEGER,
  max_points INTEGER,
  rubric_kind TEXT NOT NULL DEFAULT '10pt' CHECK(rubric_kind IN ('10pt','custom-5pt','custom-3pt')),

  correctness INTEGER,
  scope INTEGER,
  pattern INTEGER,
  verification INTEGER,
  cleanup INTEGER,

  wall_time_ms INTEGER,
  first_token_ms INTEGER,
  tool_call_count INTEGER,
  bash_calls INTEGER,
  post_change_bash_calls INTEGER,
  verify_passes INTEGER,
  mutated INTEGER,
  model_metrics_json TEXT,
  evaluation_json TEXT,
  error_kind TEXT CHECK(error_kind IN ('infra','timeout','aborted','runtime')),
  error TEXT,
  artifact_path TEXT,
  PRIMARY KEY(run_id, scenario_id)
);

INSERT INTO scenario_runs SELECT
  run_id,
  scenario_id,
  category,
  family,
  started_at,
  finished_at,
  status,
  points,
  max_points,
  rubric_kind,
  correctness,
  scope,
  pattern,
  verification,
  cleanup,
  wall_time_ms,
  first_token_ms,
  tool_call_count,
  bash_calls,
  post_change_bash_calls,
  verify_passes,
  mutated,
  model_metrics_json,
  evaluation_json,
  error_kind,
  error,
  artifact_path
FROM _scenario_runs_new;

DROP TABLE _scenario_runs_new;
