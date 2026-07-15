-- +goose Up

CREATE TABLE IF NOT EXISTS oneshot_run_events (
  run_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  ts INTEGER NOT NULL,
  type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  PRIMARY KEY(run_id, seq),
  FOREIGN KEY(run_id) REFERENCES oneshot_runs(id)
);

CREATE INDEX IF NOT EXISTS idx_oneshot_events_run ON oneshot_run_events(run_id, seq);

-- +goose Down

DROP INDEX IF EXISTS idx_oneshot_events_run;
DROP TABLE IF EXISTS oneshot_run_events;
