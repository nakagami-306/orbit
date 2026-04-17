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

// migrations is an ordered list of incremental DDL applied to existing databases.
// Each entry corresponds to a user_version transition: index 0 migrates v0→v1, etc.
// New databases get the full SchemaSQL and skip these entirely.
var migrations = []string{
	// v0 → v1: add Topic entity tables
	`CREATE TABLE IF NOT EXISTS p_topics (
	    entity_id            INTEGER PRIMARY KEY,
	    stable_id            TEXT NOT NULL,
	    project_id           INTEGER NOT NULL,
	    title                TEXT NOT NULL,
	    description          TEXT,
	    status               TEXT NOT NULL DEFAULT 'open',
	    outcome_decision_id  INTEGER,
	    FOREIGN KEY (entity_id) REFERENCES entities(id)
	);
	CREATE TABLE IF NOT EXISTS topic_threads (
	    id        INTEGER PRIMARY KEY AUTOINCREMENT,
	    topic_id  INTEGER NOT NULL,
	    thread_id INTEGER NOT NULL,
	    UNIQUE(topic_id, thread_id),
	    FOREIGN KEY (topic_id) REFERENCES p_topics(entity_id),
	    FOREIGN KEY (thread_id) REFERENCES p_threads(entity_id)
	);`,
	// v1 → v2: add fork_tx_id to p_branches for 3-way merge
	`CREATE TABLE IF NOT EXISTS _p_branches_backup AS SELECT * FROM p_branches;
	DROP TABLE p_branches;
	CREATE TABLE p_branches (
	    entity_id        INTEGER PRIMARY KEY,
	    stable_id        TEXT NOT NULL,
	    project_id       INTEGER NOT NULL,
	    name             TEXT,
	    head_decision_id INTEGER,
	    status           TEXT NOT NULL DEFAULT 'active',
	    is_main          INTEGER NOT NULL DEFAULT 0,
	    fork_tx_id       INTEGER,
	    FOREIGN KEY (entity_id) REFERENCES entities(id)
	);
	INSERT INTO p_branches (entity_id, stable_id, project_id, name, head_decision_id, status, is_main)
	    SELECT entity_id, stable_id, project_id, name, head_decision_id, status, is_main FROM _p_branches_backup;
	DROP TABLE _p_branches_backup;`,
	// v2 → v3: add source_topic_id to p_decisions
	`ALTER TABLE p_decisions ADD COLUMN source_topic_id INTEGER;`,
}

// schemaVersion is the current schema version. Must equal len(migrations).
const schemaVersion = 3

func (d *DB) migrate() error {
	// Check if this is a brand-new database (no tables at all)
	var count int
	err := d.conn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='entities'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Fresh database: apply full schema and set version
		if _, err := d.conn.Exec(SchemaSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if _, err := d.conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("set user_version: %w", err)
		}
		return nil
	}

	// Existing database: apply incremental migrations
	var ver int
	if err := d.conn.QueryRow("PRAGMA user_version").Scan(&ver); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for ver < schemaVersion {
		if _, err := d.conn.Exec(migrations[ver]); err != nil {
			return fmt.Errorf("migration v%d→v%d: %w", ver, ver+1, err)
		}
		ver++
		if _, err := d.conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", ver)); err != nil {
			return fmt.Errorf("set user_version to %d: %w", ver, err)
		}
	}
	return nil
}
