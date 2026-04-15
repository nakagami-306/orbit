package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection with Orbit-specific configuration.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the Orbit SQLite database at the given path.
// It sets pragmas and runs schema migrations.
func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set pragmas
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	d := &DB{conn: conn, path: path}

	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn returns the underlying *sql.DB for direct query access.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// Tx executes fn within a SQL transaction. It commits on success, rolls back on error.
func (d *DB) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (d *DB) migrate() error {
	// Check if schema already exists
	var count int
	err := d.conn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='entities'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // already migrated
	}

	// Apply full schema
	_, err = d.conn.Exec(SchemaSQL)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
