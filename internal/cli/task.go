package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/git"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/spf13/cobra"
)

func newTaskCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Task operations",
	}
	cmd.AddCommand(newTaskCreateCmd(app))
	cmd.AddCommand(newTaskListCmd(app))
	cmd.AddCommand(newTaskUpdateCmd(app))
	cmd.AddCommand(newTaskStartCmd(app))
	cmd.AddCommand(newTaskDoneCmd(app))
	return cmd
}

func taskService(app *App) *domain.TaskService {
	return &domain.TaskService{DB: app.DB, Projector: &projection.Projector{}}
}

func newTaskCreateCmd(app *App) *cobra.Command {
	var source, assignee, priority string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]

			if err := domain.ValidateTaskPriority(priority); err != nil {
				return err
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve --source flag to entity type and ID
			var sourceType string
			var sourceID *int64
			if source != "" {
				var eid int64
				var etype string
				err := app.DB.Conn().QueryRow(
					"SELECT id, entity_type FROM entities WHERE stable_id = ?",
					source,
				).Scan(&eid, &etype)
				if err != nil {
					return fmt.Errorf("source %q not found: %w", source, err)
				}
				if etype != "decision" && etype != "thread" {
					return fmt.Errorf("source must be a decision or thread, got %s", etype)
				}
				sourceType = etype
				sourceID = &eid
			}

			svc := taskService(app)
			stableID, err := svc.CreateTask(cmd.Context(), info.ProjectEntityID, title, "", priority, assignee, sourceType, sourceID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "task_id": stableID, "title": title,
				})
			}
			fmt.Printf("Created task %q (%s)\n", title, stableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Source decision or thread ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")
	cmd.Flags().StringVar(&priority, "priority", "m", "Priority: h/m/l")
	return cmd
}

func newTaskListCmd(app *App) *cobra.Command {
	var statusFilter, assigneeFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := taskService(app)
			tasks, err := svc.ListTasks(cmd.Context(), info.ProjectEntityID, statusFilter, assigneeFilter)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(tasks)
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks.")
				return nil
			}
			for _, t := range tasks {
				assignee := ""
				if t.Assignee != "" {
					assignee = fmt.Sprintf(" @%s", t.Assignee)
				}
				fmt.Printf("%s  [%s] [%s]  %s%s\n", t.StableID, t.Status, t.Priority, t.Title, assignee)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status")
	cmd.Flags().StringVar(&assigneeFilter, "assignee", "", "Filter by assignee")
	return cmd
}

func newTaskStartCmd(app *App) *cobra.Command {
	var branchOverride string

	cmd := &cobra.Command{
		Use:   "start <task-id>",
		Short: "Start a task and bind it to the current git branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			svc := taskService(app)
			task, err := svc.FindTask(cmd.Context(), info.ProjectEntityID, args[0])
			if err != nil {
				return err
			}

			branch := branchOverride
			if branch == "" {
				cwd, _ := os.Getwd()
				branch, err = git.CurrentBranch(cwd)
				if err != nil {
					if errors.Is(err, git.ErrDetachedHEAD) {
						return fmt.Errorf("HEAD is detached; pass --git-branch to override or check out a branch first")
					}
					return fmt.Errorf("detect git branch: %w", err)
				}
			}

			if err := svc.StartTask(cmd.Context(), task.EntityID, branch); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "started", "task_id": task.StableID, "git_branch": branch,
				})
			}
			fmt.Printf("Task %q started on branch %s\n", task.Title, branch)
			return nil
		},
	}
	cmd.Flags().StringVar(&branchOverride, "git-branch", "", "Override the auto-detected git branch name")
	return cmd
}

func newTaskDoneCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <task-id>",
		Short: "Mark a task done and scan its branch for commits",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}
			svc := taskService(app)
			task, err := svc.FindTask(cmd.Context(), info.ProjectEntityID, args[0])
			if err != nil {
				return err
			}

			branch, err := svc.DoneTask(cmd.Context(), task.EntityID)
			if err != nil {
				return err
			}

			// Branch-scoped scan: ensure commits on this branch get bound before
			// the developer deletes the branch.
			scanResult, scanErr := runScanForProject(cmd.Context(), app, info.ProjectEntityID)

			if app.Format == "json" {
				out := map[string]any{
					"action": "done", "task_id": task.StableID, "git_branch": branch,
				}
				if scanErr == nil {
					out["scan"] = scanResult
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}
			fmt.Printf("Task %q done", task.Title)
			if branch != "" {
				fmt.Printf(" (branch %s)", branch)
			}
			fmt.Println()
			if scanErr == nil && scanResult != nil {
				if scanResult.Added > 0 || scanResult.Bound > 0 || scanResult.Orphaned > 0 {
					fmt.Printf("Scan: +%d added, +%d bound, %d orphaned\n",
						scanResult.Added, scanResult.Bound, scanResult.Orphaned)
				}
			}
			return nil
		},
	}
	return cmd
}

func newTaskUpdateCmd(app *App) *cobra.Command {
	var status, assignee string

	cmd := &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := taskService(app)
			task, err := svc.FindTask(cmd.Context(), info.ProjectEntityID, taskPrefix)
			if err != nil {
				return err
			}

			if err := svc.UpdateTask(cmd.Context(), task.EntityID, status, assignee); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "updated"})
			}
			fmt.Printf("Task %q updated.\n", task.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status")
	cmd.Flags().StringVar(&assignee, "assignee", "", "New assignee")
	return cmd
}
