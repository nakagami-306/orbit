package domain

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/projection"
)

func setupTestService(t *testing.T) *ProjectService {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("setup db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return &ProjectService{
		DB:        d,
		Projector: &projection.Projector{},
	}
}

// createTestProject creates a project with a main branch and returns their IDs.
func createTestProject(t *testing.T, svc *ProjectService) (projectEntityID, branchID int64) {
	t.Helper()
	ctx := context.Background()
	_, branchID, err := svc.CreateProject(ctx, "TestProject", "A test project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	p, err := svc.GetProjectByName(ctx, "TestProject")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	return p.EntityID, branchID
}

func TestAddAndRemoveSectionRef(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	// Create two sections
	secASID, _, err := svc.AddSection(ctx, projectID, branchID, "Architecture", "Arch content", 0, "Add arch", "init", "user")
	if err != nil {
		t.Fatalf("AddSection A: %v", err)
	}
	secBSID, _, err := svc.AddSection(ctx, projectID, branchID, "Implementation", "Impl content", 1, "Add impl", "init", "user")
	if err != nil {
		t.Fatalf("AddSection B: %v", err)
	}

	_ = secASID
	_ = secBSID

	// Find sections
	secA, err := svc.FindSectionByNameOrID(ctx, "Architecture", projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection A: %v", err)
	}
	secB, err := svc.FindSectionByNameOrID(ctx, "Implementation", projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection B: %v", err)
	}

	// Add ref: Architecture → Implementation
	decSID, err := svc.AddSectionRef(ctx, secA.EntityID, secB.EntityID, branchID, projectID, "user")
	if err != nil {
		t.Fatalf("AddSectionRef: %v", err)
	}
	if decSID == "" {
		t.Fatal("decision stable ID is empty")
	}

	// Verify ref via GetSection
	detail, err := svc.GetSection(ctx, secA.EntityID, branchID)
	if err != nil {
		t.Fatalf("GetSection A: %v", err)
	}
	if len(detail.RefsTo) != 1 {
		t.Fatalf("expected 1 ref_to, got %d", len(detail.RefsTo))
	}
	if detail.RefsTo[0].Title != "Implementation" {
		t.Errorf("ref_to title = %q, want %q", detail.RefsTo[0].Title, "Implementation")
	}

	// Verify reverse ref
	detailB, err := svc.GetSection(ctx, secB.EntityID, branchID)
	if err != nil {
		t.Fatalf("GetSection B: %v", err)
	}
	if len(detailB.RefsFrom) != 1 {
		t.Fatalf("expected 1 ref_from, got %d", len(detailB.RefsFrom))
	}
	if detailB.RefsFrom[0].Title != "Architecture" {
		t.Errorf("ref_from title = %q, want %q", detailB.RefsFrom[0].Title, "Architecture")
	}

	// Remove ref
	decSID2, err := svc.RemoveSectionRef(ctx, secA.EntityID, secB.EntityID, branchID, projectID, "user")
	if err != nil {
		t.Fatalf("RemoveSectionRef: %v", err)
	}
	if decSID2 == "" {
		t.Fatal("decision stable ID is empty after remove")
	}

	// Verify ref is gone
	detail2, err := svc.GetSection(ctx, secA.EntityID, branchID)
	if err != nil {
		t.Fatalf("GetSection A after remove: %v", err)
	}
	if len(detail2.RefsTo) != 0 {
		t.Errorf("expected 0 refs_to after remove, got %d", len(detail2.RefsTo))
	}
}

func TestRemoveSection(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	// Create a section
	_, _, err := svc.AddSection(ctx, projectID, branchID, "ToRemove", "Will be removed", 0, "Add section", "init", "user")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	sec, err := svc.FindSectionByNameOrID(ctx, "ToRemove", projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection: %v", err)
	}

	// Remove it
	decSID, warnings, err := svc.RemoveSection(ctx, sec.EntityID, branchID, projectID, "No longer needed", "user")
	if err != nil {
		t.Fatalf("RemoveSection: %v", err)
	}
	if decSID == "" {
		t.Fatal("decision stable ID is empty")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Verify section is gone from projections
	sections, err := svc.GetSections(ctx, projectID, branchID)
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	for _, s := range sections {
		if s.Title == "ToRemove" {
			t.Error("section 'ToRemove' still in projections after removal")
		}
	}
}

func TestRemoveSectionWithDanglingRef(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	// Create two sections with a ref
	_, _, err := svc.AddSection(ctx, projectID, branchID, "Referrer", "Refers to target", 0, "Add referrer", "init", "user")
	if err != nil {
		t.Fatalf("AddSection referrer: %v", err)
	}
	_, _, err = svc.AddSection(ctx, projectID, branchID, "Target", "The target", 1, "Add target", "init", "user")
	if err != nil {
		t.Fatalf("AddSection target: %v", err)
	}

	referrer, _ := svc.FindSectionByNameOrID(ctx, "Referrer", projectID, branchID)
	target, _ := svc.FindSectionByNameOrID(ctx, "Target", projectID, branchID)

	// Add ref: Referrer → Target
	svc.AddSectionRef(ctx, referrer.EntityID, target.EntityID, branchID, projectID, "user")

	// Remove Target (should produce warning about dangling ref)
	_, warnings, err := svc.RemoveSection(ctx, target.EntityID, branchID, projectID, "Cleanup", "user")
	if err != nil {
		t.Fatalf("RemoveSection: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected dangling ref warning, got none")
	}
}

func TestGetSection(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	_, _, err := svc.AddSection(ctx, projectID, branchID, "Overview", "Project overview", 0, "Add overview", "init", "user")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	sec, err := svc.FindSectionByNameOrID(ctx, "Overview", projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection: %v", err)
	}

	detail, err := svc.GetSection(ctx, sec.EntityID, branchID)
	if err != nil {
		t.Fatalf("GetSection: %v", err)
	}

	if detail.Title != "Overview" {
		t.Errorf("title = %q, want %q", detail.Title, "Overview")
	}
	if detail.Content != "Project overview" {
		t.Errorf("content = %q, want %q", detail.Content, "Project overview")
	}
	if detail.IsStale {
		t.Error("section should not be stale")
	}
}

func TestFindSectionByNameOrID(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	secSID, _, err := svc.AddSection(ctx, projectID, branchID, "API Design", "API content", 0, "Add API", "init", "user")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	// Find by title
	sec, err := svc.FindSectionByNameOrID(ctx, "API Design", projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection by title: %v", err)
	}
	if sec.Title != "API Design" {
		t.Errorf("title = %q, want %q", sec.Title, "API Design")
	}

	// Find by full stable_id
	sec2, err := svc.FindSectionByNameOrID(ctx, secSID, projectID, branchID)
	if err != nil {
		t.Fatalf("FindSection by stable_id: %v", err)
	}
	if sec2.Title != "API Design" {
		t.Errorf("title = %q, want %q", sec2.Title, "API Design")
	}

	// Find non-existent section
	_, err = svc.FindSectionByNameOrID(ctx, "NonExistent", projectID, branchID)
	if err == nil {
		t.Error("expected error for non-existent section")
	}
}

func TestStaleDetectionViaRef(t *testing.T) {
	svc := setupTestService(t)
	ctx := context.Background()
	projectID, branchID := createTestProject(t, svc)

	// Create two sections
	_, _, err := svc.AddSection(ctx, projectID, branchID, "Design", "Design v1", 0, "Add design", "init", "user")
	if err != nil {
		t.Fatalf("AddSection design: %v", err)
	}
	_, _, err = svc.AddSection(ctx, projectID, branchID, "Impl", "Impl v1", 1, "Add impl", "init", "user")
	if err != nil {
		t.Fatalf("AddSection impl: %v", err)
	}

	design, _ := svc.FindSectionByNameOrID(ctx, "Design", projectID, branchID)
	impl, _ := svc.FindSectionByNameOrID(ctx, "Impl", projectID, branchID)

	// Add ref: Impl → Design (Impl references Design)
	_, err = svc.AddSectionRef(ctx, impl.EntityID, design.EntityID, branchID, projectID, "user")
	if err != nil {
		t.Fatalf("AddSectionRef: %v", err)
	}

	// Edit Design — this should mark Impl as stale
	_, err = svc.EditSection(ctx, design.EntityID, branchID, projectID, "Design v2", "Update design", "design changed", "user")
	if err != nil {
		t.Fatalf("EditSection: %v", err)
	}

	// Check that Impl is now stale
	implDetail, err := svc.GetSection(ctx, impl.EntityID, branchID)
	if err != nil {
		t.Fatalf("GetSection impl: %v", err)
	}
	if !implDetail.IsStale {
		t.Error("Impl should be marked as stale after Design was edited")
	}
}
