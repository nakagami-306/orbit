package eavt

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nakagami-306/orbit/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("setup db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestValueRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  Value
	}{
		{"string", NewString("hello")},
		{"int", NewInt(42)},
		{"bool_true", NewBool(true)},
		{"bool_false", NewBool(false)},
		{"ref", NewRef(123)},
		{"enum", NewEnum("active")},
		{"datetime", NewDateTime(time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC))},
		{"refset", NewRefSet([]int64{1, 2, 3})},
		{"empty_string", NewString("")},
		{"zero_int", NewInt(0)},
		{"empty_refset", NewRefSet([]int64{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.val.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			decoded, err := DecodeValue(encoded)
			if err != nil {
				t.Fatalf("Decode(%q): %v", encoded, err)
			}
			if decoded.Type != tt.val.Type {
				t.Errorf("type = %s, want %s", decoded.Type, tt.val.Type)
			}

			// Re-encode and compare
			reEncoded, err := decoded.Encode()
			if err != nil {
				t.Fatalf("re-Encode: %v", err)
			}
			if reEncoded != encoded {
				t.Errorf("re-encoded = %q, want %q", reEncoded, encoded)
			}
		})
	}
}

func TestCreateEntityAndLookup(t *testing.T) {
	d := setupTestDB(t)
	conn := d.Conn()
	sqlTx, err := conn.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer sqlTx.Rollback()

	// Need a bootstrap tx row for created_tx
	_, err = sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	if err != nil {
		t.Fatalf("insert bootstrap tx: %v", err)
	}

	stableID := NewStableID()
	entityID, err := CreateEntity(sqlTx, stableID, EntityProject, 1)
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if entityID <= 0 {
		t.Errorf("entity ID = %d, want > 0", entityID)
	}

	// Lookup by stable ID
	gotID, gotType, err := GetEntityByStableID(sqlTx, stableID)
	if err != nil {
		t.Fatalf("GetEntityByStableID: %v", err)
	}
	if gotID != entityID {
		t.Errorf("id = %d, want %d", gotID, entityID)
	}
	if gotType != EntityProject {
		t.Errorf("type = %s, want %s", gotType, EntityProject)
	}

	// Lookup stable ID by internal ID
	gotStable, err := GetStableID(sqlTx, entityID)
	if err != nil {
		t.Fatalf("GetStableID: %v", err)
	}
	if gotStable != stableID {
		t.Errorf("stable_id = %s, want %s", gotStable, stableID)
	}
}

func TestDatomAssertAndQuery(t *testing.T) {
	d := setupTestDB(t)
	conn := d.Conn()
	sqlTx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlTx.Rollback()

	// Bootstrap: entity + transaction
	sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	entityID, err := CreateEntity(sqlTx, NewStableID(), EntityProject, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Assert datoms
	txID := int64(1)
	if err := AssertDatom(sqlTx, entityID, AttrProjectName, NewString("MyProject"), txID); err != nil {
		t.Fatal(err)
	}
	if err := AssertDatom(sqlTx, entityID, AttrProjectStatus, NewEnum("active"), txID); err != nil {
		t.Fatal(err)
	}

	// Query entity state
	state, err := EntityState(sqlTx, entityID)
	if err != nil {
		t.Fatal(err)
	}

	name, err := state[AttrProjectName].AsString()
	if err != nil {
		t.Fatal(err)
	}
	if name != "MyProject" {
		t.Errorf("name = %q, want %q", name, "MyProject")
	}

	status, err := state[AttrProjectStatus].AsString()
	if err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Errorf("status = %q, want %q", status, "active")
	}
}

func TestRetractAndState(t *testing.T) {
	d := setupTestDB(t)
	conn := d.Conn()
	sqlTx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlTx.Rollback()

	// Bootstrap
	sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	entityID, _ := CreateEntity(sqlTx, NewStableID(), EntityProject, 1)

	// Tx 1: assert name
	AssertDatom(sqlTx, entityID, AttrProjectName, NewString("OldName"), 1)

	// Tx 2: retract old name, assert new name
	RetractDatom(sqlTx, entityID, AttrProjectName, NewString("OldName"), 2)
	AssertDatom(sqlTx, entityID, AttrProjectName, NewString("NewName"), 2)

	// Current state should show NewName
	state, _ := EntityState(sqlTx, entityID)
	name, _ := state[AttrProjectName].AsString()
	if name != "NewName" {
		t.Errorf("current name = %q, want %q", name, "NewName")
	}

	// As-of tx=1 should show OldName
	stateV1, _ := EntityStateAsOf(sqlTx, entityID, 1)
	nameV1, _ := stateV1[AttrProjectName].AsString()
	if nameV1 != "OldName" {
		t.Errorf("as-of tx=1 name = %q, want %q", nameV1, "OldName")
	}
}

func TestEntitiesByAttribute(t *testing.T) {
	d := setupTestDB(t)
	conn := d.Conn()
	sqlTx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlTx.Rollback()

	sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	e1, _ := CreateEntity(sqlTx, NewStableID(), EntityProject, 1)
	e2, _ := CreateEntity(sqlTx, NewStableID(), EntityProject, 1)

	AssertDatom(sqlTx, e1, AttrProjectStatus, NewEnum("active"), 1)
	AssertDatom(sqlTx, e2, AttrProjectStatus, NewEnum("active"), 1)

	ids, err := EntitiesByAttribute(sqlTx, AttrProjectStatus, NewEnum("active"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("got %d entities, want 2", len(ids))
	}
}

func TestNewStringCRLFNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"crlf_to_lf", "line1\r\nline2\r\nline3", "line1\nline2\nline3"},
		{"lf_unchanged", "line1\nline2\nline3", "line1\nline2\nline3"},
		{"mixed_crlf_lf", "line1\r\nline2\nline3\r\n", "line1\nline2\nline3\n"},
		{"no_newlines", "hello world", "hello world"},
		{"empty", "", ""},
		{"only_crlf", "\r\n", "\n"},
		{"bare_cr_unchanged", "line1\rline2", "line1\rline2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewString(tt.input)
			got, err := v.AsString()
			if err != nil {
				t.Fatalf("AsString: %v", err)
			}
			if got != tt.want {
				t.Errorf("NewString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDatomsForTx(t *testing.T) {
	d := setupTestDB(t)
	conn := d.Conn()
	sqlTx, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlTx.Rollback()

	sqlTx.Exec("INSERT INTO transactions (decision_id, branch_id, author) VALUES (NULL, 0, 'test')")
	entityID, _ := CreateEntity(sqlTx, NewStableID(), EntityProject, 1)

	AssertDatom(sqlTx, entityID, AttrProjectName, NewString("Test"), 1)
	AssertDatom(sqlTx, entityID, AttrProjectStatus, NewEnum("active"), 1)

	datoms, err := DatomsForTx(sqlTx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(datoms) != 2 {
		t.Errorf("got %d datoms, want 2", len(datoms))
	}
}
