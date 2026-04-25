package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file not created")
	}

	// Verify core tables exist
	tables := []string{"entities", "datoms", "transactions", "operations",
		"p_projects", "p_sections", "p_branches", "p_decisions", "p_topics", "topic_threads", "workspaces"}
	for _, table := range tables {
		var count int
		err := d.Conn().QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestPragmas(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	var journalMode string
	d.Conn().QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	var fk int
	d.Conn().QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestImmutabilityTriggers(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	conn := d.Conn()

	// Insert test data
	conn.Exec("INSERT INTO entities (stable_id, entity_type, created_tx) VALUES ('test-1', 'project', 0)")
	conn.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 1, 'test')")
	conn.Exec(`INSERT INTO datoms (e, a, v, tx, op) VALUES (1, 'project/name', '{"t":"s","v":"test"}', 1, 1)`)

	// UPDATE on datoms should fail
	_, err = conn.Exec(`UPDATE datoms SET v = '{"t":"s","v":"changed"}' WHERE e = 1`)
	if err == nil {
		t.Error("UPDATE on datoms should have been rejected by trigger")
	}

	// DELETE on datoms should fail
	_, err = conn.Exec("DELETE FROM datoms WHERE e = 1")
	if err == nil {
		t.Error("DELETE on datoms should have been rejected by trigger")
	}

	// UPDATE on transactions should fail
	_, err = conn.Exec("UPDATE transactions SET author = 'hacker' WHERE id = 1")
	if err == nil {
		t.Error("UPDATE on transactions should have been rejected by trigger")
	}

	// DELETE on transactions should fail
	_, err = conn.Exec("DELETE FROM transactions WHERE id = 1")
	if err == nil {
		t.Error("DELETE on transactions should have been rejected by trigger")
	}
}

func TestMigrateExistingDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Create a v0 database: full schema minus p_topics/topic_threads, no user_version set
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Simulate a v0 legacy DB by reverting all migrations
	// v3 → v0: drop git integration objects
	d.Conn().Exec("DROP INDEX IF EXISTS idx_p_commits_repo_sha")
	d.Conn().Exec("DROP INDEX IF EXISTS idx_p_commits_task")
	d.Conn().Exec("DROP TABLE IF EXISTS p_commits")
	d.Conn().Exec("DROP TABLE IF EXISTS p_repos")
	d.Conn().Exec("ALTER TABLE p_tasks DROP COLUMN git_branch")
	d.Conn().Exec("ALTER TABLE workspaces DROP COLUMN repo_root")
	// v2 → v0: drop topic + source_topic_id
	d.Conn().Exec("DROP TABLE IF EXISTS topic_threads")
	d.Conn().Exec("DROP TABLE IF EXISTS p_topics")
	d.Conn().Exec("ALTER TABLE p_decisions DROP COLUMN source_topic_id")
	// p_branches: recreate without fork_tx_id
	d.Conn().Exec("CREATE TABLE _pb_bak AS SELECT entity_id, stable_id, project_id, name, head_decision_id, status, is_main FROM p_branches")
	d.Conn().Exec("DROP TABLE p_branches")
	d.Conn().Exec(`CREATE TABLE p_branches (
		entity_id INTEGER PRIMARY KEY, stable_id TEXT NOT NULL, project_id INTEGER NOT NULL,
		name TEXT, head_decision_id INTEGER, status TEXT NOT NULL DEFAULT 'active',
		is_main INTEGER NOT NULL DEFAULT 0, FOREIGN KEY (entity_id) REFERENCES entities(id))`)
	d.Conn().Exec("INSERT INTO p_branches SELECT * FROM _pb_bak")
	d.Conn().Exec("DROP TABLE _pb_bak")
	d.Conn().Exec("PRAGMA user_version = 0")
	d.Close()

	// Re-open should apply incremental migration v0→v1
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after migration: %v", err)
	}
	defer d2.Close()

	// Verify new tables exist
	for _, table := range []string{"p_topics", "topic_threads"} {
		var count int
		err := d2.Conn().QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("table %s not found after migration", table)
		}
	}

	// Verify user_version is current
	var ver int
	d2.Conn().QueryRow("PRAGMA user_version").Scan(&ver)
	if ver != schemaVersion {
		t.Errorf("user_version = %d, want %d", ver, schemaVersion)
	}
}

func TestNewDBSetsVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	var ver int
	d.Conn().QueryRow("PRAGMA user_version").Scan(&ver)
	if ver != schemaVersion {
		t.Errorf("user_version = %d, want %d", ver, schemaVersion)
	}
}

func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	d1.Close()

	// Second open should not fail (migration is idempotent)
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	d2.Close()
}
