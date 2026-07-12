package storage

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the SQLite database used for runs, events, and oneshot results.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite database at path and enables WAL/foreign keys.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Store{db: db}, nil
}

// Migrate runs pending goose migrations.
func (s *Store) Migrate() error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}
