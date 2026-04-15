package domain

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/projection"
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

// TestEmptySlicesReturnJSONArray verifies that empty results from List*
// functions marshal to "[]" (not "null") when encoded as JSON.
func TestEmptySlicesReturnJSONArray(t *testing.T) {
	d := setupTestDB(t)
	ctx := context.Background()
	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}

	// First create a project so we have valid IDs to query against
	_, mainBranchID, err := proj.CreateProject(ctx, "test-proj", "desc")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Get the project to retrieve its entity ID
	p, err := proj.GetProjectByName(ctx, "test-proj")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	projectEntityID := p.EntityID

	t.Run("ListProjects_empty_filter", func(t *testing.T) {
		// Filter by a non-existent status to get empty results
		projects, err := proj.ListProjects(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		assertJSONArray(t, projects)
	})

	t.Run("ListTasks_empty", func(t *testing.T) {
		svc := &TaskService{DB: d, Projector: &projection.Projector{}}
		tasks, err := svc.ListTasks(ctx, projectEntityID, "", "")
		if err != nil {
			t.Fatalf("ListTasks: %v", err)
		}
		assertJSONArray(t, tasks)
	})

	t.Run("ListThreads_empty", func(t *testing.T) {
		svc := &ThreadService{DB: d, Projector: &projection.Projector{}}
		threads, err := svc.ListThreads(ctx, projectEntityID, "")
		if err != nil {
			t.Fatalf("ListThreads: %v", err)
		}
		assertJSONArray(t, threads)
	})

	t.Run("ListBranches_nonempty", func(t *testing.T) {
		svc := &BranchService{DB: d, Projector: &projection.Projector{}}
		branches, err := svc.ListBranches(ctx, projectEntityID)
		if err != nil {
			t.Fatalf("ListBranches: %v", err)
		}
		// Should have at least the main branch, but still verify it's an array
		assertJSONArray(t, branches)
	})

	t.Run("GetSections_empty", func(t *testing.T) {
		sections, err := proj.GetSections(ctx, projectEntityID, mainBranchID)
		if err != nil {
			t.Fatalf("GetSections: %v", err)
		}
		assertJSONArray(t, sections)
	})
}

// assertJSONArray verifies that marshaling v produces a JSON array (starting with "["),
// not "null".
func assertJSONArray(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(data)
	if s == "null" {
		t.Errorf("expected JSON array, got null")
	}
	if len(s) == 0 || s[0] != '[' {
		t.Errorf("expected JSON array, got %s", s)
	}
}
