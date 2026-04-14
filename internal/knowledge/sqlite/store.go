package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3" // SQLite driver providing libsqlite3 symbols for sqlite-vec
)

func init() {
	// Registers sqlite-vec as a SQLite auto-extension. Every new connection
	// opened via the "sqlite3" driver will have vec0 / vec_* functions loaded.
	sqlite_vec.Auto()
}

// SqliteStore persists and queries knowledge documents using SQLite + sqlite-vec.
type SqliteStore struct {
	db *sql.DB
}

// New creates a new SqliteStore at the given file path.
// Parent directories are created automatically.
func New(_ context.Context, path string) (*SqliteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlite: mkdir %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}

	return &SqliteStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *SqliteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use in migrations (e.g. from cmd/index).
func (s *SqliteStore) DB() *sql.DB {
	return s.db
}
