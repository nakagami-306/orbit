package domain

import (
	"context"
	"database/sql"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Branch represents a branch from projections.
type Branch struct {
	EntityID       int64
	StableID       string
	ProjectID      int64
	Name           string
	HeadDecisionID *int64
	Status         string
	IsMain         bool
}

// BranchService handles branch operations.
type BranchService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// CreateBranch creates a new branch from a given point (or current head of a branch).
func (s *BranchService) CreateBranch(ctx context.Context, projectEntityID, fromBranchID int64, name string) (branchStableID string, err error) {
	branchStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get head decision of source branch
		var headDecisionID sql.NullInt64
		sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", fromBranchID).Scan(&headDecisionID)

		txID, err := eavt.BeginTx(sqlTx, nil, fromBranchID, "system")
		if err != nil {
			return err
		}

		branchID, err := eavt.CreateEntity(sqlTx, branchStableID, eavt.EntityBranch, txID)
		if err != nil {
			return err
		}

		if name != "" {
			eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchName, eavt.NewString(name), txID)
		}
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchProjectID, eavt.NewRef(projectEntityID), txID)
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchStatus, eavt.NewEnum("active"), txID)
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchIsMain, eavt.NewBool(false), txID)

		if headDecisionID.Valid {
			eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(headDecisionID.Int64), txID)
		}

		if err := s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID); err != nil {
			return err
		}

		// Copy sections from source branch to new branch
		rows, err := sqlTx.Query(`
			SELECT entity_id, stable_id, project_id, title, content, position
			FROM p_sections WHERE project_id = ? AND branch_id = ?
		`, projectEntityID, fromBranchID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var secID int64
			var secStableID, title, content string
			var projID int64
			var position int
			if err := rows.Scan(&secID, &secStableID, &projID, &title, &content, &position); err != nil {
				return err
			}
			// Insert a copy of the section projection for the new branch
			_, err = sqlTx.Exec(`
				INSERT INTO p_sections (entity_id, branch_id, stable_id, project_id, title, content, position, is_stale, stale_reason)
				VALUES (?, ?, ?, ?, ?, ?, ?, 0, '')
			`, secID, branchID, secStableID, projID, title, content, position)
			if err != nil {
				return err
			}
		}

		// Copy section refs
		sqlTx.Exec(`
			INSERT INTO p_section_refs (from_section, to_section, branch_id)
			SELECT from_section, to_section, ? FROM p_section_refs WHERE branch_id = ?
		`, branchID, fromBranchID)

		return nil
	})
	return
}

// SwitchBranch updates the workspace's current branch.
func (s *BranchService) SwitchBranch(ctx context.Context, workspacePath string, branchID int64) error {
	_, err := s.DB.Conn().Exec("UPDATE workspaces SET current_branch_id = ? WHERE path = ?", branchID, normalizePath(workspacePath))
	return err
}

// ListBranches returns all branches for a project.
func (s *BranchService) ListBranches(ctx context.Context, projectEntityID int64) ([]Branch, error) {
	rows, err := s.DB.Conn().QueryContext(ctx, `
		SELECT entity_id, stable_id, project_id, COALESCE(name,''), head_decision_id, status, is_main
		FROM p_branches WHERE project_id = ?
	`, projectEntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var b Branch
		var headID sql.NullInt64
		var isMain int
		if err := rows.Scan(&b.EntityID, &b.StableID, &b.ProjectID, &b.Name, &headID, &b.Status, &isMain); err != nil {
			return nil, err
		}
		if headID.Valid {
			b.HeadDecisionID = &headID.Int64
		}
		b.IsMain = isMain == 1
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

// NameBranch sets a name on an anonymous branch.
func (s *BranchService) NameBranch(ctx context.Context, branchID int64, name string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return err
		}
		// Retract old name if any
		state, _ := eavt.EntityState(sqlTx, branchID)
		if old, ok := state[eavt.AttrBranchName]; ok {
			eavt.RetractDatom(sqlTx, branchID, eavt.AttrBranchName, old, txID)
		}
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchName, eavt.NewString(name), txID)
		return s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID)
	})
}

// AbandonBranch marks a branch as abandoned.
func (s *BranchService) AbandonBranch(ctx context.Context, branchID int64, rationale string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}
		eavt.RetractDatom(sqlTx, branchID, eavt.AttrBranchStatus, eavt.NewEnum("active"), txID)
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchStatus, eavt.NewEnum("abandoned"), txID)
		return s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID)
	})
}

// MergeBranch merges source into target. Creates a merge Decision with 2 parents.
// Returns conflicts if any sections were modified on both branches.
func (s *BranchService) MergeBranch(ctx context.Context, sourceID, targetID, projectEntityID int64, author string) (decisionStableID string, conflictCount int, err error) {
	decisionStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get heads
		var sourceHead, targetHead sql.NullInt64
		sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", sourceID).Scan(&sourceHead)
		sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", targetID).Scan(&targetHead)

		// Create merge decision
		decisionEntityID, err := eavt.CreateEntity(sqlTx, decisionStableID, eavt.EntityDecision, 0)
		if err != nil {
			return err
		}

		txID, err := eavt.BeginTx(sqlTx, &decisionEntityID, targetID, author)
		if err != nil {
			return err
		}

		// Decision datoms
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionTitle, eavt.NewString("Merge branch"), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionAuthor, eavt.NewString(author), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionProjectID, eavt.NewRef(projectEntityID), txID)

		parents := []int64{}
		if targetHead.Valid {
			parents = append(parents, targetHead.Int64)
		}
		if sourceHead.Valid {
			parents = append(parents, sourceHead.Int64)
		}
		if len(parents) > 0 {
			eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionParents, eavt.NewRefSet(parents), txID)
		}

		// Compare sections: find sections on source that differ from target
		sourceRows, err := sqlTx.Query(`
			SELECT s.entity_id, s.title, s.content
			FROM p_sections s WHERE s.project_id = ? AND s.branch_id = ?
		`, projectEntityID, sourceID)
		if err != nil {
			return err
		}

		type secData struct {
			EntityID int64
			Title    string
			Content  string
		}
		var sourceSections []secData
		for sourceRows.Next() {
			var sd secData
			sourceRows.Scan(&sd.EntityID, &sd.Title, &sd.Content)
			sourceSections = append(sourceSections, sd)
		}
		sourceRows.Close()

		for _, ss := range sourceSections {
			// Get target's version of this section
			var targetContent string
			err := sqlTx.QueryRow(
				"SELECT COALESCE(content,'') FROM p_sections WHERE entity_id = ? AND branch_id = ?",
				ss.EntityID, targetID,
			).Scan(&targetContent)

			if err != nil {
				// Section exists on source but not target: auto-merge (add it)
				sqlTx.Exec(`
					INSERT OR IGNORE INTO p_sections (entity_id, branch_id, stable_id, project_id, title, content, position, is_stale)
					SELECT entity_id, ?, stable_id, project_id, title, content, position, 0
					FROM p_sections WHERE entity_id = ? AND branch_id = ?
				`, targetID, ss.EntityID, sourceID)
				continue
			}

			if targetContent == ss.Content {
				continue // No change
			}

			// Both modified: create conflict
			conflictStableID := eavt.NewStableID()
			conflictID, _ := eavt.CreateEntity(sqlTx, conflictStableID, eavt.EntityMilestone, txID) // reuse entity creation

			sqlTx.Exec(`
				INSERT INTO p_conflicts (entity_id, stable_id, project_id, branch_id, section_id, field, base_value, merge_decision_id, status)
				VALUES (?, ?, ?, ?, ?, 'content', '', ?, 'unresolved')
			`, conflictID, conflictStableID, projectEntityID, targetID, ss.EntityID, decisionEntityID)

			sqlTx.Exec(`INSERT INTO p_conflict_sides (conflict_id, branch_id, value) VALUES (?, ?, ?)`,
				conflictID, targetID, targetContent)
			sqlTx.Exec(`INSERT INTO p_conflict_sides (conflict_id, branch_id, value) VALUES (?, ?, ?)`,
				conflictID, sourceID, ss.Content)

			conflictCount++
		}

		// Update target branch head
		if targetHead.Valid {
			eavt.RetractDatom(sqlTx, targetID, eavt.AttrBranchHeadDecision, eavt.NewRef(targetHead.Int64), txID)
		}
		eavt.AssertDatom(sqlTx, targetID, eavt.AttrBranchHeadDecision, eavt.NewRef(decisionEntityID), txID)

		// Mark source as merged
		eavt.RetractDatom(sqlTx, sourceID, eavt.AttrBranchStatus, eavt.NewEnum("active"), txID)
		eavt.AssertDatom(sqlTx, sourceID, eavt.AttrBranchStatus, eavt.NewEnum("merged"), txID)

		// Apply projections
		s.Projector.ApplyDatoms(sqlTx, decisionEntityID, eavt.EntityDecision, targetID)
		s.Projector.ApplyDatoms(sqlTx, targetID, eavt.EntityBranch, targetID)
		s.Projector.ApplyDatoms(sqlTx, sourceID, eavt.EntityBranch, sourceID)

		return nil
	})
	return
}

func normalizePath(p string) string {
	result := make([]byte, len(p))
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' {
			result[i] = '/'
		} else {
			result[i] = p[i]
		}
	}
	return string(result)
}
