-- +goose Up

ALTER TABLE runs ADD COLUMN source TEXT NOT NULL DEFAULT 'local' CHECK(source IN ('local','remote'));

-- +goose Down

ALTER TABLE runs DROP COLUMN source;
