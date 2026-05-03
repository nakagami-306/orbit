package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/git"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/spf13/cobra"
)

func commitService(app *App) *domain.CommitService {
	return &domain.CommitService{DB: app.DB, Projector: &projection.Projector{}}
}

func repoService(app *App) *domain.RepoService {
	return &domain.RepoService{DB: app.DB, Projector: &projection.Projector{}}
}

func newCommitCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit operations (git integration)",
	}
	cmd.AddCommand(newCommitListCmd(app))
	cmd.AddCommand(newCommitBindCmd(app))
	cmd.AddCommand(newCommitUnbindCmd(app))
	return cmd
}

func newCommitListCmd(app *App) *cobra.Command {
	var taskFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List commits in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			svc := commitService(app)

			var taskID *int64
			if taskFilter != "" {
				t, err := taskService(app).FindTask(cmd.Context(), info.ProjectEntityID, taskFilter)
				if err != nil {
					return err
				}
				taskID = &t.EntityID
			}

			commits, err := svc.ListCommits(cmd.Context(), info.ProjectEntityID, taskID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(commits)
			}

			if len(commits) == 0 {
				fmt.Println("No commits.")
				return nil
			}
			for _, c := range commits {
				task := "unbound"
				if c.TaskID != nil {
					task = fmt.Sprintf("task=%d", *c.TaskID)
				}
				short := c.SHA
				if len(short) > 12 {
					short = short[:12]
				}
				fmt.Printf("%s  [%s]  %s — %s\n", short, c.Status, task, c.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task ID")
	return cmd
}

func newCommitBindCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bind <sha> <task-id>",
		Short: "Manually bind a commit to a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			cs := commitService(app)
			ts := taskService(app)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if !git.IsRepo(cwd) {
				return fmt.Errorf("not inside a git repository: %s", cwd)
			}
			repoRoot, err := git.ToplevelDir(cwd)
			if err != nil {
				return err
			}
			rs := repoService(app)
			remoteURL, _ := git.RemoteURL(repoRoot, "origin")
			repo, err := rs.EnsureRepo(ctx, info.ProjectEntityID, remoteURL)
			if err != nil {
				return fmt.Errorf("ensure repo: %w", err)
			}

			// Resolve full SHA: prefer DB (already-tracked commit), fall back to git
			// for commits Orbit hasn't seen yet (the common case under Option F,
			// where unbound commits are intentionally not registered).
			var fullSHA string
			if c, ferr := cs.FindCommitBySHAPrefix(ctx, info.ProjectEntityID, args[0]); ferr == nil {
				fullSHA = c.SHA
			} else {
				gitInfo, gerr := git.CommitInfoFor(repoRoot, args[0])
				if gerr != nil {
					return fmt.Errorf("commit %s not found (in Orbit or git): %w", args[0], gerr)
				}
				fullSHA = gitInfo.SHA
			}

			t, err := ts.FindTask(ctx, info.ProjectEntityID, args[1])
			if err != nil {
				return err
			}

			if err := cs.BindCommit(ctx, info.ProjectEntityID, repo.EntityID, repoRoot, fullSHA, t.EntityID); err != nil {
				return err
			}
			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "bound", "sha": fullSHA, "task_id": t.StableID,
				})
			}
			fmt.Printf("Bound %s → task %q\n", fullSHA[:12], t.Title)
			return nil
		},
	}
	return cmd
}

func newCommitUnbindCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unbind <sha>",
		Short: "Remove the task linkage for a commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			cs := commitService(app)
			c, err := cs.FindCommitBySHAPrefix(cmd.Context(), info.ProjectEntityID, args[0])
			if err != nil {
				return err
			}
			if err := cs.UnbindCommit(cmd.Context(), info.ProjectEntityID, c.SHA); err != nil {
				return err
			}
			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "unbound", "sha": c.SHA})
			}
			fmt.Printf("Unbound %s\n", c.SHA[:12])
			return nil
		},
	}
	return cmd
}

func newSyncCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Scan the git repository for new commits and bind them to tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			res, err := runScanForProject(cmd.Context(), app, info.ProjectEntityID)
			if err != nil {
				return err
			}
			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			fmt.Printf("Scan complete: +%d added, +%d bound, %d orphaned, %d restored\n",
				res.Added, res.Bound, res.Orphaned, res.Restored)
			return nil
		},
	}
	return cmd
}

// runScanForProject is the shared scan path used by `orbit sync`, `orbit task done`,
// and the auto-scan PersistentPreRun hook.
//
// Returns nil ScanResult, nil error if the project's working tree is not a git repo —
// scan is a no-op in that case (Orbit can manage non-code projects too).
func runScanForProject(ctx context.Context, app *App, projectEntityID int64) (*domain.ScanResult, error) {
	// Locate the working tree for this workspace.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if !git.IsRepo(cwd) {
		return &domain.ScanResult{}, nil
	}
	repoRoot, err := git.ToplevelDir(cwd)
	if err != nil {
		return &domain.ScanResult{}, nil
	}

	rs := repoService(app)
	remoteURL, _ := git.RemoteURL(repoRoot, "origin")
	repo, err := rs.EnsureRepo(ctx, projectEntityID, remoteURL)
	if err != nil {
		return nil, fmt.Errorf("ensure repo: %w", err)
	}

	cs := commitService(app)
	res, err := cs.ScanRepo(ctx, projectEntityID, repo.EntityID, repoRoot)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
