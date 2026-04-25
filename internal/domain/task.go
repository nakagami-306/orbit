package domain

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Task represents a task from projections.
type Task struct {
	EntityID    int64
	StableID    string
	ProjectID   int64
	Title       string
	Description string
	Status      string
	Priority    string
	Assignee    string
	SourceType  string
	SourceID    *int64
	GitBranch   string
}

// TaskService handles task operations.
type TaskService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// CreateTask creates a new task.
func (s *TaskService) CreateTask(ctx context.Context, projectEntityID int64, title, description, priority, assignee string, sourceType string, sourceID *int64) (stableID string, err error) {
	if priority != "" {
		if err := ValidateTaskPriority(priority); err != nil {
			return "", err
		}
	}

	stableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		taskID, err := eavt.CreateEntity(sqlTx, stableID, eavt.EntityTask, txID)
		if err != nil {
			return err
		}

		eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskTitle, eavt.NewString(title), txID)
		eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskProjectID, eavt.NewRef(projectEntityID), txID)
		eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskStatus, eavt.NewEnum("todo"), txID)

		if description != "" {
			eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskDescription, eavt.NewString(description), txID)
		}
		if priority == "" {
			priority = "medium"
		}
		eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskPriority, eavt.NewEnum(priority), txID)
		if assignee != "" {
			eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskAssignee, eavt.NewString(assignee), txID)
		}
		if sourceType != "" {
			eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskSourceType, eavt.NewEnum(sourceType), txID)
		}
		if sourceID != nil {
			eavt.AssertDatom(sqlTx, taskID, eavt.AttrTaskSourceID, eavt.NewRef(*sourceID), txID)
		}

		return s.Projector.ApplyDatoms(sqlTx, taskID, eavt.EntityTask, branchID)
	})
	return
}

// ListTasks returns tasks for a project, optionally filtered.
func (s *TaskService) ListTasks(ctx context.Context, projectEntityID int64, statusFilter, assigneeFilter string) ([]Task, error) {
	query := "SELECT entity_id, stable_id, project_id, title, COALESCE(description,''), status, COALESCE(priority,'medium'), COALESCE(assignee,'') FROM p_tasks WHERE project_id = ?"
	args := []any{projectEntityID}

	if statusFilter != "" {
		query += " AND status = ?"
		args = append(args, statusFilter)
	}
	if assigneeFilter != "" {
		query += " AND assignee = ?"
		args = append(args, assigneeFilter)
	}
	query += " ORDER BY entity_id"

	rows, err := s.DB.Conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Assignee); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateTask updates a task's status and/or assignee.
func (s *TaskService) UpdateTask(ctx context.Context, taskEntityID int64, newStatus, newAssignee string) error {
	if newStatus != "" {
		if err := ValidateTaskStatus(newStatus); err != nil {
			return err
		}
	}

	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		var projectID int64
		sqlTx.QueryRow("SELECT project_id FROM p_tasks WHERE entity_id = ?", taskEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		state, _ := eavt.EntityState(sqlTx, taskEntityID)

		if newStatus != "" {
			// Validate state transition
			if currentVal, ok := state[eavt.AttrTaskStatus]; ok {
				currentStatus, _ := currentVal.AsString()
				if err := ValidateTaskTransition(currentStatus, newStatus); err != nil {
					return err
				}
			}

			if old, ok := state[eavt.AttrTaskStatus]; ok {
				eavt.RetractDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, old, txID)
			}
			eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, eavt.NewEnum(newStatus), txID)
		}
		if newAssignee != "" {
			if old, ok := state[eavt.AttrTaskAssignee]; ok {
				eavt.RetractDatom(sqlTx, taskEntityID, eavt.AttrTaskAssignee, old, txID)
			}
			eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskAssignee, eavt.NewString(newAssignee), txID)
		}

		return s.Projector.ApplyDatoms(sqlTx, taskEntityID, eavt.EntityTask, branchID)
	})
}

// FindTask finds a task by stable ID prefix.
func (s *TaskService) FindTask(ctx context.Context, projectEntityID int64, prefix string) (*Task, error) {
	var t Task
	var gitBranch sql.NullString
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, project_id, title, COALESCE(description,''), status, COALESCE(priority,'medium'), COALESCE(assignee,''), git_branch FROM p_tasks WHERE project_id = ? AND stable_id = ?",
		projectEntityID, prefix,
	).Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Assignee, &gitBranch)
	if err != nil {
		return nil, fmt.Errorf("task %q not found: %w", prefix, err)
	}
	if gitBranch.Valid {
		t.GitBranch = gitBranch.String
	}
	return &t, nil
}

// StartTask transitions a task to in-progress and binds it to a git branch.
// It enforces the 1 active task : 1 branch invariant per project.
// branchName must be non-empty (detached HEAD must be rejected by the caller).
func (s *TaskService) StartTask(ctx context.Context, taskEntityID int64, branchName string) error {
	if branchName == "" {
		return fmt.Errorf("git branch name is required (detached HEAD?)")
	}

	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID int64
		if err := sqlTx.QueryRow("SELECT project_id FROM p_tasks WHERE entity_id = ?", taskEntityID).Scan(&projectID); err != nil {
			return fmt.Errorf("task lookup: %w", err)
		}

		// Reject if another active task already owns this branch in the same project.
		var conflictID int64
		err := sqlTx.QueryRow(
			"SELECT entity_id FROM p_tasks WHERE project_id = ? AND git_branch = ? AND status IN ('todo','in-progress') AND entity_id != ? LIMIT 1",
			projectID, branchName, taskEntityID,
		).Scan(&conflictID)
		if err == nil {
			return fmt.Errorf("git branch %q is already owned by another active task (entity_id=%d)", branchName, conflictID)
		}

		var branchEntityID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchEntityID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchEntityID, "user")
		if err != nil {
			return err
		}

		state, _ := eavt.EntityState(sqlTx, taskEntityID)

		// Status: → in-progress (validate transition from current state)
		if currentVal, ok := state[eavt.AttrTaskStatus]; ok {
			currentStatus, _ := currentVal.AsString()
			if currentStatus != "in-progress" {
				if err := ValidateTaskTransition(currentStatus, "in-progress"); err != nil {
					return err
				}
				eavt.RetractDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, currentVal, txID)
				eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, eavt.NewEnum("in-progress"), txID)
			}
		} else {
			eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, eavt.NewEnum("in-progress"), txID)
		}

		// git_branch: assert (retract previous if any)
		if old, ok := state[eavt.AttrTaskGitBranch]; ok {
			eavt.RetractDatom(sqlTx, taskEntityID, eavt.AttrTaskGitBranch, old, txID)
		}
		eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskGitBranch, eavt.NewString(branchName), txID)

		return s.Projector.ApplyDatoms(sqlTx, taskEntityID, eavt.EntityTask, branchEntityID)
	})
}

// DoneTask transitions a task to done and returns the git branch it was bound to
// (so the caller can run a branch-scoped commit scan before the branch is deleted).
func (s *TaskService) DoneTask(ctx context.Context, taskEntityID int64) (string, error) {
	var gitBranch string

	err := s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID int64
		if err := sqlTx.QueryRow("SELECT project_id FROM p_tasks WHERE entity_id = ?", taskEntityID).Scan(&projectID); err != nil {
			return fmt.Errorf("task lookup: %w", err)
		}

		var branchEntityID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchEntityID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchEntityID, "user")
		if err != nil {
			return err
		}

		state, _ := eavt.EntityState(sqlTx, taskEntityID)

		if currentVal, ok := state[eavt.AttrTaskStatus]; ok {
			currentStatus, _ := currentVal.AsString()
			if currentStatus != "done" {
				if err := ValidateTaskTransition(currentStatus, "done"); err != nil {
					return err
				}
				eavt.RetractDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, currentVal, txID)
				eavt.AssertDatom(sqlTx, taskEntityID, eavt.AttrTaskStatus, eavt.NewEnum("done"), txID)
			}
		}

		if v, ok := state[eavt.AttrTaskGitBranch]; ok {
			gitBranch, _ = v.AsString()
		}

		return s.Projector.ApplyDatoms(sqlTx, taskEntityID, eavt.EntityTask, branchEntityID)
	})
	return gitBranch, err
}
