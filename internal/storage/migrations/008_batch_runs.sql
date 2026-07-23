-- +goose Up

CREATE TABLE batch_runs (
    id TEXT PRIMARY KEY,
    config_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running' CHECK(status IN ('running','completed','interrupted','failed')),
    started_at INTEGER NOT NULL,
    finished_at INTEGER
);

ALTER TABLE runs ADD COLUMN batch_run_id TEXT REFERENCES batch_runs(id);

-- +goose Down

ALTER TABLE runs DROP COLUMN batch_run_id;

DROP TABLE batch_runs;
