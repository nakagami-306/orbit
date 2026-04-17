package domain

import (
	"context"
	"testing"

	"github.com/nakagami-306/orbit/internal/projection"
)

// TestThreeWayMerge_NoFalseConflict verifies that modifying different sections
// on separate branches does not produce a conflict (the core 3-way merge fix).
func TestThreeWayMerge_NoFalseConflict(t *testing.T) {
	d := setupTestDB(t)
	ctx := context.Background()
	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}
	branchSvc := &BranchService{DB: d, Projector: &projection.Projector{}}

	// Create project with main branch
	_, mainBranchID, err := proj.CreateProject(ctx, "test", "desc")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, _ := proj.GetProjectByName(ctx, "test")

	// Add two sections on main
	_, _, err = proj.AddSection(ctx, p.EntityID, mainBranchID, "Section A", "initial A", 0, "add A", "", "user")
	if err != nil {
		t.Fatalf("add section A: %v", err)
	}
	_, _, err = proj.AddSection(ctx, p.EntityID, mainBranchID, "Section B", "initial B", 1, "add B", "", "user")
	if err != nil {
		t.Fatalf("add section B: %v", err)
	}

	sections, _ := proj.GetSections(ctx, p.EntityID, mainBranchID)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	secA := sections[0]
	secB := sections[1]

	// Create a feature branch (fork point recorded here)
	_, err = branchSvc.CreateBranch(ctx, p.EntityID, mainBranchID, "feature")
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	branches, _ := branchSvc.ListBranches(ctx, p.EntityID)
	var featureBranchID int64
	for _, b := range branches {
		if b.Name == "feature" {
			featureBranchID = b.EntityID
			if b.ForkTxID == nil {
				t.Fatal("fork_tx_id should be set on new branch")
			}
			break
		}
	}
	if featureBranchID == 0 {
		t.Fatal("feature branch not found")
	}

	// Edit Section A on feature branch
	_, err = proj.EditSection(ctx, secA.EntityID, featureBranchID, p.EntityID, "modified A on feature", "edit A", "", "user")
	if err != nil {
		t.Fatalf("edit section A on feature: %v", err)
	}

	// Edit Section B on main branch
	_, err = proj.EditSection(ctx, secB.EntityID, mainBranchID, p.EntityID, "modified B on main", "edit B", "", "user")
	if err != nil {
		t.Fatalf("edit section B on main: %v", err)
	}

	// Merge feature → main: should produce 0 conflicts (3-way merge)
	_, conflictCount, err := branchSvc.MergeBranch(ctx, featureBranchID, mainBranchID, p.EntityID, "user")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if conflictCount != 0 {
		t.Errorf("expected 0 conflicts (3-way merge), got %d", conflictCount)
	}

	// Verify: main should have both modifications
	merged, _ := proj.GetSections(ctx, p.EntityID, mainBranchID)
	for _, s := range merged {
		switch s.Title {
		case "Section A":
			if s.Content != "modified A on feature" {
				t.Errorf("Section A: expected 'modified A on feature', got %q", s.Content)
			}
		case "Section B":
			if s.Content != "modified B on main" {
				t.Errorf("Section B: expected 'modified B on main', got %q", s.Content)
			}
		}
	}
}

// TestThreeWayMerge_RealConflict verifies that modifying the same section
// on both branches produces a genuine conflict.
func TestThreeWayMerge_RealConflict(t *testing.T) {
	d := setupTestDB(t)
	ctx := context.Background()
	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}
	branchSvc := &BranchService{DB: d, Projector: &projection.Projector{}}

	_, mainBranchID, err := proj.CreateProject(ctx, "test", "desc")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, _ := proj.GetProjectByName(ctx, "test")

	// Add one section
	_, _, err = proj.AddSection(ctx, p.EntityID, mainBranchID, "Shared", "initial content", 0, "add", "", "user")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sections, _ := proj.GetSections(ctx, p.EntityID, mainBranchID)
	sec := sections[0]

	// Create feature branch
	_, err = branchSvc.CreateBranch(ctx, p.EntityID, mainBranchID, "feature")
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	branches, _ := branchSvc.ListBranches(ctx, p.EntityID)
	var featureBranchID int64
	for _, b := range branches {
		if b.Name == "feature" {
			featureBranchID = b.EntityID
			break
		}
	}

	// Edit same section on both branches with different content
	_, err = proj.EditSection(ctx, sec.EntityID, featureBranchID, p.EntityID, "version A", "edit", "", "user")
	if err != nil {
		t.Fatalf("edit on feature: %v", err)
	}
	_, err = proj.EditSection(ctx, sec.EntityID, mainBranchID, p.EntityID, "version B", "edit", "", "user")
	if err != nil {
		t.Fatalf("edit on main: %v", err)
	}

	// Merge: should produce exactly 1 conflict
	_, conflictCount, err := branchSvc.MergeBranch(ctx, featureBranchID, mainBranchID, p.EntityID, "user")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if conflictCount != 1 {
		t.Errorf("expected 1 conflict, got %d", conflictCount)
	}
}

// TestThreeWayMerge_NewSectionOnBranch verifies that a section added on a
// feature branch (not present at fork point) is auto-merged to target.
func TestThreeWayMerge_NewSectionOnBranch(t *testing.T) {
	d := setupTestDB(t)
	ctx := context.Background()
	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}
	branchSvc := &BranchService{DB: d, Projector: &projection.Projector{}}

	_, mainBranchID, err := proj.CreateProject(ctx, "test", "desc")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, _ := proj.GetProjectByName(ctx, "test")

	// Create feature branch (no sections yet)
	_, err = branchSvc.CreateBranch(ctx, p.EntityID, mainBranchID, "feature")
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	branches, _ := branchSvc.ListBranches(ctx, p.EntityID)
	var featureBranchID int64
	for _, b := range branches {
		if b.Name == "feature" {
			featureBranchID = b.EntityID
			break
		}
	}

	// Add a new section on the feature branch
	_, _, err = proj.AddSection(ctx, p.EntityID, featureBranchID, "New Section", "new content", 0, "add new", "", "user")
	if err != nil {
		t.Fatalf("add section on feature: %v", err)
	}

	// Merge: new section should be auto-merged, 0 conflicts
	_, conflictCount, err := branchSvc.MergeBranch(ctx, featureBranchID, mainBranchID, p.EntityID, "user")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if conflictCount != 0 {
		t.Errorf("expected 0 conflicts, got %d", conflictCount)
	}

	// Verify section exists on main
	mainSections, _ := proj.GetSections(ctx, p.EntityID, mainBranchID)
	found := false
	for _, s := range mainSections {
		if s.Title == "New Section" && s.Content == "new content" {
			found = true
		}
	}
	if !found {
		t.Error("new section was not merged to main")
	}
}
