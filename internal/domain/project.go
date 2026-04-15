package domain

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Project represents a project entity from projections.
type Project struct {
	EntityID    int64
	StableID    string
	Name        string
	Description string
	Status      string
}

// Section represents a section entity from projections.
type Section struct {
	EntityID   int64
	StableID   string
	ProjectID  int64
	Title      string
	Content    string
	Position   int
	IsStale    bool
	StaleReason string
}

// ProjectService handles project and section operations.
type ProjectService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// CreateProject creates a new project with a main branch.
// Returns the project's stable ID and main branch entity ID.
func (s *ProjectService) CreateProject(ctx context.Context, name, description string) (projectStableID string, mainBranchID int64, err error) {
	projectStableID = eavt.NewStableID()
	branchStableID := eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// 1. Create a bootstrap transaction (branch needs to exist for tx, but tx needs branch_id)
		// Solution: insert branch entity first with created_tx=0, create tx, then update created_tx
		branchID, err := eavt.CreateEntity(sqlTx, branchStableID, eavt.EntityBranch, 0)
		if err != nil {
			return fmt.Errorf("create branch entity: %w", err)
		}
		mainBranchID = branchID

		// 2. Create EAVT transaction (system operation, no decision)
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		// 3. Create project entity
		projectID, err := eavt.CreateEntity(sqlTx, projectStableID, eavt.EntityProject, txID)
		if err != nil {
			return fmt.Errorf("create project entity: %w", err)
		}

		// 4. Assert project datoms
		if err := eavt.AssertDatom(sqlTx, projectID, eavt.AttrProjectName, eavt.NewString(name), txID); err != nil {
			return err
		}
		if description != "" {
			if err := eavt.AssertDatom(sqlTx, projectID, eavt.AttrProjectDescription, eavt.NewString(description), txID); err != nil {
				return err
			}
		}
		if err := eavt.AssertDatom(sqlTx, projectID, eavt.AttrProjectStatus, eavt.NewEnum("active"), txID); err != nil {
			return err
		}

		// 5. Assert branch datoms
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchName, eavt.NewString("main"), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchProjectID, eavt.NewRef(projectID), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchStatus, eavt.NewEnum("active"), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchIsMain, eavt.NewBool(true), txID); err != nil {
			return err
		}

		// 6. Apply projections
		if err := s.Projector.ApplyDatoms(sqlTx, projectID, eavt.EntityProject, branchID); err != nil {
			return fmt.Errorf("project projection: %w", err)
		}
		if err := s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID); err != nil {
			return fmt.Errorf("branch projection: %w", err)
		}

		return nil
	})

	return
}

// GetProjectByName finds a project by name.
func (s *ProjectService) GetProjectByName(ctx context.Context, name string) (*Project, error) {
	var p Project
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, name, description, status FROM p_projects WHERE name = ?", name,
	).Scan(&p.EntityID, &p.StableID, &p.Name, &p.Description, &p.Status)
	if err != nil {
		return nil, fmt.Errorf("get project %q: %w", name, err)
	}
	return &p, nil
}

// GetProjectByID finds a project by entity ID.
func (s *ProjectService) GetProjectByID(ctx context.Context, entityID int64) (*Project, error) {
	var p Project
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, name, COALESCE(description,''), status FROM p_projects WHERE entity_id = ?", entityID,
	).Scan(&p.EntityID, &p.StableID, &p.Name, &p.Description, &p.Status)
	if err != nil {
		return nil, fmt.Errorf("get project by id %d: %w", entityID, err)
	}
	return &p, nil
}

// ListProjects returns all projects, optionally filtered by status.
func (s *ProjectService) ListProjects(ctx context.Context, statusFilter string) ([]Project, error) {
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = s.DB.Conn().QueryContext(ctx,
			"SELECT entity_id, stable_id, name, COALESCE(description,''), status FROM p_projects WHERE status = ?", statusFilter)
	} else {
		rows, err = s.DB.Conn().QueryContext(ctx,
			"SELECT entity_id, stable_id, name, COALESCE(description,''), status FROM p_projects")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.EntityID, &p.StableID, &p.Name, &p.Description, &p.Status); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetMainBranch returns the main branch for a project.
func (s *ProjectService) GetMainBranch(ctx context.Context, projectEntityID int64) (int64, error) {
	var branchID int64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID,
	).Scan(&branchID)
	if err != nil {
		return 0, fmt.Errorf("get main branch: %w", err)
	}
	return branchID, nil
}

// GetSections returns all sections for a project on a given branch, ordered by position.
func (s *ProjectService) GetSections(ctx context.Context, projectEntityID, branchID int64) ([]Section, error) {
	rows, err := s.DB.Conn().QueryContext(ctx, `
		SELECT entity_id, stable_id, project_id, title, COALESCE(content,''), position, is_stale, COALESCE(stale_reason,'')
		FROM p_sections
		WHERE project_id = ? AND branch_id = ?
		ORDER BY position
	`, projectEntityID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []Section
	for rows.Next() {
		var sec Section
		var isStale int
		if err := rows.Scan(&sec.EntityID, &sec.StableID, &sec.ProjectID, &sec.Title, &sec.Content, &sec.Position, &isStale, &sec.StaleReason); err != nil {
			return nil, err
		}
		sec.IsStale = isStale == 1
		sections = append(sections, sec)
	}
	return sections, rows.Err()
}

// EditSection modifies a section's content and creates a Decision.
// Returns the decision's stable ID.
func (s *ProjectService) EditSection(ctx context.Context, sectionEntityID, branchID, projectEntityID int64, newContent, decisionTitle, rationale, author string) (decisionStableID string, err error) {
	decisionStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get current head decision for this branch
		var currentHead *int64
		var headID int64
		headErr := sqlTx.QueryRow(
			"SELECT head_decision_id FROM p_branches WHERE entity_id = ?", branchID,
		).Scan(&headID)
		if headErr == nil && headID > 0 {
			currentHead = &headID
		}

		// 1. Create decision entity
		decisionEntityID, err := eavt.CreateEntity(sqlTx, decisionStableID, eavt.EntityDecision, 0)
		if err != nil {
			return err
		}

		// 2. Create EAVT transaction linked to decision
		txID, err := eavt.BeginTx(sqlTx, &decisionEntityID, branchID, author)
		if err != nil {
			return err
		}

		// 3. Retract old content, assert new content
		// Get current content to retract
		state, err := eavt.EntityState(sqlTx, sectionEntityID)
		if err != nil {
			return err
		}
		if oldContent, ok := state[eavt.AttrSectionContent]; ok {
			if err := eavt.RetractDatom(sqlTx, sectionEntityID, eavt.AttrSectionContent, oldContent, txID); err != nil {
				return err
			}
		}
		if err := eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionContent, eavt.NewString(newContent), txID); err != nil {
			return err
		}

		// 4. Assert decision datoms
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionTitle, eavt.NewString(decisionTitle), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionRationale, eavt.NewString(rationale), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionAuthor, eavt.NewString(author), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionProjectID, eavt.NewRef(projectEntityID), txID); err != nil {
			return err
		}
		if currentHead != nil {
			if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionParents, eavt.NewRefSet([]int64{*currentHead}), txID); err != nil {
				return err
			}
		}

		// 5. Update branch head
		if currentHead != nil {
			oldHeadVal := eavt.NewRef(*currentHead)
			eavt.RetractDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, oldHeadVal, txID)
		}
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(decisionEntityID), txID); err != nil {
			return err
		}

		// 6. Apply projections
		if err := s.Projector.ApplyDatoms(sqlTx, sectionEntityID, eavt.EntitySection, branchID); err != nil {
			return err
		}
		if err := s.Projector.ApplyDatoms(sqlTx, decisionEntityID, eavt.EntityDecision, branchID); err != nil {
			return err
		}
		if err := s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID); err != nil {
			return err
		}

		// 7. Update last_decision_id on the section projection
		sqlTx.Exec("UPDATE p_sections SET last_decision_id = ? WHERE entity_id = ? AND branch_id = ?",
			decisionEntityID, sectionEntityID, branchID)

		// 8. Stale detection
		if err := s.Projector.MarkStale(sqlTx, sectionEntityID, branchID, decisionEntityID); err != nil {
			return err
		}

		return nil
	})
	return
}

// AddSection creates a new section and a Decision.
func (s *ProjectService) AddSection(ctx context.Context, projectEntityID, branchID int64, title, content string, position int, decisionTitle, rationale, author string) (sectionStableID, decisionStableID string, err error) {
	sectionStableID = eavt.NewStableID()
	decisionStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get current head
		var currentHead *int64
		var headID int64
		headErr := sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", branchID).Scan(&headID)
		if headErr == nil && headID > 0 {
			currentHead = &headID
		}

		// Create decision entity
		decisionEntityID, err := eavt.CreateEntity(sqlTx, decisionStableID, eavt.EntityDecision, 0)
		if err != nil {
			return err
		}

		// Create EAVT transaction
		txID, err := eavt.BeginTx(sqlTx, &decisionEntityID, branchID, author)
		if err != nil {
			return err
		}

		// Create section entity
		sectionEntityID, err := eavt.CreateEntity(sqlTx, sectionStableID, eavt.EntitySection, txID)
		if err != nil {
			return err
		}

		// Assert section datoms
		if err := eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionTitle, eavt.NewString(title), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionContent, eavt.NewString(content), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionPosition, eavt.NewInt(int64(position)), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionProjectID, eavt.NewRef(projectEntityID), txID); err != nil {
			return err
		}

		// Assert decision datoms
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionTitle, eavt.NewString(decisionTitle), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionRationale, eavt.NewString(rationale), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionAuthor, eavt.NewString(author), txID); err != nil {
			return err
		}
		if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionProjectID, eavt.NewRef(projectEntityID), txID); err != nil {
			return err
		}
		if currentHead != nil {
			if err := eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionParents, eavt.NewRefSet([]int64{*currentHead}), txID); err != nil {
				return err
			}
		}

		// Update branch head
		if currentHead != nil {
			eavt.RetractDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(*currentHead), txID)
		}
		if err := eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(decisionEntityID), txID); err != nil {
			return err
		}

		// Apply projections
		if err := s.Projector.ApplyDatoms(sqlTx, sectionEntityID, eavt.EntitySection, branchID); err != nil {
			return err
		}
		if err := s.Projector.ApplyDatoms(sqlTx, decisionEntityID, eavt.EntityDecision, branchID); err != nil {
			return err
		}
		if err := s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID); err != nil {
			return err
		}

		return nil
	})
	return
}
