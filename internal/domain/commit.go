package domain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/git"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Commit is the projection-shaped view of a commit entity.
type Commit struct {
	EntityID   int64
	StableID   string
	ProjectID  int64
	RepoID     int64
	SHA        string
	Message    string
	Author     string
	AuthoredAt string
	Parents    []string
	TaskID     *int64
	Status     string
}

// ScanResult summarizes a single scan invocation.
type ScanResult struct {
	Added    int
	Bound    int
	Orphaned int
	Restored int
}

// CommitService handles commit scanning and binding.
type CommitService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// ScanRepo walks the local repo and registers any unseen commits, then
// re-marks orphan/active status across the existing commit set.
// repoRoot is the absolute path to the git working tree.
// Safe to call when the directory is not a git repo: returns zero result, nil error.
func (s *CommitService) ScanRepo(ctx context.Context, projectEntityID, repoEntityID int64, repoRoot string) (ScanResult, error) {
	var res ScanResult

	if !git.IsRepo(repoRoot) {
		return res, nil
	}

	allSHAs, err := git.AllCommitSHAs(repoRoot)
	if err != nil {
		return res, fmt.Errorf("rev-list --all: %w", err)
	}

	// Existing SHAs and their entity IDs for this repo
	rows, err := s.DB.Conn().QueryContext(ctx,
		"SELECT sha, entity_id, status FROM p_commits WHERE repo_id = ?", repoEntityID)
	if err != nil {
		return res, err
	}
	type existing struct {
		entityID int64
		status   string
	}
	known := map[string]existing{}
	for rows.Next() {
		var sha, status string
		var eid int64
		if err := rows.Scan(&sha, &eid, &status); err != nil {
			rows.Close()
			return res, err
		}
		known[sha] = existing{entityID: eid, status: status}
	}
	rows.Close()

	reachable := map[string]struct{}{}
	for _, sha := range allSHAs {
		reachable[sha] = struct{}{}
	}

	tips, err := git.BranchTips(repoRoot)
	if err != nil {
		return res, fmt.Errorf("for-each-ref: %w", err)
	}
	_ = tips // currently unused; reserved for future fast-path branch resolution

	// Register newly seen commits
	newSHAs := []string{}
	for _, sha := range allSHAs {
		if _, ok := known[sha]; !ok {
			newSHAs = append(newSHAs, sha)
		}
	}

	for _, sha := range newSHAs {
		info, err := git.CommitInfoFor(repoRoot, sha)
		if err != nil {
			// Skip unreadable commits but continue
			continue
		}
		taskID, _ := s.resolveTaskForCommit(ctx, projectEntityID, repoRoot, sha)
		if err := s.createCommitEntity(ctx, projectEntityID, repoEntityID, info, taskID); err != nil {
			return res, err
		}
		res.Added++
		if taskID != nil {
			res.Bound++
		}
	}

	// Re-mark orphaned/active based on reachability
	for sha, ex := range known {
		_, isReachable := reachable[sha]
		if !isReachable && ex.status != "orphaned" {
			if err := s.markStatus(ctx, ex.entityID, projectEntityID, "orphaned"); err == nil {
				res.Orphaned++
			}
		} else if isReachable && ex.status == "orphaned" {
			if err := s.markStatus(ctx, ex.entityID, projectEntityID, "active"); err == nil {
				res.Restored++
			}
		}
	}

	return res, nil
}

// resolveTaskForCommit picks the best matching task entity for a commit.
// Strategy: branches containing the commit → look up p_tasks where
// git_branch matches and status is in-progress (preferred) or todo.
func (s *CommitService) resolveTaskForCommit(ctx context.Context, projectEntityID int64, repoRoot, sha string) (*int64, error) {
	branches, err := git.BranchesContaining(repoRoot, sha)
	if err != nil || len(branches) == 0 {
		return nil, err
	}
	// Prefer in-progress, then todo. Iterate over candidate branches.
	for _, status := range []string{"in-progress", "todo"} {
		for _, br := range branches {
			var tid int64
			err := s.DB.Conn().QueryRowContext(ctx,
				"SELECT entity_id FROM p_tasks WHERE project_id = ? AND git_branch = ? AND status = ? ORDER BY entity_id LIMIT 1",
				projectEntityID, br, status,
			).Scan(&tid)
			if err == nil {
				return &tid, nil
			}
		}
	}
	return nil, nil
}

func (s *CommitService) createCommitEntity(ctx context.Context, projectEntityID, repoEntityID int64, info git.CommitInfo, taskID *int64) error {
	stableID := eavt.NewStableID()
	parentsJSON, _ := json.Marshal(info.Parents)

	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return err
		}
		commitID, err := eavt.CreateEntity(sqlTx, stableID, eavt.EntityCommit, txID)
		if err != nil {
			return err
		}
		eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitSha, eavt.NewString(info.SHA), txID)
		eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitProjectID, eavt.NewRef(projectEntityID), txID)
		eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitRepoID, eavt.NewRef(repoEntityID), txID)
		if info.Subject != "" {
			eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitMessage, eavt.NewString(info.Subject), txID)
		}
		if info.Author != "" {
			eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitAuthor, eavt.NewString(info.Author), txID)
		}
		if info.AuthoredAt != "" {
			eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitAuthoredAt, eavt.NewString(info.AuthoredAt), txID)
		}
		if len(info.Parents) > 0 {
			eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitParents, eavt.NewString(string(parentsJSON)), txID)
		}
		if taskID != nil {
			eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitTaskID, eavt.NewRef(*taskID), txID)
		}
		eavt.AssertDatom(sqlTx, commitID, eavt.AttrCommitStatus, eavt.NewEnum("active"), txID)
		return s.Projector.ApplyDatoms(sqlTx, commitID, eavt.EntityCommit, branchID)
	})
}

func (s *CommitService) markStatus(ctx context.Context, commitEntityID, projectEntityID int64, newStatus string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return err
		}
		state, _ := eavt.EntityState(sqlTx, commitEntityID)
		if old, ok := state[eavt.AttrCommitStatus]; ok {
			eavt.RetractDatom(sqlTx, commitEntityID, eavt.AttrCommitStatus, old, txID)
		}
		eavt.AssertDatom(sqlTx, commitEntityID, eavt.AttrCommitStatus, eavt.NewEnum(newStatus), txID)
		return s.Projector.ApplyDatoms(sqlTx, commitEntityID, eavt.EntityCommit, branchID)
	})
}

// BindCommit attaches a previously-unbound commit to a task.
// Used as the manual fallback when scan couldn't resolve.
func (s *CommitService) BindCommit(ctx context.Context, projectEntityID int64, sha string, taskEntityID int64) error {
	var commitEntityID int64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id FROM p_commits WHERE project_id = ? AND sha = ?",
		projectEntityID, sha,
	).Scan(&commitEntityID)
	if err != nil {
		return fmt.Errorf("commit %s not found in project: %w", sha, err)
	}

	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}
		state, _ := eavt.EntityState(sqlTx, commitEntityID)
		if old, ok := state[eavt.AttrCommitTaskID]; ok {
			eavt.RetractDatom(sqlTx, commitEntityID, eavt.AttrCommitTaskID, old, txID)
		}
		eavt.AssertDatom(sqlTx, commitEntityID, eavt.AttrCommitTaskID, eavt.NewRef(taskEntityID), txID)
		return s.Projector.ApplyDatoms(sqlTx, commitEntityID, eavt.EntityCommit, branchID)
	})
}

// UnbindCommit removes the task linkage for a commit.
func (s *CommitService) UnbindCommit(ctx context.Context, projectEntityID int64, sha string) error {
	var commitEntityID int64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id FROM p_commits WHERE project_id = ? AND sha = ?",
		projectEntityID, sha,
	).Scan(&commitEntityID)
	if err != nil {
		return fmt.Errorf("commit %s not found in project: %w", sha, err)
	}

	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}
		state, _ := eavt.EntityState(sqlTx, commitEntityID)
		if old, ok := state[eavt.AttrCommitTaskID]; ok {
			eavt.RetractDatom(sqlTx, commitEntityID, eavt.AttrCommitTaskID, old, txID)
		}
		return s.Projector.ApplyDatoms(sqlTx, commitEntityID, eavt.EntityCommit, branchID)
	})
}

// ListCommits returns commits for a project, optionally filtered by task entity ID.
func (s *CommitService) ListCommits(ctx context.Context, projectEntityID int64, taskFilter *int64) ([]Commit, error) {
	q := `SELECT entity_id, stable_id, project_id, repo_id, sha,
	             COALESCE(message,''), COALESCE(author,''), COALESCE(authored_at,''),
	             COALESCE(parents,'[]'), task_id, status
	      FROM p_commits WHERE project_id = ?`
	args := []any{projectEntityID}
	if taskFilter != nil {
		q += " AND task_id = ?"
		args = append(args, *taskFilter)
	}
	q += " ORDER BY authored_at DESC, entity_id DESC"

	rows, err := s.DB.Conn().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Commit{}
	for rows.Next() {
		var c Commit
		var parentsJSON string
		var taskID sql.NullInt64
		if err := rows.Scan(&c.EntityID, &c.StableID, &c.ProjectID, &c.RepoID, &c.SHA,
			&c.Message, &c.Author, &c.AuthoredAt, &parentsJSON, &taskID, &c.Status); err != nil {
			return nil, err
		}
		if taskID.Valid {
			v := taskID.Int64
			c.TaskID = &v
		}
		_ = json.Unmarshal([]byte(parentsJSON), &c.Parents)
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindCommitBySHAPrefix returns the commit whose SHA starts with the given prefix.
// Errors if the prefix is ambiguous or unknown.
func (s *CommitService) FindCommitBySHAPrefix(ctx context.Context, projectEntityID int64, prefix string) (*Commit, error) {
	prefix = strings.ToLower(prefix)
	q := `SELECT entity_id, stable_id, project_id, repo_id, sha,
	             COALESCE(message,''), COALESCE(author,''), COALESCE(authored_at,''),
	             COALESCE(parents,'[]'), task_id, status
	      FROM p_commits WHERE project_id = ? AND lower(sha) LIKE ? LIMIT 2`
	rows, err := s.DB.Conn().QueryContext(ctx, q, projectEntityID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := []Commit{}
	for rows.Next() {
		var c Commit
		var parentsJSON string
		var taskID sql.NullInt64
		if err := rows.Scan(&c.EntityID, &c.StableID, &c.ProjectID, &c.RepoID, &c.SHA,
			&c.Message, &c.Author, &c.AuthoredAt, &parentsJSON, &taskID, &c.Status); err != nil {
			return nil, err
		}
		if taskID.Valid {
			v := taskID.Int64
			c.TaskID = &v
		}
		_ = json.Unmarshal([]byte(parentsJSON), &c.Parents)
		matches = append(matches, c)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no commit matches prefix %q", prefix)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous commit prefix %q: matches >=2", prefix)
	}
}
