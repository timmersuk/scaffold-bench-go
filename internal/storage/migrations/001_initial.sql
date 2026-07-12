-- +goose Up

CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  status TEXT NOT NULL CHECK(status IN ('running','done','failed','stopped')),

  scenario_ids TEXT NOT NULL,

  runtime TEXT NOT NULL,
  runtime_kind TEXT NOT NULL DEFAULT 'llama.cpp',
  endpoint TEXT,
  model TEXT NOT NULL,
  model_file TEXT,
  quant TEXT,
  quant_tier REAL,
  quant_source TEXT,
  context_size INTEGER,
  harness TEXT,

  gpu_backend TEXT,
  gpu_model TEXT,
  gpu_count INTEGER,
  vram_total_mb INTEGER,
  host_thermal_note TEXT,

  total_points INTEGER,
  max_points INTEGER,
  report_path TEXT,
  error TEXT
);

CREATE TABLE IF NOT EXISTS scenario_runs (
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

CREATE TABLE IF NOT EXISTS run_events (
  run_id TEXT NOT NULL,
  scenario_id TEXT,
  seq INTEGER NOT NULL,
  ts INTEGER NOT NULL,
  type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  PRIMARY KEY(run_id, seq),
  FOREIGN KEY(run_id) REFERENCES runs(id)
);

CREATE INDEX IF NOT EXISTS idx_scenario_runs_by_scenario ON scenario_runs(scenario_id);
CREATE INDEX IF NOT EXISTS idx_runs_by_model_quant ON runs(model, quant);
CREATE INDEX IF NOT EXISTS idx_events_scenario ON run_events(run_id, scenario_id, seq);

-- +goose Down

DROP INDEX IF EXISTS idx_events_scenario;
DROP INDEX IF EXISTS idx_runs_by_model_quant;
DROP INDEX IF EXISTS idx_scenario_runs_by_scenario;
DROP TABLE IF EXISTS run_events;
DROP TABLE IF EXISTS scenario_runs;
DROP TABLE IF EXISTS runs;
