package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show project health summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			project, err := app.Service.GetProjectByID(cmd.Context(), info.ProjectEntityID)
			if err != nil {
				return err
			}

			conn := app.DB.Conn()

			// Aggregate counts
			var sectionCount, staleCount, decisionCount, threadCount, conflictCount, taskCount int

			conn.QueryRow("SELECT count(*) FROM p_sections WHERE project_id = ? AND branch_id = ?",
				info.ProjectEntityID, info.BranchID).Scan(&sectionCount)
			conn.QueryRow("SELECT count(*) FROM p_sections WHERE project_id = ? AND branch_id = ? AND is_stale = 1",
				info.ProjectEntityID, info.BranchID).Scan(&staleCount)
			conn.QueryRow("SELECT count(*) FROM p_decisions WHERE project_id = ? AND branch_id = ?",
				info.ProjectEntityID, info.BranchID).Scan(&decisionCount)
			conn.QueryRow("SELECT count(*) FROM p_threads WHERE project_id = ? AND status = 'open'",
				info.ProjectEntityID).Scan(&threadCount)
			conn.QueryRow("SELECT count(*) FROM p_conflicts WHERE project_id = ? AND branch_id = ? AND status = 'unresolved'",
				info.ProjectEntityID, info.BranchID).Scan(&conflictCount)
			conn.QueryRow("SELECT count(*) FROM p_tasks WHERE project_id = ? AND status IN ('todo', 'in-progress')",
				info.ProjectEntityID).Scan(&taskCount)

			// Branch name
			var branchName string
			conn.QueryRow("SELECT COALESCE(name, '(unnamed)') FROM p_branches WHERE entity_id = ?",
				info.BranchID).Scan(&branchName)

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"project":              project.Name,
					"status":               project.Status,
					"branch":               branchName,
					"sections":             sectionCount,
					"stale_sections":       staleCount,
					"decisions":            decisionCount,
					"open_threads":         threadCount,
					"unresolved_conflicts": conflictCount,
					"pending_tasks":        taskCount,
				})
			}

			fmt.Printf("Project: %s (%s)\n", project.Name, project.Status)
			fmt.Printf("Branch:  %s\n", branchName)
			fmt.Printf("Sections:  %d", sectionCount)
			if staleCount > 0 {
				fmt.Printf(" (%d stale ⚠)", staleCount)
			}
			fmt.Println()
			fmt.Printf("Decisions: %d\n", decisionCount)
			if threadCount > 0 {
				fmt.Printf("Open threads: %d\n", threadCount)
			}
			if conflictCount > 0 {
				fmt.Printf("Unresolved conflicts: %d ⚠\n", conflictCount)
			}
			if taskCount > 0 {
				fmt.Printf("Pending tasks: %d\n", taskCount)
			}
			return nil
		},
	}
}
