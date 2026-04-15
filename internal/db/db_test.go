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
		"p_projects", "p_sections", "p_branches", "p_decisions", "workspaces"}
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
